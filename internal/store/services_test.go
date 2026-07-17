package store

import (
	"strings"
	"testing"
)

func TestServiceRegistrySQLUsesPaginationSearchAndSafeSort(t *testing.T) {
	p := ListParams{Page: 2, Limit: 10, Search: "api", SortBy: "name", SortDir: "asc"}
	sql := listServicesSQL(p)
	lower := strings.ToLower(sql)
	for _, want := range []string{"from service_registry", "name ilike", "kind ilike", "order by name asc", "limit $2 offset $3"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("list services SQL %q should contain %q", sql, want)
		}
	}
}

func TestServiceRegistrySQLRejectsUnsafeSort(t *testing.T) {
	p := ListParams{SortBy: "name;drop table service_registry", SortDir: "asc"}
	sql := strings.ToLower(listServicesSQL(p))
	if strings.Contains(sql, "drop table") {
		t.Fatalf("list services SQL should not include unsafe sort input: %q", sql)
	}
	if !strings.Contains(sql, "order by updated_at asc") {
		t.Fatalf("list services SQL should fall back to updated_at: %q", sql)
	}
}

func TestListPublishedAPIsSQLOnlySelectsLivePublishedDefinitions(t *testing.T) {
	query := strings.ToLower(listPublishedRevisionSnapshotsSQL)
	for _, want := range []string{
		"from api_revisions r",
		"join api_definitions a on a.id = r.api_definition_id",
		"where a.status = 'published'",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("published catalog SQL must contain %q: %s", want, listPublishedRevisionSnapshotsSQL)
		}
	}
}

func TestStaticConfiguredServicesHaveHealthReadinessAndCapabilities(t *testing.T) {
	services := StaticConfiguredServices(map[string]string{"postgres": "http://connector-postgres:8080"})
	if len(services) != 1 {
		t.Fatalf("services len = %d, want 1", len(services))
	}
	s := services[0]
	if s.Name != "connector-postgres" || s.Kind != "connector" || !s.Enabled {
		t.Fatalf("unexpected static service: %+v", s)
	}
	if s.HealthURL != "http://connector-postgres:8080/healthz" || s.ReadyURL != "http://connector-postgres:8080/readyz" || s.MetricsURL != "http://connector-postgres:8080/metrics" {
		t.Fatalf("static service missing health/ready/metrics urls: %+v", s)
	}
	if string(s.Capabilities) != `{"connector":"postgres"}` {
		t.Fatalf("capabilities = %s", string(s.Capabilities))
	}
}
