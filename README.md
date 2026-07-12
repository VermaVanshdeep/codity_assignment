# Distributed Job Scheduler

A production-inspired distributed job scheduling platform designed for reliable execution of asynchronous background jobs across multiple workers. Built with a focus on backend engineering, database design, concurrency, and reliability.

## 🚀 Key Features

* **Full Job Lifecycle:** Queued → Scheduled → Claimed → Running → Completed (with retries and DLQ).
* **Concurrency & Reliability:** Uses `SELECT ... FOR UPDATE SKIP LOCKED` for atomic job claiming, preventing duplicate executions.
* **Configurable Retries:** Supports Fixed, Linear, and Exponential Backoff (with optional jitter).
* **Cron & Batch Jobs:** Schedule recurring cron jobs or execute jobs in tracked batches.
* **Authentication & RBAC:** JWT-based authentication with Organizational and Project-level access controls.
* **Real-time Observability:** React/Next.js dashboard connected via WebSockets for live updates on queue health, worker status, and metrics.
* **Graceful Shutdown:** Workers capture `SIGTERM` and finish executing claimed jobs before shutting down.
* **Rate Limiting:** IP-based rate limiting via Redis.

## 🛠 Tech Stack

* **Backend:** Go (Fiber, pgx, golang-migrate)
* **Database:** PostgreSQL (Core schema, queues, execution logs)
* **Cache/Broker:** Redis (Rate Limiting, WebSockets Pub/Sub, Distributed Locking)
* **Frontend:** Next.js (React), Zustand, TanStack Query, TailwindCSS
* **Deployment:** Docker & Docker Compose

## 📦 Setup Instructions

1. **Clone the repository:**
   ```bash
   git clone https://github.com/VermaVanshdeep/codify_assignment.git
   cd codify_assignment
   ```

2. **Environment Variables:**
   ```bash
   cp .env.example .env
   # Modify .env if needed, though default values work out-of-the-box for Docker.
   ```

3. **Run with Docker Compose:**
   ```bash
   docker-compose up --build -d
   ```
   This will start PostgreSQL, Redis, the Go API server, the Go Worker service, and the Next.js Frontend.
   Database migrations will automatically apply on startup.

4. **Seed Initial Data:**
   ```bash
   # Seeds the database with an admin user, organization, project, queue, and mock jobs
   python3 seed.py
   ```

5. **Access the Application:**
   * **Dashboard:** [http://localhost:3000](http://localhost:3000)
   * **API Base URL:** `http://localhost:8080/api/v1`
   
   **Test Credentials:**
   * **Email:** `admin@example.com`
   * **Password:** `password123`

## 📁 Documentation Deliverables

* **Architecture Diagram:** [docs/architecture.md](docs/architecture.md)
* **ER Diagram:** [docs/er_diagram.md](docs/er_diagram.md)
* **API Documentation:** [docs/api.md](docs/api.md)
* **Design Decisions & Trade-offs:** [docs/design_decisions.md](docs/design_decisions.md)
