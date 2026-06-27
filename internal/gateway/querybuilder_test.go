package gateway

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
)

func TestBuildDynamicQueryAppliesSafeFiltersSortAndJoins(t *testing.T) {
	api := apiWithBuilder(t, map[string]any{
		"base_table": "karyawan",
		"select": []any{
			"karyawan.id",
			"karyawan.nama",
			"COUNT(t.id) AS total_transaksi",
		},
		"joins": []any{map[string]any{
			"type":  "left",
			"table": "transaksi",
			"alias": "t",
			"on": map[string]any{
				"left":     "karyawan.id",
				"operator": "=",
				"right":    "t.karyawan_id",
			},
		}},
		"group_by": []any{"karyawan.id", "karyawan.nama"},
		"filters": []any{
			map[string]any{"name": "nama", "column": "karyawan.nama", "type": "string", "operators": []any{"eq", "like"}},
			map[string]any{"name": "status", "column": "karyawan.status", "type": "string", "operators": []any{"eq", "in"}},
			map[string]any{"name": "created_at", "column": "karyawan.created_at", "type": "date", "operators": []any{"between"}},
			map[string]any{"name": "deleted_at", "column": "karyawan.deleted_at", "type": "date", "operators": []any{"isnull"}},
		},
		"sortable_columns": []any{
			map[string]any{"name": "created_at", "column": "karyawan.created_at"},
			map[string]any{"name": "nama", "column": "karyawan.nama"},
		},
	})
	q := url.Values{
		"nama":       {"like:heri"},
		"status":     {"in:active,pending"},
		"created_at": {"between:2026-01-01,2026-12-31"},
		"deleted_at": {"isnull:true"},
		"sort":       {"-created_at,nama"},
	}

	built, apiErr := BuildDynamicQuery(api, q, map[string]any{"tenant_id": "internal"})
	if apiErr != nil {
		t.Fatalf("BuildDynamicQuery error = %v", apiErr)
	}

	wantSQL := "SELECT karyawan.id, karyawan.nama, COUNT(t.id) AS total_transaksi FROM karyawan LEFT JOIN transaksi t ON karyawan.id = t.karyawan_id WHERE karyawan.nama LIKE :ddag_filter_nama_0 AND karyawan.status IN (:ddag_filter_status_0, :ddag_filter_status_1) AND karyawan.created_at BETWEEN :ddag_filter_created_at_0 AND :ddag_filter_created_at_1 AND karyawan.deleted_at IS NULL GROUP BY karyawan.id, karyawan.nama ORDER BY karyawan.created_at DESC, karyawan.nama ASC"
	if built.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\n got: %s", wantSQL, built.SQL)
	}
	if built.Params["ddag_filter_nama_0"] != "%heri%" ||
		built.Params["ddag_filter_status_0"] != "active" ||
		built.Params["ddag_filter_status_1"] != "pending" ||
		built.Params["ddag_filter_created_at_0"] != "2026-01-01" ||
		built.Params["ddag_filter_created_at_1"] != "2026-12-31" ||
		built.Params["tenant_id"] != "internal" {
		t.Fatalf("params = %+v", built.Params)
	}
}

func TestBuildDynamicQueryRejectsInvalidFilterAndSort(t *testing.T) {
	api := apiWithBuilder(t, map[string]any{
		"base_table": "users",
		"select":     []any{"users.id", "users.name"},
		"filters": []any{
			map[string]any{"name": "status", "column": "users.status", "operators": []any{"eq"}},
		},
		"sortable_columns": []any{
			map[string]any{"name": "name", "column": "users.name"},
		},
	})

	_, apiErr := BuildDynamicQuery(api, url.Values{"status": {"like:%admin%"}}, nil)
	if apiErr == nil || apiErr.Code != httpx.CodeQueryValidationFailed {
		t.Fatalf("invalid operator error = %#v", apiErr)
	}

	_, apiErr = BuildDynamicQuery(api, url.Values{"sort": {"password_hash"}}, nil)
	if apiErr == nil || !strings.Contains(apiErr.Message, "sort") {
		t.Fatalf("invalid sort error = %#v", apiErr)
	}
}

func TestBuildDynamicQueryKeepsInjectionValuesBound(t *testing.T) {
	api := apiWithBuilder(t, map[string]any{
		"base_table": "users",
		"select":     []any{"users.id", "users.name"},
		"filters": []any{
			map[string]any{"name": "name", "column": "users.name", "operators": []any{"eq"}},
		},
	})
	injection := "x' OR '1'='1"

	built, apiErr := BuildDynamicQuery(api, url.Values{"name": {"eq:" + injection}}, nil)
	if apiErr != nil {
		t.Fatalf("BuildDynamicQuery error = %v", apiErr)
	}
	if strings.Contains(built.SQL, injection) {
		t.Fatalf("SQL leaked raw input: %s", built.SQL)
	}
	if built.Params["ddag_filter_name_0"] != injection {
		t.Fatalf("bound param = %+v", built.Params)
	}
}

func apiWithBuilder(t *testing.T, cfg map[string]any) models.APIDefinition {
	t.Helper()
	b, err := json.Marshal(map[string]any{"query_builder": cfg})
	if err != nil {
		t.Fatal(err)
	}
	return models.APIDefinition{
		Name:            "builder",
		Method:          "GET",
		Path:            "/api/v1/users",
		ResponseMapping: b,
		QueryTemplate:   "",
		DefaultLimit:    20,
		MaxLimit:        100,
	}
}
