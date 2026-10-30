package sshx

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func runPlan(t *testing.T, command, password, stdin string) string {
	t.Helper()
	wrapped, payload, err := rootPlan(command, password, []byte(stdin))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", wrapped)
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return string(out)
}

func TestRootPlanForwardsOnlyPayload(t *testing.T) {
	if out, _ := exec.Command("id", "-u").Output(); strings.TrimSpace(string(out)) != "0" {
		t.Skip("requires root to exercise the non-sudo branch")
	}
	got := runPlan(t, "cat; echo \"arg=$0\"", "pa'ss", "line1\nline2\n")
	if got != "line1\nline2\narg=bash\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestQuote(t *testing.T) {
	out, err := exec.Command("bash", "-c", "printf %s "+Quote(`it's "$HOME" \n`)).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `it's "$HOME" \n` {
		t.Fatalf("got %q", out)
	}
}

func TestMarker(t *testing.T) {
	if got := Marker("a=1\nKEY=old\n  KEY=new\n", "KEY"); got != "new" {
		t.Fatalf("got %q", got)
	}
}
