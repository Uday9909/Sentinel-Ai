package main

import (
	"context"
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
)

// This is the shape of a "Log". Every log sent to us must look like this.
type LogEntry struct {
	Service string `json:"service"` // e.g., "payment-gateway"
	Level   string `json:"level"`   // e.g., "error" or "info"
	Message string `json:"message"` // e.g., "Database connection failed"
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

	// 3. Define the "Ingest" endpoint
	r.POST("/ingest", func(c *gin.Context) {
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

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send to Kafka"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "Log received and sent to pipe!"})
	})

	// Start the server on port 8080
	r.Run(":8080")
}