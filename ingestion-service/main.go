package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
)

type LogEntry struct {
	Service   string            `json:"service"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	TraceID   string            `json:"trace_id,omitempty"`
	Host      string            `json:"host,omitempty"`
	Timestamp int64             `json:"timestamp,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

const defaultMaxBodySize = 1 << 20 // 1 MB
const kafkaHealthCheckTimeout = 3 * time.Second

func getMaxBodySize() int64 {
	if s := os.Getenv("MAX_BODY_SIZE"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return defaultMaxBodySize
}

var (
	logsIngested = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_ingested_total",
			Help: "Total number of logs received by the ingestion service",
		},
		[]string{"service", "level"},
	)

	ingestionLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ingestion_duration_seconds",
			Help:    "Time taken to process and send log to Kafka",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	logsDLQTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_dlq_total",
			Help: "Total number of logs sent to the Dead Letter Queue (DLQ)",
		},
		[]string{"service", "reason"},
	)

	dlqWriteFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dlq_write_failures_total",
			Help: "Total number of failed attempts to write to the Dead Letter Queue (DLQ)",
		},
		[]string{"service"},
	)
)

func init() {
	prometheus.MustRegister(logsIngested)
	prometheus.MustRegister(ingestionLatency)
	prometheus.MustRegister(logsDLQTotal)
	prometheus.MustRegister(dlqWriteFailuresTotal)
}

// logWriter is satisfied by kafkaLogWriter (or mockWriter in tests) and allows mock injection in tests.
type logWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Ping(ctx context.Context) error
	Close() error
}

type kafkaLogWriter struct {
	*kafka.Writer
	broker string
}

func newKafkaLogWriter(broker string) *kafkaLogWriter {
	return &kafkaLogWriter{
		Writer: &kafka.Writer{
			Addr:     kafka.TCP(broker),
			Topic:    "raw-logs",
			Balancer: &kafka.Hash{},
		},
		broker: broker,
	}
}

func (w *kafkaLogWriter) Ping(ctx context.Context) error {
	if w == nil || w.broker == "" {
		return errors.New("kafka broker not configured")
	}
	conn, err := kafka.DialContext(ctx, "tcp", w.broker)
	if err != nil {
		return err
	}
	return conn.Close()
}

type LogJob struct {
	Entry LogEntry
	Raw   []byte
}

const (
	defaultQueueCapacity = 10000
	defaultWorkerCount   = 4
	maxRetries           = 3
	initialBackoff       = 100 * time.Millisecond
	maxBackoff           = 2000 * time.Millisecond
)

func getQueueCapacity() int {
	if s := os.Getenv("INGEST_QUEUE_CAPACITY"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return defaultQueueCapacity
}

func getWorkerCount() int {
	if s := os.Getenv("INGEST_WORKERS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return defaultWorkerCount
}

func computeBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return initialBackoff
	}
	delay := initialBackoff * time.Duration(1<<(attempt-1))
	if delay > maxBackoff {
		return maxBackoff
	}
	return delay
}

type Server struct {
	writer      logWriter
	dlqWriter   logWriter
	router      *gin.Engine
	http        *http.Server
	queue       chan LogJob
	workerCount int
	wg          sync.WaitGroup
	sleepFn     func(time.Duration)
}

func NewServer(writer logWriter, dlqWriters ...logWriter) *Server {
	var dlq logWriter
	if len(dlqWriters) > 0 {
		dlq = dlqWriters[0]
	}
	s := &Server{
		writer:      writer,
		dlqWriter:   dlq,
		queue:       make(chan LogJob, getQueueCapacity()),
		workerCount: getWorkerCount(),
		sleepFn:     time.Sleep,
	}
	s.router = s.setupRouter()
	s.startWorkers()
	return s
}

func (s *Server) startWorkers() {
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.workerLoop()
	}
}

func (s *Server) workerLoop() {
	defer s.wg.Done()
	for job := range s.queue {
		s.processJob(job)
	}
}

type DLQPayload struct {
	OriginalLog   LogEntry `json:"original_log"`
	OriginalTopic string   `json:"original_topic"`
	RetryCount    int      `json:"retry_count"`
	FailureReason string   `json:"failure_reason"`
	FailedAt      int64    `json:"failed_at"`
}

func newKafkaDLQWriter(broker string) *kafkaLogWriter {
	return &kafkaLogWriter{
		Writer: &kafka.Writer{
			Addr:     kafka.TCP(broker),
			Topic:    "raw-logs-dlq",
			Balancer: &kafka.Hash{},
		},
		broker: broker,
	}
}

func (s *Server) processJob(job LogJob) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := computeBackoff(attempt)
			if s.sleepFn != nil {
				s.sleepFn(backoff)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(job.Entry.Service),
			Value: job.Raw,
		})
		cancel()

		if err == nil {
			logsIngested.WithLabelValues(job.Entry.Service, job.Entry.Level).Inc()
			return
		}
		lastErr = err
		log.Printf("kafka write attempt %d failed for service %s: %v", attempt+1, job.Entry.Service, err)
	}

	s.routeToDLQ(job, lastErr)
}

func (s *Server) routeToDLQ(job LogJob, lastErr error) {
	errMsg := "unknown error"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}

	dlqMsg := DLQPayload{
		OriginalLog:   job.Entry,
		OriginalTopic: "raw-logs",
		RetryCount:    maxRetries,
		FailureReason: errMsg,
		FailedAt:      time.Now().Unix(),
	}

	val, err := json.Marshal(dlqMsg)
	if err != nil {
		dlqWriteFailuresTotal.WithLabelValues(job.Entry.Service).Inc()
		log.Printf("failed to marshal DLQ payload for service %s: %v", job.Entry.Service, err)
		return
	}

	if s.dlqWriter == nil {
		dlqWriteFailuresTotal.WithLabelValues(job.Entry.Service).Inc()
		log.Printf("DLQ writer not configured for service %s", job.Entry.Service)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = s.dlqWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(job.Entry.Service),
		Value: val,
	})
	cancel()

	if err != nil {
		dlqWriteFailuresTotal.WithLabelValues(job.Entry.Service).Inc()
		log.Printf("failed to write to DLQ for service %s: %v", job.Entry.Service, err)
		return
	}

	logsDLQTotal.WithLabelValues(job.Entry.Service, "retry_exhaustion").Inc()
	log.Printf("successfully routed log for service %s to DLQ topic 'raw-logs-dlq'", job.Entry.Service)
}

func (s *Server) setupRouter() *gin.Engine {
	r := gin.Default()
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/healthz", s.handleHealthz)
	r.POST("/ingest", s.handleIngest)
	return r
}

func (s *Server) handleHealthz(c *gin.Context) {
	if s.writer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "log writer not initialized",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), kafkaHealthCheckTimeout)
	defer cancel()

	if err := s.writer.Ping(ctx); err != nil {
		log.Printf("kafka health check failed: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "kafka unavailable",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleIngest(c *gin.Context) {
	start := time.Now()

	// Limit request body to 1 MB to prevent memory exhaustion from
	// malicious or misconfigured clients. http.MaxBytesReader returns
	// a *http.MaxBytesError when the limit is exceeded.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, getMaxBodySize())

	var entry LogEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid log format"})
		return
	}

	if entry.Service == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service is required"})
		return
	}
	if entry.Level == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level is required"})
		return
	}

	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().Unix()
	}

	val, err := json.Marshal(entry)
	if err != nil {
		ingestionLatency.WithLabelValues("error").Observe(time.Since(start).Seconds())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize log entry"})
		return
	}

	job := LogJob{
		Entry: entry,
		Raw:   val,
	}

	select {
	case s.queue <- job:
		ingestionLatency.WithLabelValues("success").Observe(time.Since(start).Seconds())
		c.JSON(http.StatusAccepted, gin.H{"status": "accepted", "message": "log queued for ingestion"})
	default:
		ingestionLatency.WithLabelValues("rate_limited").Observe(time.Since(start).Seconds())
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "ingestion queue full, retry later"})
	}
}

func (s *Server) Start(addr string) error {
	s.http = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	var httpErr error
	if s.http != nil {
		httpErr = s.http.Shutdown(ctx)
	}

	if s.queue != nil {
		close(s.queue)
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		if httpErr != nil {
			return httpErr
		}
		return ctx.Err()
	}

	return httpErr
}

func main() {
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	writer := newKafkaLogWriter(kafkaBroker)
	dlqWriter := newKafkaDLQWriter(kafkaBroker)

	srv := NewServer(writer, dlqWriter)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		addr := ":8080"
		if v := os.Getenv("LISTEN_ADDR"); v != "" {
			addr = v
		}
		log.Printf("starting server on %s", addr)
		if err := srv.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}

	if err := writer.Close(); err != nil {
		log.Printf("kafka writer close error: %v", err)
	}

	if err := dlqWriter.Close(); err != nil {
		log.Printf("kafka dlq writer close error: %v", err)
	}

	log.Println("shutdown complete")
}
