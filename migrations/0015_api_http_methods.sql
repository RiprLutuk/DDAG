-- Expand dynamic data-plane methods. QUERY (RFC 9727) is a safe, body-capable
-- read method; mutation methods remain guarded by application-level is_write
-- validation and the existing approval/publish lifecycle.
ALTER TABLE api_definitions
    DROP CONSTRAINT IF EXISTS api_definitions_method_check;

ALTER TABLE api_definitions
    ADD CONSTRAINT api_definitions_method_check
    CHECK (method IN ('GET','QUERY','POST','PUT','PATCH','DELETE'));
