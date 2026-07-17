-- 0007_audit_operation: add operation type to request logs for operation-aware audit.
ALTER TABLE api_request_logs ADD COLUMN IF NOT EXISTS operation TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_reqlog_operation ON api_request_logs(operation);
