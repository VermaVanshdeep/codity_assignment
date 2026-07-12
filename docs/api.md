# API Documentation

The REST API exposes resources for managing the job lifecycle, configurations, and organizational access. All requests (except authentication) require a valid JWT Bearer token.

## Authentication

### `POST /api/v1/auth/login`
Authenticates a user and returns a JWT access token.
* **Body:** `{"email": "admin@example.com", "password": "password123"}`
* **Response:** `200 OK` with `{ "data": { "access_token": "ey...", "user": {...} } }`

## Queues

### `POST /api/v1/projects/:projectId/queues`
Creates a new job queue.
* **Body:** 
  ```json
  {
    "name": "email-queue",
    "priority": 5,
    "concurrency": 20,
    "max_retries": 3,
    "retry_strategy": "exponential"
  }
  ```

### `GET /api/v1/projects/:projectId/queues`
Lists all queues in a project with pagination.

## Jobs

### `POST /api/v1/projects/:projectId/queues/:queueId/jobs`
Enqueues a new job for execution.
* **Body:**
  ```json
  {
    "type": "email.send",
    "payload": {
      "to": "user@example.com",
      "subject": "Welcome!"
    },
    "run_at": "2024-12-01T12:00:00Z", 
    "idempotency_key": "unique-request-id-1234"
  }
  ```
*(Note: Omit `run_at` for immediate execution).*

### `GET /api/v1/projects/:projectId/queues/:queueId/jobs?status=failed&limit=50`
Queries jobs by status, type, and execution time.

## Cron & Batches

### `POST /api/v1/projects/:projectId/queues/:queueId/cron`
Creates a recurring cron job definition.
* **Body:**
  ```json
  {
    "name": "daily-report",
    "cron_expr": "0 0 * * *",
    "job_type": "report.generate",
    "payload": {}
  }
  ```

## Real-time Metrics (WebSocket)

### `WS /api/v1/ws?access_token=<JWT>`
Upgrades the connection to a WebSocket. Pushes live JSON events:
```json
{
  "type": "job_completed",
  "data": { "job_id": "uuid", "duration_ms": 150 }
}
```
