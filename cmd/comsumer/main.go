// cmd/consumer/main.go
package main

import (
	"encoding/json"
	"log"
	"logtrace/internal/config"
	"logtrace/internal/loki"
	"logtrace/internal/middleware"
	natsclient "logtrace/internal/nats"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Set consumer name
	consumerName := "loki-consumer"

	// Set up NATS client
	natsConfig := natsclient.Config{
		URL:             cfg.NatsURL,
		ReconnectWait:   2 * time.Second,
		MaxReconnects:   -1,
		ConnectionName:  "log-consumer",
		StreamName:      cfg.NatsStreamName,
		StreamSubjects:  cfg.NatsSubjects,
		RetentionPolicy: nats.WorkQueuePolicy,
		StorageType:     cfg.NatsStorageType,
		MaxAge:          cfg.NatsMaxAge,
		Replicas:        cfg.NatsReplicas,
	}

	client, err := natsclient.NewClient(natsConfig)
	if err != nil {
		log.Fatalf("Failed to create NATS client: %v", err)
	}
	defer client.Close()

	log.Printf("Connected to NATS at %s", cfg.NatsURL)

	// Create Loki client
	lokiClient := loki.NewClient(cfg.LokiURL)

	// Create a pull consumer to batch process logs
	sub, err := client.SubscribePull(consumerName, cfg.NatsSubjects[0])
	if err != nil {
		log.Fatalf("Failed to create pull subscription: %v", err)
	}

	log.Printf("Pull subscription created, waiting for logs")

	// Channel to signal shutdown
	shutdown := make(chan struct{})

	// Start the consumer loop
	go func() {
		// Buffer for batch processing
		var batch []middleware.LogEntry
		var batchTimer *time.Timer
		const batchSize = 100
		const batchTimeoutMs = 1000 // 1 second

		resetTimer := func() {
			if batchTimer != nil {
				batchTimer.Stop()
			}
			batchTimer = time.AfterFunc(batchTimeoutMs*time.Millisecond, func() {
				if len(batch) > 0 {
					// Process the batch when the timer expires
					processBatch(batch, lokiClient)
					batch = batch[:0] // Clear the batch
				}
			})
		}

		resetTimer()

		for {
			select {
			case <-shutdown:
				// Process any remaining logs before exiting
				if len(batch) > 0 {
					processBatch(batch, lokiClient)
				}
				return
			default:
				// Try to fetch messages
				msgs, err := sub.Fetch(batchSize, nats.MaxWait(500*time.Millisecond))
				if err == nats.ErrTimeout {
					// No messages, continue
					continue
				}
				if err != nil {
					log.Printf("Error fetching messages: %v", err)
					time.Sleep(1 * time.Second)
					continue
				}

				// Process received messages
				for _, msg := range msgs {
					var logEntry middleware.LogEntry
					err := json.Unmarshal(msg.Data, &logEntry)
					if err != nil {
						log.Printf("Error unmarshaling log entry: %v", err)
						msg.Ack() // Acknowledge even if we couldn't process it
						continue
					}

					// Add to batch
					batch = append(batch, logEntry)

					// Acknowledge the message in NATS
					msg.Ack()
				}

				// Process batch if it's full
				if len(batch) >= batchSize {
					processBatch(batch, lokiClient)
					batch = batch[:0] // Clear the batch
					resetTimer()
				} else if len(batch) > 0 {
					// Reset the timer whenever we add to a non-empty batch
					resetTimer()
				}
			}
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	close(shutdown)
	time.Sleep(1 * time.Second) // Give the consumer loop time to finish

	log.Println("Consumer exiting")
}

// processBatch sends a batch of logs to Loki
func processBatch(batch []middleware.LogEntry, lokiClient *loki.Client) {
	if len(batch) == 0 {
		return
	}

	log.Printf("Processing batch of %d logs", len(batch))

	// Send batch to Loki
	err := lokiClient.SendBatchLogs(batch)
	if err != nil {
		log.Printf("Error sending logs to Loki: %v", err)

		// If batch send fails, try sending logs individually
		log.Println("Attempting to send logs individually")
		for _, entry := range batch {
			err := lokiClient.SendLog(entry)
			if err != nil {
				log.Printf("Error sending log to Loki: %v", err)
			}
		}
		return
	}

	log.Printf("Successfully sent %d logs to Loki", len(batch))
}
