package admin

import (
	"strings"
	"testing"
)

func TestLastJSONLineSkipsNoise(t *testing.T) {
	output := "sudo: warning\n{\"ok\":false}\nnoise\n{\"ok\":true,\"message\":\"done\"}\n"
	if got := string(lastJSONLine(output)); got != `{"ok":true,"message":"done"}` {
		t.Fatalf("got %q", got)
	}
	if lastJSONLine("no json here\n{broken}") != nil {
		t.Fatal("invalid JSON must be ignored")
	}
}

func TestFriendlyError(t *testing.T) {
	err := friendlyError("admin_socket_unavailable: dial unix /run/wdtt/admin.sock")
	if !strings.Contains(err.Error(), "служба wdtt") {
		t.Fatalf("got %v", err)
	}
	if got := friendlyError("  пароль не найден ").Error(); got != "пароль не найден" {
		t.Fatalf("got %q", got)
	}
}
