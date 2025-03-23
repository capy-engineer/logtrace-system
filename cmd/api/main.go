package main

import (
	"context"
	"fmt"
	"log"
	"logtrace/docs"
	"logtrace/internal/config"
	"logtrace/internal/middleware"
	natsclient "logtrace/internal/nats"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	cfg := config.Load()
	// Initialize tracing
	shutdown, err := middleware.InitTracer(cfg.ServiceName, cfg.JaegerURL)
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	// Set up NATS client
	natsConfig := natsclient.Config{
		URL:             cfg.NatsURL,
		ReconnectWait:   2 * time.Second,
		MaxReconnects:   -1,
		ConnectionName:  cfg.ServiceName,
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

	// Set up the log subject
	logSubject := fmt.Sprintf("logs.%s", cfg.ServiceName)

	// Set up Gin router
	router := gin.New()
	docs.SwaggerInfo.BasePath = ""
	router.Use(gin.Recovery())
	router.Use(middleware.Tracing(cfg.ServiceName))
	router.Use(middleware.Logger(client.JS, cfg.ServiceName, cfg.Environment, logSubject))

	// Validation endpoints
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	router.GET("/ping", ping)

	// Set up routes
	setupRoutes(router)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
	}

	go func() {
		log.Printf("Starting %s server on port %d", cfg.ServiceName, cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Give the server 5 seconds to finish ongoing requests
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

// setupRoutes adds routes to the Gin router
func setupRoutes(router *gin.Engine) {
	// Health check
	router.GET("/ping", ping)

	// Example API endpoints
	v1 := router.Group("/api/v1")
	{
		v1.GET("/users")
		v1.GET("/users/:id")
		v1.POST("/users")
		v1.GET("/error") // Example endpoint to test error logging
	}
}

// @Summary Ping service
// @Description This endpoint checks the health of the service
// @Tags health
// @Accept  json
// @Produce json
// @Success 200 {string} string "pong"
// @Router /ping [get]
func ping(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}
