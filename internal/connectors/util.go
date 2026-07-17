package connectors

import "time"

// rowToMap zips column names with values, converting raw byte slices to strings
// so the result serializes cleanly to JSON.
func rowToMap(cols []string, vals []any) map[string]any {
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		var v any
		if i < len(vals) {
			v = vals[i]
		}
		if b, ok := v.([]byte); ok {
			v = string(b)
		}
		m[c] = v
	}
	return m
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// connectTO returns the connect timeout, defaulting to 5s.
func connectTO(cfg PoolConfig) time.Duration {
	if cfg.ConnectTimeout > 0 {
		return cfg.ConnectTimeout
	}
	return 5 * time.Second
}

// queryTO resolves the effective query timeout: the per-request override if set,
// else the connection default, else 30s. A timeout is always enforced (PRD §13.5).
func queryTO(cfg PoolConfig, reqMS int) time.Duration {
	if reqMS > 0 {
		return time.Duration(reqMS) * time.Millisecond
	}
	if cfg.QueryTimeout > 0 {
		return cfg.QueryTimeout
	}
	return 30 * time.Second
}
