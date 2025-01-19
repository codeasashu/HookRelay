## architecture Diagram

![Diagram](./docs/assets/diagram.svg)

---

### Key Components and Integrations

1. **Event Sources**:

   - REST API: Primary entry point for ingesting events.
   - SNS Listener: Handles events published to AWS SNS topics.
   - WebSocket Gateway: Supports real-time event ingestion.

2. **Event Dispatcher**:

   - Maps events to subscriptions and endpoints.
   - Implements fanout logic.
   - Supports rate limiting via Redis.

3. **Rate Limiting**:

   - Utilizes Redis to implement configurable rate limiting per endpoint.

4. **Retry Mechanism**:

   - Uses Kafka or NATS for queuing failed deliveries.
   - Retry workers process the queue with exponential backoff and jitter.

5. **Database**:

   - PostgreSQL: Stores endpoints, subscriptions, and event metadata.
   - Redis: Caches frequently accessed data and implements rate limiting.

6. **Notifications**:

   - Integrates with email services (e.g., SES) and Slack for failure alerts.

7. **Dashboard**:

   - Provides admin tools for debugging, retries, and configuration.

8. **Kubernetes Deployment**:
   - Deploy services as separate pods for modular scaling.
   - Use Helm charts for deployment management.
   - Utilize Prometheus and Grafana for monitoring and alerting.

This architecture ensures scalability, low latency, and high availability while providing tools for monitoring and debugging.
