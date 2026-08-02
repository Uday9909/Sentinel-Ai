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
)

func init() {
	prometheus.MustRegister(logsIngested)
	prometheus.MustRegister(ingestionLatency)
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

type Server struct {
	writer logWriter
	router *gin.Engine
	http   *http.Server
}

func NewServer(writer logWriter) *Server {
	s := &Server{writer: writer}
	s.router = s.setupRouter()
	return s
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
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

	// Use the request's context with a timeout so a slow/broken Kafka doesn't
	// hold the connection open indefinitely.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	err = s.writer.WriteMessages(ctx,
		kafka.Message{
			Key:   []byte(entry.Service),
			Value: val,
		},
	)

	duration := time.Since(start).Seconds()
	if err != nil {
		ingestionLatency.WithLabelValues("error").Observe(duration)
		if errors.Is(err, context.DeadlineExceeded) {
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "kafka write timed out"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send to kafka"})
		return
	}

	ingestionLatency.WithLabelValues("success").Observe(duration)
	logsIngested.WithLabelValues(entry.Service, entry.Level).Inc()
	c.JSON(http.StatusOK, gin.H{"status": "log received"})
}

func (s *Server) Start(addr string) error {
	s.http = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func main() {
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	writer := newKafkaLogWriter(kafkaBroker)

	srv := NewServer(writer)

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

	log.Println("shutdown complete")
}
