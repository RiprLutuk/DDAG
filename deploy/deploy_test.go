package deploy_test

import (
	"os"
	"strings"
	"testing"
)

func TestK8sDashboardIsStaticAndUsesIngressAdminPrefix(t *testing.T) {
	b, err := os.ReadFile("k8s/dashboard.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dashboard := string(b)
	if strings.Contains(dashboard, "NUXT_") || strings.Contains(dashboard, "VITE_") {
		t.Fatal("static dashboard must not depend on runtime frontend environment variables")
	}

	b, err = os.ReadFile("k8s/ingress.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ingress := string(b)
	if !strings.Contains(ingress, "path: /admin") || !strings.Contains(ingress, "name: admin-backend") {
		t.Fatal("ingress should route the /admin prefix to admin-backend")
	}
}

func TestHelmValuesIncludeDashboardService(t *testing.T) {
	b, err := os.ReadFile("helm/ddag/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	values := string(b)
	for _, want := range []string{"dashboard:", "ddag-dashboard", "env: {}"} {
		if !strings.Contains(values, want) {
			t.Fatalf("helm values should contain %q", want)
		}
	}
}

func TestVPSDocsUseAdminPrefixForDashboardAPI(t *testing.T) {
	b, err := os.ReadFile("../docs/DEPLOY_VPS.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	if !strings.Contains(doc, "static Vue 3 + Vite dashboard") || !strings.Contains(doc, "apps/dashboard/dist") {
		t.Fatal("VPS docs should document the static Vite dashboard served by Caddy")
	}
}
