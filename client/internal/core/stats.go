package core

import (
	"context"
	"sync/atomic"
	"time"
)

var globalActiveConnections atomic.Int32

type transferStats struct {
	ActiveConnections atomic.Int32
	TotalBytesUp      atomic.Int64
	TotalBytesDown    atomic.Int64
	latency           atomic.Int64
}

// noteLatency keeps a smoothed keepalive round-trip time over all channels.
func (s *transferStats) noteLatency(rtt time.Duration) {
	for {
		old := s.latency.Load()
		next := int64(rtt)
		if old > 0 {
			next = (old*3 + int64(rtt)) / 4
		}
		if s.latency.CompareAndSwap(old, next) {
			return
		}
	}
}

func newTransferStats() *transferStats {
	return &transferStats{}
}

func (s *transferStats) snapshot() Stats {
	return Stats{
		ActiveConnections: s.ActiveConnections.Load(),
		BytesUp:           s.TotalBytesUp.Load(),
		BytesDown:         s.TotalBytesDown.Load(),
		LatencyMs:         time.Duration(s.latency.Load()).Milliseconds(),
	}
}

func (s *transferStats) run(ctx context.Context, report func(Stats)) {
	if report == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report(s.snapshot())
		}
	}
}
