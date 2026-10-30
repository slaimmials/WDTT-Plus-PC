// Package routing turns user routing rules into the set of IPv4 prefixes that
// must be routed through the VPN adapter.
package routing

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"

	"go4.org/netipx"
)

type Mode string

const (
	All     Mode = "all"
	Exclude Mode = "exclude"
	Include Mode = "include"
)

// localNetworks are never sent into the tunnel when BypassLAN is set.
var localNetworks = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("255.255.255.255/32"),
}

type Resolver func(ctx context.Context, host string) ([]netip.Addr, error)

func SystemResolver(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip4", host)
}

type Plan struct {
	Mode      Mode
	Rules     []string
	BypassLAN bool
}

// Build returns the prefixes to route through the tunnel. Rules that cannot
// be parsed or resolved are reported in the returned warnings and skipped.
func (p Plan) Build(ctx context.Context, resolve Resolver) ([]netip.Prefix, []error) {
	var warnings []error
	var rules netipx.IPSetBuilder
	for _, rule := range p.Rules {
		prefixes, err := parseRule(ctx, rule, resolve)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("%s: %w", rule, err))
			continue
		}
		for _, prefix := range prefixes {
			rules.AddPrefix(prefix)
		}
	}
	ruleSet, _ := rules.IPSet()

	var routes netipx.IPSetBuilder
	switch p.Mode {
	case Include:
		routes.AddSet(ruleSet)
	case Exclude:
		routes.AddPrefix(netip.MustParsePrefix("0.0.0.0/0"))
		routes.RemoveSet(ruleSet)
	default:
		routes.AddPrefix(netip.MustParsePrefix("0.0.0.0/0"))
	}
	if p.BypassLAN {
		for _, prefix := range localNetworks {
			routes.RemovePrefix(prefix)
		}
	}
	set, _ := routes.IPSet()
	return splitDefault(set.Prefixes()), warnings
}

// splitDefault replaces 0.0.0.0/0 with two /1 halves so the tunnel route wins
// over the existing default route without deleting it.
func splitDefault(prefixes []netip.Prefix) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(prefixes)+1)
	for _, prefix := range prefixes {
		if prefix.Bits() == 0 {
			out = append(out, netip.MustParsePrefix("0.0.0.0/1"), netip.MustParsePrefix("128.0.0.0/1"))
			continue
		}
		out = append(out, prefix)
	}
	return out
}

func parseRule(ctx context.Context, raw string, resolve Resolver) ([]netip.Prefix, error) {
	rule := strings.TrimSpace(raw)
	if rule == "" {
		return nil, errors.New("пустое правило")
	}
	if prefix, err := netip.ParsePrefix(rule); err == nil {
		if !prefix.Addr().Is4() {
			return nil, errors.New("поддерживается только IPv4")
		}
		return []netip.Prefix{prefix.Masked()}, nil
	}
	if addr, err := netip.ParseAddr(rule); err == nil {
		if !addr.Is4() {
			return nil, errors.New("поддерживается только IPv4")
		}
		return []netip.Prefix{netip.PrefixFrom(addr, 32)}, nil
	}
	if from, to, ok := strings.Cut(rule, "-"); ok && looksLikeIP(from) && looksLikeIP(to) {
		r, err := parseRange(from, to)
		if err != nil {
			return nil, err
		}
		return r.Prefixes(), nil
	}
	host, err := ruleHost(rule)
	if err != nil {
		return nil, err
	}
	addrs, err := resolve(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("не удалось разрешить %s: %w", host, err)
	}
	prefixes := make([]netip.Prefix, 0, len(addrs))
	for _, addr := range addrs {
		if addr.Is4() {
			prefixes = append(prefixes, netip.PrefixFrom(addr, 32))
		}
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("у %s нет IPv4-адресов", host)
	}
	return prefixes, nil
}

func parseRange(from, to string) (netipx.IPRange, error) {
	start, err := netip.ParseAddr(strings.TrimSpace(from))
	if err != nil {
		return netipx.IPRange{}, err
	}
	end, err := netip.ParseAddr(strings.TrimSpace(to))
	if err != nil {
		return netipx.IPRange{}, err
	}
	r := netipx.IPRangeFrom(start, end)
	if !start.Is4() || !end.Is4() || !r.IsValid() {
		return netipx.IPRange{}, errors.New("некорректный диапазон IPv4")
	}
	return r, nil
}

func looksLikeIP(value string) bool {
	_, err := netip.ParseAddr(strings.TrimSpace(value))
	return err == nil
}

func ruleHost(rule string) (string, error) {
	value := rule
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Hostname() == "" {
			return "", errors.New("некорректный URL")
		}
		value = parsed.Hostname()
	} else if i := strings.IndexAny(value, "/:"); i >= 0 {
		value = value[:i]
	}
	value = strings.TrimSuffix(strings.ToLower(value), ".")
	if !strings.Contains(value, ".") || strings.ContainsAny(value, " \t") {
		return "", errors.New("нужен IPv4, подсеть, диапазон, домен или URL")
	}
	return value, nil
}

// Validate reports rules that are syntactically invalid without resolving
// domains.
func Validate(rule string) error {
	_, err := parseRule(context.Background(), rule, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.IPv4Unspecified()}, nil
	})
	return err
}
