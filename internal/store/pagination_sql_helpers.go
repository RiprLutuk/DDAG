package store

func roleListCountSQL() string {
	return `SELECT count(*) FROM roles WHERE ($1='' OR name ILIKE '%'||$1||'%' OR description ILIKE '%'||$1||'%')`
}
func roleListSQL(p ListParams) string {
	return `SELECT id, name, description, is_system, created_at, updated_at FROM roles WHERE ($1='' OR name ILIKE '%'||$1||'%' OR description ILIKE '%'||$1||'%') ORDER BY ` + p.OrderBy(map[string]string{"name": "name", "created_at": "created_at"}, "name") + ` LIMIT $2 OFFSET $3`
}
func scopeListCountSQL() string {
	return `SELECT count(*) FROM scopes WHERE ($1='' OR scope_code ILIKE '%'||$1||'%' OR description ILIKE '%'||$1||'%')`
}
func scopeListSQL(p ListParams) string {
	return `SELECT id, scope_code, description, created_at, updated_at FROM scopes WHERE ($1='' OR scope_code ILIKE '%'||$1||'%' OR description ILIKE '%'||$1||'%') ORDER BY ` + p.OrderBy(map[string]string{"scope_code": "scope_code", "created_at": "created_at"}, "scope_code") + ` LIMIT $2 OFFSET $3`
}
func rateLimitListCountSQL() string {
	return `SELECT count(*) FROM rate_limit_rules WHERE ($1='' OR applies_to ILIKE '%'||$1||'%')`
}
func rateLimitListSQL(p ListParams) string {
	return `SELECT ` + rateCols + ` FROM rate_limit_rules WHERE ($1='' OR applies_to ILIKE '%'||$1||'%') ORDER BY ` + p.OrderBy(map[string]string{"applies_to": "applies_to", "created_at": "created_at"}, "created_at") + ` LIMIT $2 OFFSET $3`
}
func ipWhitelistListCountSQL() string {
	return `SELECT count(*) FROM ip_whitelists WHERE ($1='' OR ip_cidr ILIKE '%'||$1||'%' OR description ILIKE '%'||$1||'%')`
}
func ipWhitelistListSQL(p ListParams) string {
	return `SELECT ` + ipCols + ` FROM ip_whitelists WHERE ($1='' OR ip_cidr ILIKE '%'||$1||'%' OR description ILIKE '%'||$1||'%') ORDER BY ` + p.OrderBy(map[string]string{"created_at": "created_at", "status": "status"}, "created_at") + ` LIMIT $2 OFFSET $3`
}
func cacheRuleListCountSQL() string {
	return `SELECT count(*) FROM cache_rules cr LEFT JOIN api_definitions a ON a.id=cr.api_definition_id WHERE ($1='' OR a.name ILIKE '%'||$1||'%' OR a.path ILIKE '%'||$1||'%')`
}
func cacheRuleListSQL(p ListParams) string {
	return `SELECT cr.id, cr.api_definition_id, COALESCE(a.name, '') AS api_name, COALESCE(a.method, '') AS api_method, COALESCE(a.path, '') AS api_path, cr.enabled, cr.ttl_seconds, cr.cache_key_strategy, cr.vary_by_client, cr.created_at, cr.updated_at FROM cache_rules cr LEFT JOIN api_definitions a ON a.id=cr.api_definition_id WHERE ($1='' OR a.name ILIKE '%'||$1||'%' OR a.path ILIKE '%'||$1||'%') ORDER BY ` + p.OrderBy(map[string]string{"created_at": "created_at"}, "created_at") + ` LIMIT $2 OFFSET $3`
}
