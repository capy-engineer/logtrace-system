package loki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"logtrace/internal/middleware"
	"net/http"
	"time"
)

// Client represents a Loki client
type Client struct {
	URL        string
	HTTPClient *http.Client
}

// PushRequest is the structure needed for Loki push API
type PushRequest struct {
	Streams []Stream `json:"streams"`
}

// Stream represents a stream of logs with a set of labels
type Stream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"` // [timestamp, log line]
}

// NewClient creates a new Loki client
func NewClient(url string) *Client {
	return &Client{
		URL: url,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendLog sends a log entry to Loki
func (c *Client) SendLog(entry middleware.LogEntry) error {
	// Convert log entry to JSON for Loki
	logLine, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Format timestamp for Loki (nanoseconds since epoch)
	timestampNano := entry.Timestamp.UnixNano()
	timestampStr := fmt.Sprintf("%d", timestampNano)

	// Create labels for the log stream
	labels := map[string]string{
		"service":     entry.ServiceName,
		"environment": entry.Environment,
		"trace_id":    entry.TraceID,
		"method":      entry.Method,
		"status":      fmt.Sprintf("%d", entry.Status),
	}

	// Create Loki push request
	req := PushRequest{
		Streams: []Stream{
			{
				Stream: labels,
				Values: [][]string{
					{timestampStr, string(logLine)},
				},
			},
		},
	}

	return c.sendToLoki(req)
}

// sendToLoki sends the push request to Loki
func (c *Client) sendToLoki(req PushRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal Loki request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", c.URL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request to Loki: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Loki returned error status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendBatchLogs sends multiple log entries to Loki in a single request
func (c *Client) SendBatchLogs(entries []middleware.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Group logs by labels
	streamMap := make(map[string][]middleware.LogEntry)
	for _, entry := range entries {
		// Create a key for grouping similar logs
		key := fmt.Sprintf("%s-%s-%s", entry.ServiceName, entry.Environment, entry.TraceID)
		streamMap[key] = append(streamMap[key], entry)
	}

	// Create streams for each group
	var streams []Stream
	for _, group := range streamMap {
		if len(group) == 0 {
			continue
		}

		// Use labels from the first entry
		first := group[0]
		labels := map[string]string{
			"service":     first.ServiceName,
			"environment": first.Environment,
			"trace_id":    first.TraceID,
		}

		// Create values for this stream
		var values [][]string
		for _, entry := range group {
			logLine, err := json.Marshal(entry)
			if err != nil {
				continue // Skip entries that can't be marshaled
			}

			timestampNano := entry.Timestamp.UnixNano()
			timestampStr := fmt.Sprintf("%d", timestampNano)
			values = append(values, []string{timestampStr, string(logLine)})
		}

		streams = append(streams, Stream{
			Stream: labels,
			Values: values,
		})
	}

	// Send to Loki
	req := PushRequest{
		Streams: streams,
	}

	return c.sendToLoki(req)
}
