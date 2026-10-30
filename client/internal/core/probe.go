package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const probeTimeout = 45 * time.Second

// Probe connects to the server through VK TURN like a normal session and
// returns the round trip of the first keepalive over the full path.
func Probe(ctx context.Context, cfg Config, hooks Hooks) (time.Duration, error) {
	cfg.Mode = ModeVPN
	cfg.Workers = workersPerGroup
	cfg.ListenAddr = "127.0.0.1:0"

	latency := make(chan time.Duration, 1)
	probeHooks := Hooks{
		SolveCaptcha: hooks.SolveCaptcha,
		OnStats: func(stats Stats) {
			if stats.LatencyMs > 0 {
				select {
				case latency <- time.Duration(stats.LatencyMs) * time.Millisecond:
				default:
				}
			}
		},
	}
	session, err := Start(ctx, cfg, probeHooks)
	if err != nil {
		return 0, err
	}
	defer func() {
		session.Stop()
		<-session.Done()
	}()

	timer := time.NewTimer(probeTimeout)
	defer timer.Stop()
	select {
	case rtt := <-latency:
		return rtt, nil
	case <-session.Done():
		if session.err != nil {
			return 0, session.err
		}
		return 0, errors.New("соединение с сервером завершилось")
	case <-timer.C:
		return 0, fmt.Errorf("сервер не ответил за %d с", int(probeTimeout.Seconds()))
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
