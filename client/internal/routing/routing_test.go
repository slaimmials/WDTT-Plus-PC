package routing

import (
	"context"
	"net/netip"
	"slices"
	"testing"
)

func fakeResolver(_ context.Context, host string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("203.0.113.7")}, nil
}

func contains(prefixes []netip.Prefix, addr string) bool {
	ip := netip.MustParseAddr(addr)
	return slices.ContainsFunc(prefixes, func(p netip.Prefix) bool { return p.Contains(ip) })
}

func TestAllRoutesSplitDefault(t *testing.T) {
	routes, warnings := Plan{Mode: All}.Build(context.Background(), fakeResolver)
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	want := []netip.Prefix{netip.MustParsePrefix("0.0.0.0/1"), netip.MustParsePrefix("128.0.0.0/1")}
	if !slices.Equal(routes, want) {
		t.Fatalf("got %v", routes)
	}
}

func TestExcludeRules(t *testing.T) {
	plan := Plan{
		Mode:      Exclude,
		Rules:     []string{"8.8.8.8", "1.1.1.0/24", "9.9.9.1-9.9.9.9", "https://example.com/path", "bad rule"},
		BypassLAN: true,
	}
	routes, warnings := plan.Build(context.Background(), fakeResolver)
	if len(warnings) != 1 {
		t.Fatalf("want one warning, got %v", warnings)
	}
	for _, addr := range []string{"8.8.8.8", "1.1.1.200", "9.9.9.5", "203.0.113.7", "192.168.1.1", "10.1.2.3"} {
		if contains(routes, addr) {
			t.Errorf("%s must bypass the tunnel", addr)
		}
	}
	for _, addr := range []string{"8.8.4.4", "9.9.9.10", "93.184.216.34"} {
		if !contains(routes, addr) {
			t.Errorf("%s must go through the tunnel", addr)
		}
	}
}

func TestIncludeRules(t *testing.T) {
	routes, _ := Plan{Mode: Include, Rules: []string{"youtube.com", "5.5.0.0/16"}}.Build(context.Background(), fakeResolver)
	if !contains(routes, "203.0.113.7") || !contains(routes, "5.5.1.1") || contains(routes, "8.8.8.8") {
		t.Fatalf("unexpected routes %v", routes)
	}
}

func TestValidate(t *testing.T) {
	for _, ok := range []string{"1.2.3.4", "10.0.0.0/8", "1.1.1.1-1.1.1.5", "vk.com", "https://vk.com/x"} {
		if err := Validate(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "::1", "localhost", "1.1.1.9-1.1.1.1"} {
		if err := Validate(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}
