package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
)

// This is the shape of a "Log". Every log sent to us must look like this.
type LogEntry struct {
	Service string `json:"service"` // e.g., "payment-gateway"
	Level   string `json:"level"`   // e.g., "error" or "info"
	Message string `json:"message"` // e.g., "Database connection failed"
}

// --- PROMETHEUS METRICS ---
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
	// Register metrics with Prometheus
	prometheus.MustRegister(logsIngested)
	prometheus.MustRegister(ingestionLatency)
}

func main() {
	// 1. Setup the connection to Kafka (The Pipe)
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "raw-logs",
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// 2. Create the Web Server (The Funnel)
	r := gin.Default()

	// --- EXPOSE METRICS ENDPOINT ---
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 3. Define the "Ingest" endpoint
	r.POST("/ingest", func(c *gin.Context) {
		start := time.Now()
		var entry LogEntry

		// Check if the incoming data is a valid LogEntry
		if err := c.ShouldBindJSON(&entry); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log format"})
			return
		}

		// Push the log into the Kafka Pipe
		err := writer.WriteMessages(context.Background(),
			kafka.Message{
				Value: []byte(entry.Message),
			},
		)

		duration := time.Since(start).Seconds()

		if err != nil {
			// Record failure metric
			ingestionLatency.WithLabelValues("error").Observe(duration)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send to Kafka"})
			return
		}

		// Record success metrics
		ingestionLatency.WithLabelValues("success").Observe(duration)
		logsIngested.WithLabelValues(entry.Service, entry.Level).Inc()

		c.JSON(http.StatusOK, gin.H{"status": "Log received and sent to pipe!"})
	})

	// Start the server on port 8080
	r.Run(":8080")
}