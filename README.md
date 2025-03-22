# Microservice Logging and Tracing System

This project implements a comprehensive logging and tracing system for microservices using NATS JetStream, Loki, Grafana, and Jaeger with OpenTelemetry.

## System Architecture

### Components

1. **Middleware (Gin)**
    - Logs request & response with Trace ID
    - Sends logs to NATS JetStream
    - Integrates OpenTelemetry for tracing

2. **NATS JetStream**
    - Message queue for asynchronous log processing
    - Provides persistence and replay capabilities

3. **Log Consumer**
    - Receives logs from NATS
    - Forwards logs to Loki

4. **Loki + Grafana**
    - Stores and visualizes logs
    - Provides querying capabilities

5. **Jaeger**
    - Distributed tracing system
    - Visualizes request flow across services

## Project Structure

```
├── cmd/
│   ├── api/
│   │   └── main.go        # API service entrypoint
│   └── consumer/
│       └── main.go        # Log consumer entrypoint
├── middleware/
│   ├── logger.go          # Request/response logging middleware
│   └── tracing.go         # OpenTelemetry tracing middleware
├── nats/
│   └── client.go          # NATS JetStream client wrapper
├── grafana/
│   └── provisioning/      # Grafana dashboards and datasources
├── docker-compose.yml     # Docker Compose setup
├── Dockerfile.api         # API service Dockerfile
├── Dockerfile.consumer    # Log consumer Dockerfile
├── go.mod                 # Go module definition
└── go.sum                 # Go module checksums
```

## Setup Instructions

### Prerequisites

- Docker and Docker Compose
- Go 1.18 or higher (for local development)

### Running the System

1. Clone the repository:
   ```bash
   git clone https://github.com/yourapplication/service.git
   cd service
   ```

2. Create required directories for Grafana provisioning:
   ```bash
   mkdir -p grafana/provisioning/datasources
   mkdir -p grafana/provisioning/dashboards/json
   ```

3. Copy the configuration files to the appropriate directories:
   ```bash
   # Copy Loki datasource configuration
   cp grafana/provisioning/datasources/loki.yaml grafana/provisioning/datasources/
   
   # Copy dashboard provider configuration
   cp grafana/provisioning/dashboards/dashboard.yaml grafana/provisioning/dashboards/
   
   # Copy dashboard JSON
   cp grafana/provisioning/dashboards/json/logs-dashboard.json grafana/provisioning/dashboards/json/
   ```

4. Start the system using Docker Compose:
   ```bash
   docker-compose up -d
   ```

5. Access the services:
    - API Service: http://localhost:8080
    - Grafana: http://localhost:3000 (username: admin, password: admin)
    - Jaeger UI: http://localhost:16686
    - NATS Monitoring: http://localhost:8222

### Using the API

The example API service has the following endpoints:

- GET `/health` - Health check endpoint
- GET `/api/v1/users` - Get all users
- GET `/api/v1/users/:id` - Get user by ID
- POST `/api/v1/users` - Create a new user

Example API request:
```bash
# Create a user
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"John Doe","email":"john@example.com"}'

# Get users
curl http://localhost:8080/api/v1/users

# Get a specific user
curl http://localhost:8080/api/v1/users/1
```

### Viewing Logs and Traces

1. **Grafana (Logs)**
    - Open http://localhost:3000
    - Login with admin/admin
    - Navigate to the "Microservices Logs" dashboard
    - Use LogQL to query logs, e.g.: `{service="api-service"}`

2. **Jaeger (Traces)**
    - Open http://localhost:16686
    - Select "api-service" from the Service dropdown
    - Click "Find Traces" to view traces
    - Click on a trace to see the detailed span information

## Extending the System

### Adding a New Service

1. Create a new service directory:
   ```bash
   mkdir -p cmd/newservice
   ```

2. Implement your service using the same middleware:
   ```go
   // Import the middleware and nats packages
   router.Use(middleware.Tracing("new-service"))
   router.Use(middleware.Logger(client.JS, "new-service", environment, "logs.new-service"))
   ```

3. Add the service to Docker Compose:
   ```yaml
   new-service:
     build:
       context: .
       dockerfile: Dockerfile.newservice
     environment:
       - NATS_URL=nats://nats:4222
       - JAEGER_URL=jaeger:4317
       - SERVICE_NAME=new-service
       - ENVIRONMENT=development
       - PORT=8081
     ports:
       - "8081:8081"
     networks:
       - app-network
     depends_on:
       - nats
       - jaeger
   ```

### Customizing Log Fields

To add custom fields to the logs, modify the `LogEntry` struct in `middleware/logger.go`:

```go
type LogEntry struct {
    // Existing fields...
    
    // Add your custom fields
    UserID      string `json:"user_id,omitempty"`
    RequestID   string `json:"request_id,omitempty"`
    CustomField string `json:"custom_field,omitempty"`
}
```

Then update the logger middleware to populate these fields.

## Troubleshooting

### Common Issues

1. **NATS Connection Issues**
    - Check if NATS is running: `docker-compose ps`
    - Verify connection string: `nats://nats:4222`

2. **Missing Logs in Grafana**
    - Verify Loki datasource is configured correctly
    - Check the log consumer service is running: `docker-compose logs log-consumer`
    - Check Loki logs: `docker-compose logs loki`

3. **Missing Traces in Jaeger**
    - Ensure the OTLP port is properly exposed (4317)
    - Check Jaeger logs: `docker-compose logs jaeger`

## Performance Considerations

- For high-traffic services, consider:
    - Scaling the log consumer horizontally
    - Using file storage for NATS JetStream
    - Implementing log sampling for high-volume endpoints

## Security Considerations

- This setup uses default credentials for demonstration
- For production:
    - Secure NATS with authentication
    - Change default Grafana credentials
    - Add proper authentication to the API service
    - Implement TLS for all service communication