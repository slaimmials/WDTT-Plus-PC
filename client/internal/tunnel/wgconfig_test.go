package tunnel

import (
	"strings"
	"testing"
)

const sampleConfig = `# WDTT_TRANSPORTS=uplink2
[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.66.66.2/32
DNS = 1.1.1.1, 8.8.8.8
MTU = 1280

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
Endpoint = 127.0.0.1:9000
PersistentKeepalive = 25`

func TestParseWireGuardConfig(t *testing.T) {
	cfg, err := ParseWireGuardConfig(sampleConfig)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MTU != 1280 || len(cfg.DNS) != 2 || cfg.Endpoint != "127.0.0.1:9000" || cfg.Keepalive != 25 {
		t.Fatalf("unexpected config %+v", cfg)
	}
	uapi, err := cfg.UAPI()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"private_key=c809f3e5", "endpoint=127.0.0.1:9000", "allowed_ip=0.0.0.0/0", "persistent_keepalive_interval=25"} {
		if !strings.Contains(uapi, want) {
			t.Errorf("UAPI misses %q:\n%s", want, uapi)
		}
	}
}

func TestParseWireGuardConfigRequiresKeys(t *testing.T) {
	if _, err := ParseWireGuardConfig("[Interface]\nAddress = 10.0.0.2/32\n"); err == nil {
		t.Fatal("expected error")
	}
}
