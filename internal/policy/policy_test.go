package policy

import "testing"

func TestHasScope(t *testing.T) {
	cases := []struct {
		token, required string
		want            bool
	}{
		{"brim.site.read brim.wo.read", "brim.site.read", true},
		{"brim.site.read", "brim.wo.read", false},
		{"", "brim.site.read", false},
		{"anything", "", true}, // no required scope → always allowed
		{"a b c", "c", true},
	}
	for _, c := range cases {
		if got := HasScope(c.token, c.required); got != c.want {
			t.Errorf("HasScope(%q,%q)=%v want %v", c.token, c.required, got, c.want)
		}
	}
}

func TestIPAllowed(t *testing.T) {
	cases := []struct {
		ip    string
		list  []string
		want  bool
		label string
	}{
		{"10.10.10.5", nil, false, "empty list denies all because whitelist is mandatory"},
		{"10.10.10.5", []string{"10.10.10.0/24"}, true, "in cidr"},
		{"10.10.11.5", []string{"10.10.10.0/24"}, false, "outside cidr"},
		{"103.20.30.40", []string{"103.20.30.40"}, true, "single ip match"},
		{"103.20.30.41", []string{"103.20.30.40"}, false, "single ip mismatch"},
		{"not-an-ip", []string{"10.0.0.0/8"}, false, "invalid ip"},
		{"10.0.0.1", []string{"bad-cidr", "10.0.0.0/8"}, true, "skips bad entry, matches next"},
	}
	for _, c := range cases {
		if got := IPAllowed(c.ip, c.list); got != c.want {
			t.Errorf("%s: IPAllowed(%q,%v)=%v want %v", c.label, c.ip, c.list, got, c.want)
		}
	}
}

func TestWindowsFromRule(t *testing.T) {
	ws := WindowsFromRule(10, 300, 0, 100000)
	if len(ws) != 4 {
		t.Fatalf("want 4 windows, got %d", len(ws))
	}
	if ws[0].Limit != 10 || ws[1].Limit != 300 || ws[2].Limit != 0 || ws[3].Limit != 100000 {
		t.Fatalf("unexpected limits: %+v", ws)
	}
	if ws[0].Suffix != "s" || ws[3].Suffix != "d" {
		t.Fatalf("unexpected suffixes: %+v", ws)
	}
}
