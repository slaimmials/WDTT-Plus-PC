package deploy

import (
	"encoding/json"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := map[string]state{
		"WDTT_STANDALONE=1\nWDTT_MANAGED=1\nWDTT_TRACE=1": stateStandalone,
		"WDTT_STANDALONE=0\nWDTT_MANAGED=1\nWDTT_TRACE=1": stateManaged,
		"WDTT_PRESERVED=1\nWDTT_TRACE=1":                  statePreserved,
		"WDTT_TRACE=0":                                    stateNone,
		"WDTT_TRACE=1":                                    stateUnknown,
	}
	for output, want := range cases {
		if got := classify(output); got != want {
			t.Errorf("%q: got %s, want %s", output, got, want)
		}
	}
}

func TestChooseInstallMode(t *testing.T) {
	if mode, _ := chooseInstallMode(stateNone, ModeReset); mode != "fresh" {
		t.Errorf("empty server must get a fresh install, got %s", mode)
	}
	if mode, _ := chooseInstallMode(stateManaged, ModeInstall); mode != "preserve" {
		t.Errorf("managed server must be updated in place, got %s", mode)
	}
	if mode, _ := chooseInstallMode(stateUnknown, ModeReset); mode != "reset" {
		t.Errorf("reset must be allowed for unknown installs, got %s", mode)
	}
	if _, err := chooseInstallMode(stateUnknown, ModeInstall); err == nil {
		t.Error("unknown install must not be updated silently")
	}
	if _, err := chooseInstallMode(stateStandalone, ModeReset); err == nil {
		t.Error("standalone installs must be left to install.sh")
	}
}

func TestParseProgress(t *testing.T) {
	fraction, step, ok := parseProgress("WDTT_PROGRESS|0.60|Бинарник...")
	if !ok || fraction != 0.6 || step != "Бинарник..." {
		t.Fatalf("got %v %q %v", fraction, step, ok)
	}
	if _, _, ok := parseProgress("plain output"); ok {
		t.Fatal("plain output must be ignored")
	}
}

func TestInitialDatabase(t *testing.T) {
	raw, err := initialDatabase(Options{MainPassword: "secret", DNS: "1.1.1.1", DTLSPort: 56000, WGPort: 56001, LocalPort: 9000})
	if err != nil {
		t.Fatal(err)
	}
	var db map[string]any
	if err := json.Unmarshal(raw, &db); err != nil {
		t.Fatal(err)
	}
	if db["main_password"] != "secret" || db["default_ports"] != "56000,56001,9000" {
		t.Fatalf("unexpected database %v", db)
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	base := Options{MainPassword: "x", DTLSPort: 56000, WGPort: 56001, LocalPort: 9000, SSHPort: 22, DNS: "1.1.1.1,8.8.8.8"}
	if err := validate(base); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.DNS = "1.1.1.1; rm -rf /"
	if err := validate(bad); err == nil {
		t.Error("DNS with shell characters must be rejected")
	}
	bad = base
	bad.WGPort = bad.DTLSPort
	if err := validate(bad); err == nil {
		t.Error("equal ports must be rejected")
	}
}
