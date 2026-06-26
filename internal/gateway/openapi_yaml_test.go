package gateway

import (
	"strings"
	"testing"

	"github.com/ddag/ddag/internal/models"
)

func TestGenerateOpenAPIYAML(t *testing.T) {
	b, err := GenerateOpenAPIYAML([]models.APIDefinition{{
		Name:          "List sites",
		Namespace:     "brim",
		Path:          "/api/v1/sites",
		Method:        "GET",
		RequiredScope: "brim.site.read",
	}})
	if err != nil {
		t.Fatalf("GenerateOpenAPIYAML: %v", err)
	}
	out := string(b)
	for _, want := range []string{"openapi: 3.0.3", "/api/v1/sites:", "bearerAuth:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("YAML missing %q:\n%s", want, out)
		}
	}
}
