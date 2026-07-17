package adminsvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/store"
)

type serviceProbeResult struct {
	Status       string
	Message      string
	Version      string
	CommitSHA    string
	Capabilities json.RawMessage
}

func (s *service) listServices(w http.ResponseWriter, r *http.Request) {
	p := listParams(r)
	if err := s.ensureStaticServices(r.Context()); err != nil {
		storeErr(w, r, err)
		return
	}
	rows, total, err := s.store.ListServices(r.Context(), p)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	list(w, r, rows, p, total)
}

func (s *service) createService(w http.ResponseWriter, r *http.Request) {
	var in models.Service
	if !decode(w, r, &in) {
		return
	}
	if in.ManagedBy == "" {
		in.ManagedBy = "admin"
	}
	if in.Capabilities == nil {
		in.Capabilities = []byte(`{}`)
	}
	id, err := s.store.UpsertService(r.Context(), &in)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	row, err := s.store.GetService(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, row)
}

func (s *service) updateService(w http.ResponseWriter, r *http.Request) {
	id, okID := idParam(w, r)
	if !okID {
		return
	}
	var in models.Service
	if !decode(w, r, &in) {
		return
	}
	in.ID = id
	if in.ManagedBy == "" {
		in.ManagedBy = "admin"
	}
	if in.Capabilities == nil {
		in.Capabilities = []byte(`{}`)
	}
	if err := s.store.UpdateService(r.Context(), &in); err != nil {
		storeErr(w, r, err)
		return
	}
	row, err := s.store.GetService(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, row)
}

func (s *service) refreshService(w http.ResponseWriter, r *http.Request) {
	id, okID := idParam(w, r)
	if !okID {
		return
	}
	row, err := s.store.GetService(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	probe := serviceProbeResult{Status: "unknown"}
	if !row.Enabled {
		probe.Status = "disabled"
	} else if row.ReadyURL != "" || row.HealthURL != "" {
		probe = s.probeService(r.Context(), row)
	}
	if err := s.store.SetServiceHealthMetadata(r.Context(), id, probe.Status, probe.Message, time.Now(), probe.Version, probe.CommitSHA, probe.Capabilities); err != nil {
		storeErr(w, r, err)
		return
	}
	row, _ = s.store.GetService(r.Context(), id)
	ok(w, r, row)
}

func (s *service) ensureStaticServices(ctx context.Context) error {
	for _, svc := range store.StaticConfiguredServices(s.connectors) {
		if _, err := s.store.UpsertService(ctx, &svc); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) probeService(ctx context.Context, row *models.Service) serviceProbeResult {
	url := row.ReadyURL
	if url == "" {
		url = row.HealthURL
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(url), nil)
	if err != nil {
		return serviceProbeResult{Status: "degraded", Message: err.Error()}
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		return serviceProbeResult{Status: "degraded", Message: err.Error()}
	}
	defer resp.Body.Close()
	probe := serviceProbeResult{Status: "degraded", Message: resp.Status}
	if resp.Body != nil {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if len(body) > 0 && json.Valid(body) {
			var meta struct {
				Version      string          `json:"version"`
				CommitSHA    string          `json:"commit_sha"`
				Commit       string          `json:"commit"`
				Capabilities json.RawMessage `json:"capabilities"`
			}
			if json.Unmarshal(body, &meta) == nil {
				probe.Version = meta.Version
				probe.CommitSHA = meta.CommitSHA
				if probe.CommitSHA == "" {
					probe.CommitSHA = meta.Commit
				}
				probe.Capabilities = meta.Capabilities
			}
		}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		probe.Status = "healthy"
		probe.Message = ""
		return probe
	}
	return probe
}
