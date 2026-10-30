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
}

func newTransferStats() *transferStats {
	return &transferStats{}
}

func (s *transferStats) snapshot() Stats {
	return Stats{
		ActiveConnections: s.ActiveConnections.Load(),
		BytesUp:           s.TotalBytesUp.Load(),
		BytesDown:         s.TotalBytesDown.Load(),
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
