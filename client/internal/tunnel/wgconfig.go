// Package tunnel runs the system VPN adapter: userspace WireGuard on a wintun
// interface with routes, DNS and a leak-protection firewall.
package tunnel

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// WireGuardConfig is the subset of a wg-quick config issued by the server.
type WireGuardConfig struct {
	PrivateKey string
	Addresses  []netip.Prefix
	DNS        []netip.Addr
	MTU        int

	PeerPublicKey string
	AllowedIPs    []netip.Prefix
	Endpoint      string
	Keepalive     int
}

func ParseWireGuardConfig(text string) (WireGuardConfig, error) {
	cfg := WireGuardConfig{MTU: 1280}
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		var err error
		switch section + "." + key {
		case "interface.privatekey":
			cfg.PrivateKey = value
		case "interface.address":
			cfg.Addresses, err = parsePrefixList(value)
		case "interface.dns":
			for _, item := range splitList(value) {
				addr, parseErr := netip.ParseAddr(item)
				if parseErr != nil {
					return cfg, fmt.Errorf("DNS %q: %w", item, parseErr)
				}
				cfg.DNS = append(cfg.DNS, addr)
			}
		case "interface.mtu":
			cfg.MTU, err = strconv.Atoi(value)
		case "peer.publickey":
			cfg.PeerPublicKey = value
		case "peer.allowedips":
			cfg.AllowedIPs, err = parsePrefixList(value)
		case "peer.endpoint":
			cfg.Endpoint = value
		case "peer.persistentkeepalive":
			cfg.Keepalive, err = strconv.Atoi(value)
		}
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", key, err)
		}
	}
	if cfg.PrivateKey == "" || cfg.PeerPublicKey == "" || cfg.Endpoint == "" || len(cfg.Addresses) == 0 {
		return cfg, errors.New("в конфигурации WireGuard не хватает обязательных полей")
	}
	return cfg, nil
}

func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func parsePrefixList(value string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, item := range splitList(value) {
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			addr, addrErr := netip.ParseAddr(item)
			if addrErr != nil {
				return nil, err
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

// UAPI renders the config in the wireguard-go IPC format.
func (cfg WireGuardConfig) UAPI() (string, error) {
	privateKey, err := keyToHex(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("PrivateKey: %w", err)
	}
	publicKey, err := keyToHex(cfg.PeerPublicKey)
	if err != nil {
		return "", fmt.Errorf("PublicKey: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "private_key=%s\nreplace_peers=true\n", privateKey)
	fmt.Fprintf(&b, "public_key=%s\nendpoint=%s\nreplace_allowed_ips=true\n", publicKey, cfg.Endpoint)
	if cfg.Keepalive > 0 {
		fmt.Fprintf(&b, "persistent_keepalive_interval=%d\n", cfg.Keepalive)
	}
	for _, prefix := range cfg.AllowedIPs {
		fmt.Fprintf(&b, "allowed_ip=%s\n", prefix)
	}
	return b.String(), nil
}

func keyToHex(key string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(raw) != 32 {
		return "", errors.New("некорректный ключ")
	}
	return hex.EncodeToString(raw), nil
}
