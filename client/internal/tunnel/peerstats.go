package tunnel

import (
	"bufio"
	"strconv"
	"strings"
	"time"
)

// PeerStats are the WireGuard counters of the server peer.
type PeerStats struct {
	TxBytes       int64
	RxBytes       int64
	LastHandshake time.Time
}

func parsePeerStats(uapi string) PeerStats {
	var stats PeerStats
	var sec, nsec int64
	scanner := bufio.NewScanner(strings.NewReader(uapi))
	for scanner.Scan() {
		key, value, _ := strings.Cut(scanner.Text(), "=")
		n, _ := strconv.ParseInt(value, 10, 64)
		switch key {
		case "tx_bytes":
			stats.TxBytes += n
		case "rx_bytes":
			stats.RxBytes += n
		case "last_handshake_time_sec":
			sec = n
		case "last_handshake_time_nsec":
			nsec = n
		}
	}
	if sec > 0 {
		stats.LastHandshake = time.Unix(sec, nsec)
	}
	return stats
}
