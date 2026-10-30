//go:build !windows

package tunnel

import (
	"errors"
	"net/netip"
	"syscall"
)

var errUnsupported = errors.New("системный VPN поддерживается только в Windows")

type Binder struct{}

func NewBinder(func()) (*Binder, error) { return &Binder{}, nil }

func (*Binder) Close() {}

func (*Binder) Control(string, string, syscall.RawConn) error { return nil }

type Options struct {
	Config     WireGuardConfig
	Routes     []netip.Prefix
	KillSwitch bool
	Binder     *Binder
}

type Tunnel struct{ Warning string }

func Start(Options) (*Tunnel, error) { return nil, errUnsupported }

func (*Tunnel) PeerStats() (PeerStats, error) { return PeerStats{}, errUnsupported }

func (*Tunnel) Close() {}

func IsElevated() bool { return false }
