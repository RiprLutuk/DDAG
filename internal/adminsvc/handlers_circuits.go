package adminsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/internalauth"
)

type circuitRow struct {
	ConnectionID string `json:"connection_id"`
	Connection   string `json:"connection"`
	DBType       string `json:"db_type"`
	State        string `json:"state"`
	Status       string `json:"status"`
}

func (s *service) listCircuitBreakers(w http.ResponseWriter, r *http.Request) {
	conns, err := s.store.ListAllConnections(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	states := map[string]string{}
	for dbType := range s.connectors {
		rows, err := s.callConnectorCircuits(r.Context(), dbType)
		if err != nil {
			s.log.Warn("connector_circuit_state_failed", "db_type", dbType, "error", err.Error())
			continue
		}
		for _, row := range rows {
			states[row.ConnectionID] = row.State
		}
	}
	out := make([]circuitRow, 0, len(conns))
	for _, conn := range conns {
		state := states[conn.ID.String()]
		if state == "" {
			state = "closed"
		}
		out = append(out, circuitRow{
			ConnectionID: conn.ID.String(),
			Connection:   conn.Name,
			DBType:       conn.DatabaseType,
			State:        state,
			Status:       conn.Status,
		})
	}
	ok(w, r, out)
}

func (s *service) callConnectorCircuits(ctx context.Context, dbType string) ([]circuitRow, error) {
	base, found := s.connectors[dbType]
	if !found {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/circuits", bytes.NewReader(nil))
	if err != nil {
		return nil, err
	}
	req.Header.Set(httpx.RequestIDHeader, "")
	if s.cfg.Gateway.InternalAuthSecret != "" {
		internalauth.SignHeaders(req, nil, s.cfg.Gateway.InternalAuthSecret, time.Now())
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var env struct {
		Success bool         `json:"success"`
		Data    []circuitRow `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	return env.Data, nil
}
