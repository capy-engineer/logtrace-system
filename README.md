# Microservice Logging and Tracing System

This project implements a comprehensive logging and tracing system for microservices using NATS JetStream, Loki, Grafana, and Jaeger with OpenTelemetry.

## Key Features

- Request/response logging with trace IDs
- Asynchronous log processing via NATS JetStream
- Log storage and visualization with Loki and Grafana
- Distributed tracing with OpenTelemetry and Jaeger
- Easy extension for multiple microservices
- Batch processing for efficient log handling

## Architecture

![Architecture Diagram](https://mermaid.ink/img/pako:eNptUsFqwzAM_RWhUwv5gN2ywWCw9jLYJ100ZBZOpDRm9kFKtn-fkyzttsMgSH56evqSNmCsMxiArXy7vSJEEZ6fwJj5mPzwFZ4rUSRrS9QQQvlXw8e8LMvEfmkkmzJd5J7DcFpXnJZAIdC5DmWMZDyJqF2TXtYuGb1Nx-OXPzFmU1eZloPRnRNJgDlRYd29YQ-JDFg6F9Z3aAQXUqfjrD3sNXTHy13iDy7c99jqHXNwm_6vUe2CJnVhkq8Gl6Gj-bJy_XyV1kxb5EfEmr1aUv-YKxM6fFvIRJLIcJMMpXVqZgaVPXpJ5GxZQ0CfLvbH0GNgX9mTb5aE2Xm6AybGxOAG5dZiScyj-MFgUgUZlRfQbkUzRZowoHJz5wUEiX-lfwd4wuC4Gw-)

### Components

1. **Middleware (Gin)**
   - Logs request & response with trace IDs
   - Sends logs to NATS JetStream
   - Integrates OpenTelemetry for tracing

2. **NATS JetStream**
   - Message queue for asynchronous log processing
   - Provides persistence and replay capabilities

3. **Log Consumer**
   - Receives logs from NATS
   - Forwards logs to Loki in batches

4. **Loki + Grafana**
   - Stores and visualizes logs
   - Provides querying capabilities

5. **Jaeger**
   - Distributed tracing system
   - Visualizes request flow across services

## Project Structure

```
microservice-logger/
├── cmd/
│   ├── api/
│   │   └── main.go                   # API service entrypoint
│   └── consumer/
│       └── main.go                   # Log consumer entrypoint
├── internal/
│   ├── config/
│   │   └── config.go                 # Configuration loader
│   ├── middleware/
│   │   ├── logger.go                 # Logging middleware
│   │   └── tracing.go                # Tracing middleware
│   ├── nats/
│   │   └── client.go                 # NATS JetStream client
│   └── loki/
│       └── client.go                 # Loki client
├── docker/
│   ├── grafana/
│   │   └── provisioning/
│   │       ├── dashboards/
│   │       │   ├── dashboard.yml
│   │       │   └── logs_dashboard.json
│   │       └── datasources/
│   │           └── loki.yml
│   └── jaeger/
│       └── jaeger.yml
├── docker-compose.yml                # Full stack deployment
├── Dockerfile.api                    # API service Dockerfile
├── Dockerfile.consumer               # Log consumer Dockerfile
├── go.mod
└── README.md
```

## Setup Instructions

### Prerequisites

- Docker and Docker Compose
- Go 1.18 or higher (for local development)

### Running the System

1. Clone the repository:
   ```bash
   git clone https://github.com/your-org/microservice-logger.git
   cd microservice-logger
   ```

2. Create required directories:
   ```bash
   mkdir -p docker/grafana/provisioning/datasources
   mkdir -p docker/grafana/provisioning/dashboards
   ```

3. Copy configuration files to the appropriate locations.

4. Start the system using Docker Compose:
   ```bash
   docker-compose up -d
   ```

5. Access the services:
   - API Service: http://localhost:8080
   - Grafana: http://localhost:3000 (username: admin, password: admin)
   - Jaeger UI: http://localhost:16686
   - NATS Monitoring: http://localhost:8222

### API Endpoints

The example API service provides these endpoints:

- `GET /health`: Health check endpoint
- `GET /api/v1/users`: Get all users
- `GET /api/v1/users/:id`: Get user by ID
- `POST /api/v1/users`: Create a new user
- `GET /api/v1/error`: Generates an error (for testing logging)

## Using the Middleware in Other Services

To add logging and tracing to another microservice:

1. Import the middleware package
2. Set up the NATS client
3. Add the middleware to your Gin router:

```go
// Set up NATS client
natsClient, err := natsclient.NewClient(natsConfig)
if err != nil {
    log.Fatalf("Failed to create NATS client: %v", err)
}
defer natsClient.Close()

// Set up Gin router
router := gin.New()
router.Use(gin.Recovery())
router.Use(middleware.Tracing(serviceName))
router.Use(middleware.Logger(natsClient.JS, serviceName, environment, logSubject))
```

## Viewing Logs and Traces

### Grafana (Logs)
1. Open http://localhost:3000
2. Log in with admin/admin
3. Navigate to the "Microservices Logs" dashboard
4. Use LogQL to query logs, e.g.: `{service="api-service"}`

### Jaeger (Traces)
1. Open http://localhost:16686
2. Select "api-service" from the Service dropdown
3. Click "Find Traces" to view traces
4. Click on a trace to see the detailed span information

## Advanced Configuration

Environment variables for configuration:

| Variable | Description | Default |
|----------|-------------|---------|
| SERVICE_NAME | Name of the service | microservice |
| ENVIRONMENT | Environment (dev, prod, etc.) | development |
| PORT | API service port | 8080 |
| NATS_URL | NATS connection URL | nats://localhost:4222 |
| NATS_STREAM | Name of the JetStream stream | logs |
| NATS_SUBJECT | Subject pattern for logs | logs.> |
| NATS_STORAGE_TYPE | Storage type (file or memory) | file |
| NATS_MAX_AGE | Maximum age of log entries | 168h (7 days) |
| JAEGER_URL | Jaeger OTLP endpoint | localhost:4317 |
| LOKI_URL | Loki HTTP push endpoint | http://localhost:3100/loki/api/v1/push |

## Performance Considerations

- Log consumer uses batch processing for efficient log forwarding
- NATS JetStream provides persistent storage with configurable retention
- Selective logging of request/response bodies based on content type
- Configurable batch size and flush intervals

## License

This project is licensed under the MIT License - see the LICENSE file for details.