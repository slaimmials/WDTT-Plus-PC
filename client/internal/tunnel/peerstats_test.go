package tunnel

import "testing"

func TestParsePeerStats(t *testing.T) {
	stats := parsePeerStats("private_key=00\npublic_key=11\nlast_handshake_time_sec=1700000000\nlast_handshake_time_nsec=5\ntx_bytes=1480\nrx_bytes=92\n")
	if stats.TxBytes != 1480 || stats.RxBytes != 92 || stats.LastHandshake.Unix() != 1700000000 {
		t.Fatalf("unexpected stats %+v", stats)
	}
	if !parsePeerStats("").LastHandshake.IsZero() {
		t.Fatal("missing handshake must stay zero")
	}
}
