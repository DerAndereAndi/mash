package discovery

import "testing"

func TestNormalizeHostname(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MacBookPro16.local", "MacBookPro16"},
		{"MacBookPro16.local.", "MacBookPro16"},
		{"myhost", "myhost"},
		{"myhost.", "myhost"},
		{"", ""},
		{"host.example.com", "host.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeHostname(tt.input)
			if got != tt.want {
				t.Errorf("normalizeHostname(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInterfaceIPs_ReturnsAddresses(t *testing.T) {
	ips := interfaceIPs(nil)
	if len(ips) == 0 {
		t.Fatal("expected at least one IP from interfaceIPs(nil)")
	}
}

func TestResolvedHost_WithHint(t *testing.T) {
	got := resolvedHost("myhost.local")
	if got != "myhost" {
		t.Errorf("resolvedHost(%q) = %q, want %q", "myhost.local", got, "myhost")
	}
}

func TestResolvedHost_EmptyHint_FallsBackToHostname(t *testing.T) {
	got := resolvedHost("")
	if got == "" {
		t.Error("resolvedHost(\"\") should not return empty string")
	}
	// Should be either os.Hostname() (normalized) or "mash-device"
	if got == "mash-device" {
		return // fallback is valid
	}
	// Must not contain ".local" suffix (normalized away)
	if len(got) > 6 && got[len(got)-6:] == ".local" {
		t.Errorf("resolvedHost(\"\") = %q, should not end with .local", got)
	}
}
