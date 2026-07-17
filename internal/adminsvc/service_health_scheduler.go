package adminsvc

import (
	"context"
	"time"

	"github.com/ddag/ddag/internal/store"
)

func defaultServiceHealthListParams() store.ListParams {
	p := store.ListParams{Page: 1, Limit: 500, SortBy: "updated_at", SortDir: "desc"}
	p.Normalize()
	return p
}

func (s *service) serviceHealthLoop(ctx context.Context, every time.Duration) {
	check := func() {
		if err := s.ensureStaticServices(ctx); err != nil {
			s.log.Warn("service_health_seed_failed", "error", err.Error())
			return
		}
		rows, _, err := s.store.ListServices(ctx, defaultServiceHealthListParams())
		if err != nil {
			s.log.Warn("service_health_list_failed", "error", err.Error())
			return
		}
		for _, row := range rows {
			probe := serviceProbeResult{Status: "unknown"}
			if !row.Enabled {
				probe.Status = "disabled"
			} else if row.ReadyURL != "" || row.HealthURL != "" {
				probe = s.probeService(ctx, &row)
			}
			if err := s.store.SetServiceHealthMetadata(context.Background(), row.ID, probe.Status, probe.Message, time.Now(), probe.Version, probe.CommitSHA, probe.Capabilities); err != nil {
				s.log.Warn("service_health_update_failed", "service_id", row.ID.String(), "error", err.Error())
			}
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
