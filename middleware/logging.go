package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// Logging returns a middleware that logs request details including:
// - HTTP method
// - Request path
// - Client IP
// - Response status code
// - Latency time
// - User agent
func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		startTime := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(startTime)

		// Get request and response details
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()
		userAgent := c.Request.UserAgent()

		// Log format
		logLine := fmt.Sprintf("[REQUEST] %s | %d | %v | %s | %s | %s",
			method,
			statusCode,
			latency,
			clientIP,
			path,
			userAgent,
		)

		// Log based on status code
		if statusCode >= 500 {
			c.Error(fmt.Errorf(logLine))
		} else if statusCode >= 400 {
			c.Error(fmt.Errorf(logLine))
		} else {
			fmt.Println(logLine)
		}
	}
}
