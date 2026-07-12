# Design Decisions & Trade-offs

## 1. PostgreSQL as the Message Broker
**Decision:** We utilized PostgreSQL (`SELECT ... FOR UPDATE SKIP LOCKED`) instead of a dedicated message broker like RabbitMQ or Kafka.
**Trade-offs:**
* **Pros:** Simplifies infrastructure (no extra broker to maintain), ensures strict transactional guarantees (a job is only queued if the business logic transaction commits), and allows complex querying (e.g., finding jobs by payload or status).
* **Cons:** Lower raw throughput compared to in-memory brokers. `SKIP LOCKED` does incur some CPU overhead under extreme concurrency.
* **Mitigation:** We implemented *partial indices* on `(queue_id, priority, run_at) WHERE status IN ('pending', 'scheduled')` which keeps the index size extremely small and lookup times fast regardless of historical job bloat.

## 2. Distributed Locking vs. Atomic DB Operations
**Decision:** Job claiming is purely atomic via the database. Distributed locking (Redis) is reserved for strict concurrency boundaries (e.g., global cron scheduler limits or rate limits).
**Trade-offs:**
* **Pros:** Prevents race conditions intrinsically at the data tier. Workers are stateless and can scale horizontally indefinitely.
* **Cons:** Heavy reliance on the database connection pool.

## 3. WebSockets via Redis Pub/Sub
**Decision:** Real-time updates to the dashboard use WebSockets backed by a Redis Pub/Sub instance.
**Trade-offs:**
* **Pros:** Any API instance can handle WebSocket connections, while workers broadcast events to Redis. This decouples the API servers from the workers entirely.
* **Cons:** Adds Redis as a hard dependency for live metrics. 

## 4. Exponential Backoff with Jitter
**Decision:** Job retries default to exponential backoff with ±25% randomized jitter.
**Trade-offs:**
* **Pros:** Prevents "thundering herd" problems where downstream services recover from an outage only to be immediately DDoS'd by perfectly synchronized retrying workers.
* **Cons:** Delays might be slightly unpredictable, which is why a Fixed retry strategy is also implemented for latency-critical jobs.

## 5. Dead Letter Queues (DLQ)
**Decision:** Jobs that exhaust their `max_retries` transition to a `dead` status rather than being deleted.
**Trade-offs:**
* **Pros:** High observability. Operators can manually inspect and requeue dead letters from the dashboard.
* **Cons:** Requires active database cleanup/reaper processes (or table partitioning) to prevent endless storage growth over time.
