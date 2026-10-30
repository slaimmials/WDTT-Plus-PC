package connection

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchTrace(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "fl=1\nip=203.0.113.5\nloc=NL\ntls=TLSv1.3\n")
	}))
	defer server.Close()

	ip, country, err := fetchTrace(server.Client(), server.URL)
	if err != nil || ip != "203.0.113.5" || country != "NL" {
		t.Fatalf("got %q %q %v", ip, country, err)
	}
}

func TestFetchTraceRejectsUnexpectedBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<html>blocked</html>")
	}))
	defer server.Close()

	if _, _, err := fetchTrace(server.Client(), server.URL); err == nil {
		t.Fatal("expected error for a page without trace fields")
	}
}
