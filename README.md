# HookRelay

HookRelay is an open-source, highly available webhook notification service designed for low-latency and high scalability. It provides seamless integration for producers and consumers, enabling efficient event-driven communication with support for rate limiting, retries, and endpoint failure notifications.

## Features

1. **Fanout**: Allows clients to subscribe to specific event types and receive notifications via webhooks. A client can configure multiple receivers for greater flexibility.
2. **Webhook Gateway**: Consumes events from multiple sources, including REST APIs, AWS SNS, and WebSockets.
3. **Rate Limiting**: Throttles event delivery to endpoints at a configurable rate per endpoint.
4. **Retry Mechanism**: Supports constant time and exponential backoff with jitter. Includes batch retry functionality for failed endpoints.
5. **Dashboard**: Provides an intuitive interface to manage endpoints, subscriptions, events, and sources. Allows debugging, retrying events, and configuring settings.
6. **Endpoint Failure Notifications**: Notifies users via email or Slack when endpoints consecutively fail to process events, automatically disabling them to prevent further issues.

## Core Concepts

### Endpoint

An endpoint is a valid HTTP URL capable of receiving webhook events. Each endpoint can be configured with:

- **Timeout**: Maximum duration to wait for a response.
- **Rate Limit**: Ensures compatibility with consumer capacity.
- **Authentication**: Adds an extra layer of security.
- **Owner ID**: Groups endpoints for fanning out events.

### Sources

Sources define how events are ingested into HookRelay. Supported sources include:

- **REST API**: Directly send events to the service.
- **AWS SNS**: Listen to AWS SNS topics for event ingestion.
- **WebSockets**: Real-time event streaming.

### Subscription

A subscription maps an endpoint to one or more event types. Subscriptions are created by admins and allow targeted event delivery.

### Event

An event is a message sent to HookRelay in the following format:

```json
{
  "payload": {...},
  "owner_id": "{string}",
  "event_type": "{string}"
}
```

Based on the `event_type`, HookRelay delivers the event to all matching subscriptions. The `owner_id` ensures targeted delivery to related endpoints.

## Architecture Overview

The architecture of HookRelay consists of the following key components:

- **Event Sources**: REST API, AWS SNS, WebSockets.
- **Event Dispatcher**: Handles fanout logic, matching events to subscriptions.
- **Rate Limiter**: Implements per-endpoint rate limiting using Redis.
- **Retry Queue**: Processes failed events with retry algorithms using Kafka or NATS.
- **Database**: PostgreSQL for storing metadata, Redis for caching.
- **Notification System**: Integrates with email and Slack for alerts.
- **Dashboard**: Admin interface for monitoring and configuration.

## Getting Started

### Prerequisites

- Go 1.20+
- Docker
- PostgreSQL
- Redis
- Kafka or NATS (optional for pub/sub)

### Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/codeasashu/hookrelay.git
   cd hookrelay
   ```

2. Build and run the application:

   ```bash
   make build
   make run
   ```

3. Deploy to Kubernetes:
   ```bash
   kubectl apply -f k8s/
   ```

### Configuration

Update the `config.yaml` file to match your environment. Example configuration:

```yaml
server:
  port: 8080

rateLimiter:
  redis:
    address: localhost:6379

retryQueue:
  kafka:
    brokers: ["localhost:9092"]
```

### Migrations

Migrations are created using [golang-migrate](https://github.com/golang-migrate/migrate) tool. Running command such as:

```sh
brew install golang-migrate

# Create a migration for postgresql
migrate create -ext sql -dir migrations/postgres -seq create_subscriptions

# Create a migration for mysql
migrate create -ext sql -dir migrations/mysql -seq create_subscriptions

# Apply all migration (for postgresql)
export POSTGRESQL_URL='postgres://admin:admin@localhost:5432/hookrelay?sslmode=disable'
migrate -database ${POSTGRESQL_URL} -path migrations/postgres up

# Apply all migration (for mysql)
export MYSQL_URL='mysql://admin:admin@tcp(localhost:3306)/hookrelay?x-tls-insecure-skip-verify=false'
migrate -database ${MYSQL_URL} -path migrations/mysql up
```

will create migrations

## Contributing

Contributions are welcome! Please read the [CONTRIBUTING.md](CONTRIBUTING.md) file for guidelines.

## License

HookRelay is licensed under the [MIT License](LICENSE).
