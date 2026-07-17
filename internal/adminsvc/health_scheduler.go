package adminsvc

import (
	"context"
	"time"

	"github.com/ddag/ddag/internal/models"
)

func (s *service) connectorHealthLoop(ctx context.Context, every time.Duration) {
	check := func() {
		conns, err := s.store.ListAllConnections(ctx)
		if err != nil {
			s.log.Warn("connector_health_list_failed", "error", err.Error())
			return
		}
		for _, c := range conns {
			if c.Status != "active" {
				continue
			}
			s.checkConnectionHealth(ctx, c)
		}
	}
	check()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			check()
		}
	}
}

func (s *service) checkConnectionHealth(parent context.Context, c models.DatabaseConnection) {
	timeout := time.Duration(c.ConnectionTimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	password := ""
	if c.SecretRef != nil {
		if b, err := s.secrets.Get(ctx, *c.SecretRef); err == nil {
			password = string(b)
		}
	}
	res, err := s.callConnectorTest(ctx, c.DatabaseType, map[string]any{
		"host": c.Host, "port": c.Port, "database_name": c.DatabaseName,
		"service_name": c.ServiceName, "schema_name": c.SchemaName, "username": c.Username,
		"password": password, "ssl_mode": c.SSLMode, "connection_timeout_ms": c.ConnectionTimeoutMS,
	})
	status := connectionHealthStatus(res, err)
	if err := s.store.SetConnectionHealth(context.Background(), c.ID, status); err != nil {
		s.log.Warn("connector_health_update_failed", "connection_id", c.ID.String(), "error", err.Error())
	}
}
