-- =============================================================================
-- Migration 000001: Rollback
-- Drops everything created in 000001_init_schema.up.sql in reverse dependency order.
-- =============================================================================

DROP TABLE IF EXISTS queue_metrics       CASCADE;
DROP TABLE IF EXISTS execution_logs      CASCADE;
DROP TABLE IF EXISTS job_executions      CASCADE;
DROP TABLE IF EXISTS workers             CASCADE;
DROP TABLE IF EXISTS jobs                CASCADE;
DROP TABLE IF EXISTS batches             CASCADE;
DROP TABLE IF EXISTS cron_jobs           CASCADE;
DROP TABLE IF EXISTS queues              CASCADE;
DROP TABLE IF EXISTS project_members     CASCADE;
DROP TABLE IF EXISTS projects            CASCADE;
DROP TABLE IF EXISTS org_members         CASCADE;
DROP TABLE IF EXISTS organizations       CASCADE;
DROP TABLE IF EXISTS refresh_tokens      CASCADE;
DROP TABLE IF EXISTS users               CASCADE;

DROP FUNCTION IF EXISTS trigger_set_updated_at CASCADE;

DROP TYPE IF EXISTS batch_status;
DROP TYPE IF EXISTS log_level;
DROP TYPE IF EXISTS execution_status;
DROP TYPE IF EXISTS worker_status;
DROP TYPE IF EXISTS retry_strategy;
DROP TYPE IF EXISTS job_status;
DROP TYPE IF EXISTS project_role;
DROP TYPE IF EXISTS org_role;
