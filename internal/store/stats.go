package store

import "context"

// Overview is the dashboard landing-page summary (PRD §11.16).
type Overview struct {
	TotalAPIsActive    int64           `json:"total_apis_active"`
	TotalClientsActive int64           `json:"total_clients_active"`
	TotalConnections   int64           `json:"total_connections"`
	RequestsToday      int64           `json:"requests_today"`
	ErrorRateToday     float64         `json:"error_rate_today"`
	AvgLatencyMS       float64         `json:"avg_latency_ms"`
	CacheHitRatio      float64         `json:"cache_hit_ratio"`
	TopTraffic         []EndpointStat  `json:"top_traffic"`
	TopSlow            []EndpointStat  `json:"top_slow"`
	TopErrors          []EndpointStat  `json:"top_errors"`
	Connectors         []ConnectorStat `json:"connectors"`
}

// EndpointStat is a per-endpoint aggregate for the overview lists.
type EndpointStat struct {
	Label     string  `json:"label" db:"label"`
	Count     int64   `json:"count" db:"count"`
	AvgMS     float64 `json:"avg_ms" db:"avg_ms"`
	ErrorRate float64 `json:"error_rate" db:"error_rate"`
}

// ConnectorStat is a source-DB health summary.
type ConnectorStat struct {
	ID           string `json:"id" db:"id"`
	Name         string `json:"name" db:"name"`
	DatabaseType string `json:"database_type" db:"database_type"`
	Status       string `json:"status" db:"status"`
	HealthStatus string `json:"health_status" db:"health_status"`
}

// Overview computes the dashboard summary from live metadata + request logs.
func (s *Store) Overview(ctx context.Context) (*Overview, error) {
	o := &Overview{}
	_ = s.get(ctx, &o.TotalAPIsActive, `SELECT count(*) FROM api_definitions WHERE status='PUBLISHED'`)
	_ = s.get(ctx, &o.TotalClientsActive, `SELECT count(*) FROM clients WHERE status='active'`)
	_ = s.get(ctx, &o.TotalConnections, `SELECT count(*) FROM database_connections`)
	_ = s.get(ctx, &o.RequestsToday, `SELECT count(*) FROM api_request_logs WHERE created_at >= date_trunc('day', now())`)

	_ = s.get(ctx, &o.ErrorRateToday, `
		SELECT COALESCE(
			100.0 * count(*) FILTER (WHERE status_code >= 400) / NULLIF(count(*),0), 0)
		FROM api_request_logs WHERE created_at >= date_trunc('day', now())`)
	_ = s.get(ctx, &o.AvgLatencyMS, `
		SELECT COALESCE(avg(latency_ms),0) FROM api_request_logs WHERE created_at >= date_trunc('day', now())`)
	_ = s.get(ctx, &o.CacheHitRatio, `
		SELECT COALESCE(
			100.0 * count(*) FILTER (WHERE cached) / NULLIF(count(*),0), 0)
		FROM api_request_logs WHERE created_at >= date_trunc('day', now())`)

	topTraffic, err := s.endpointStats(ctx, `ORDER BY count DESC`)
	if err != nil {
		return nil, err
	}
	o.TopTraffic = topTraffic
	topSlow, err := s.endpointStats(ctx, `ORDER BY avg_ms DESC`)
	if err != nil {
		return nil, err
	}
	o.TopSlow = topSlow
	topErr, err := s.endpointStats(ctx, `HAVING count(*) FILTER (WHERE status_code>=400) > 0 ORDER BY error_rate DESC`)
	if err != nil {
		return nil, err
	}
	o.TopErrors = topErr

	var conns []ConnectorStat
	if err := s.selectRows(ctx, &conns, `
		SELECT id::text, name, database_type, status, last_health_status AS health_status
		FROM database_connections ORDER BY name`); err != nil {
		return nil, err
	}
	o.Connectors = conns
	return o, nil
}

func (s *Store) endpointStats(ctx context.Context, tail string) ([]EndpointStat, error) {
	var out []EndpointStat
	err := s.selectRows(ctx, &out, `
		SELECT
			COALESCE(NULLIF(api_label,''), method||' '||path) AS label,
			count(*) AS count,
			COALESCE(avg(latency_ms),0) AS avg_ms,
			COALESCE(100.0 * count(*) FILTER (WHERE status_code>=400) / NULLIF(count(*),0),0) AS error_rate
		FROM api_request_logs
		WHERE created_at >= now() - interval '24 hours'
		GROUP BY 1 `+tail+` LIMIT 5`)
	if out == nil {
		out = []EndpointStat{}
	}
	return out, err
}
