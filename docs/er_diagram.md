# Entity-Relationship (ER) Diagram

The system employs a heavily normalized relational schema with strict foreign key constraints and optimized partial indices for high-throughput queue operations.

```mermaid
erDiagram
    users ||--o{ org_members : "belongs to"
    users ||--o{ project_members : "belongs to"
    organizations ||--o{ projects : "owns"
    organizations ||--o{ org_members : "has"
    projects ||--o{ project_members : "has"
    projects ||--o{ queues : "owns"
    
    queues ||--o{ jobs : "contains"
    queues ||--o{ cron_jobs : "defines"
    queues ||--o{ batches : "groups"
    queues ||--o{ queue_metrics : "tracked by"
    
    cron_jobs ||--o{ jobs : "spawns"
    batches ||--o{ jobs : "contains"
    
    jobs ||--o{ job_executions : "has attempts"
    workers ||--o{ job_executions : "executes"
    job_executions ||--o{ execution_logs : "emits"

    users {
        uuid id PK
        string email UK
        string password_hash
    }
    organizations {
        uuid id PK
        string slug UK
    }
    projects {
        uuid id PK
        uuid org_id FK
        string api_key_hash UK
    }
    queues {
        uuid id PK
        uuid project_id FK
        int priority
        int concurrency
        int max_retries
        string retry_strategy
    }
    jobs {
        uuid id PK
        uuid queue_id FK
        uuid batch_id FK
        uuid cron_job_id FK
        string status
        timestamp run_at
        string idempotency_key UK
    }
    job_executions {
        uuid id PK
        uuid job_id FK
        uuid worker_id FK
        int attempt
        string status
    }
    workers {
        uuid id PK
        string status
        timestamp last_heartbeat_at
    }
```
