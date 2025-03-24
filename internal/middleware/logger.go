package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	Timestamp    time.Time         `json:"timestamp"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Status       int               `json:"status"`
	Latency      float64           `json:"latency_ms"`
	ClientIP     string            `json:"client_ip"`
	UserAgent    string            `json:"user_agent"`
	RequestBody  string            `json:"request_body,omitempty"`
	ResponseBody string            `json:"response_body,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	ServiceName  string            `json:"service_name"`
	Environment  string            `json:"environment"`
	Error        string            `json:"error,omitempty"`
}

// bodyLogWriter is a custom response writer that captures the response body
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func Logger(js nats.JetStreamContext, serviceName, environment, subject string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Get or create trace context
		spanCtx := trace.SpanContextFromContext(c.Request.Context())
		traceID := spanCtx.TraceID().String()
		spanID := spanCtx.SpanID().String()

		// If no trace ID exists, create one
		if traceID == "00000000000000000000000000000000" {
			traceID = uuid.New().String()
			c.Set("trace_id", traceID)
		}

		// Set trace ID in response header
		c.Header("X-Trace-ID", traceID)

		// Read request body if it's not a multipart form
		var requestBodyBytes []byte
		if c.Request.Body != nil && c.Request.Body != http.NoBody && !strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
			requestBodyBytes, _ = io.ReadAll(c.Request.Body)
			// Restore the body so it can be read again in handlers
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))
		}

		// Create a response body writer
		bodyWriter := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = bodyWriter

		// Process request
		c.Next()

		// Collect headers
		headers := make(map[string]string)
		for k, v := range c.Request.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		// Create log entry
		entry := LogEntry{
			TraceID:     traceID,
			SpanID:      spanID,
			Timestamp:   time.Now(),
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			Status:      c.Writer.Status(),
			Latency:     float64(time.Since(start).Microseconds()) / 1000.0, // Convert to ms
			ClientIP:    c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
			Headers:     headers,
			ServiceName: serviceName,
			Environment: environment,
		}

		// Capture errors from gin context
		if len(c.Errors) > 0 {
			entry.Error = c.Errors.String()
		}

		// Include request body for non-binary content types
		contentType := c.GetHeader("Content-Type")
		if !isBinaryContent(contentType) && len(requestBodyBytes) > 0 {
			// Limit the size of logged request body
			if len(requestBodyBytes) > 10000 {
				entry.RequestBody = string(requestBodyBytes[:10000]) + "... (truncated)"
			} else {
				entry.RequestBody = string(requestBodyBytes)
			}
		}

		// Include response body for non-binary content types
		respContentType := bodyWriter.Header().Get("Content-Type")
		if !isBinaryContent(respContentType) && bodyWriter.body.Len() > 0 {
			// Limit the size of logged response body
			responseBody := bodyWriter.body.String()
			if len(responseBody) > 10000 {
				entry.ResponseBody = responseBody[:10000] + "... (truncated)"
			} else {
				entry.ResponseBody = responseBody
			}
		}

		// Marshal log entry to JSON
		entryJSON, err := json.Marshal(entry)
		if err != nil {
			// If JSON marshaling fails, just log the error and continue
			return
		}

		// Publish log entry to NATS JetStream
		_, err = js.Publish(subject, entryJSON)
		if err != nil {
			// In a real implementation, you might want to handle this error
			// For now, we'll just continue
		}
	}
}

func isBinaryContent(contentType string) bool {
	if contentType == "" {
		return false
	}

	return strings.Contains(contentType, "image/") ||
		strings.Contains(contentType, "video/") ||
		strings.Contains(contentType, "audio/") ||
		strings.Contains(contentType, "application/octet-stream") ||
		strings.Contains(contentType, "application/pdf") ||
		strings.Contains(contentType, "application/zip")
}
