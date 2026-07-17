package adminsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/internalauth"
)

type poolStatRow struct {
	ConnectionID   string `json:"connection_id"`
	Connection     string `json:"connection"`
	DBType         string `json:"db_type"`
	InUse          int    `json:"in_use"`
	Idle           int    `json:"idle"`
	Total          int    `json:"total"`
	Max            int    `json:"max"`
	WaitCount      int64  `json:"wait_count"`
	WaitDurationMS int64  `json:"wait_duration_ms"`
	TimeoutCount   int64  `json:"timeout_count"`
}

func (s *service) listPoolStats(w http.ResponseWriter, r *http.Request) {
	conns, err := s.store.ListAllConnections(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	byID := map[string]poolStatRow{}
	for _, conn := range conns {
		byID[conn.ID.String()] = poolStatRow{
			ConnectionID: conn.ID.String(),
			Connection:   conn.Name,
			DBType:       conn.DatabaseType,
			Max:          conn.MaxPoolSize,
		}
	}
	for dbType := range s.connectors {
		rows, err := s.callConnectorPools(r.Context(), dbType)
		if err != nil {
			s.log.Warn("connector_pool_stats_failed", "db_type", dbType, "error", err.Error())
			continue
		}
		for _, row := range rows {
			existing := byID[row.ConnectionID]
			row.Connection = existing.Connection
			row.DBType = dbType
			if row.Max == 0 {
				row.Max = existing.Max
			}
			byID[row.ConnectionID] = row
		}
	}
	out := make([]poolStatRow, 0, len(byID))
	for _, conn := range conns {
		out = append(out, byID[conn.ID.String()])
	}
	ok(w, r, out)
}

func (s *service) callConnectorPools(ctx context.Context, dbType string) ([]poolStatRow, error) {
	base, found := s.connectors[dbType]
	if !found {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/pools", bytes.NewReader(nil))
	if err != nil {
		return nil, err
	}
	if s.cfg.Gateway.InternalAuthSecret != "" {
		internalauth.SignHeaders(req, nil, s.cfg.Gateway.InternalAuthSecret, time.Now())
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var env struct {
		Success bool          `json:"success"`
		Data    []poolStatRow `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	return env.Data, nil
}
