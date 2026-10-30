package profile

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	settings, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(settings)
	if strings.Contains(string(raw), "null") {
		t.Fatalf("default settings must not contain null lists: %s", raw)
	}

	p := FromLink(Link{Host: "1.2.3.4", DTLSPort: 56000, WGPort: 56001, LocalPort: 9000, Password: "x", Hashes: []string{"h"}})
	settings.Profiles = append(settings.Profiles, p)
	settings.ActiveProfile = p.ID
	if err := store.Save(settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != 1 || loaded.Profiles[0].Password != "x" || loaded.DeviceID != settings.DeviceID {
		t.Fatalf("unexpected settings after reload: %+v", loaded)
	}
}
