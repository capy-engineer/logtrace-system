# 📊 Project: LogTrace System with OpenTelemetry & NATS

## **_ System Design Document _**

![LogTrace System](pkg/log-system.png "LogTrace System")

## **1. Objectives**

- Log request & response with **Trace ID**.
- Push logs to **NATS** for asynchronous processing.
- Store logs in **Loki** for querying and visualization in **Grafana**.
- Track **tracing** with **OpenTelemetry & Jaeger**.
- Easily extendable for multiple microservices.

---

## **2. System Architecture**

### **Main Components:**

- **Middleware (Gin)**: Logs request & response, sends logs to NATS, integrates OpenTelemetry.
- **NATS (Message Queue)**: Supports asynchronous log processing.
- **Consumer (Worker)**: Receives logs from NATS, sends logs to Loki.
- **Loki + Grafana**: Stores & visualizes logs.
- **Jaeger (Tracing)**: Tracks request flow across multiple services.

---

```
docker run -p 4222:4222 -v nats:/data nats -js -sd /data
```