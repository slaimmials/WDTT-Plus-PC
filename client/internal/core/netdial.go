package core

import (
	"context"
	"net"
	"time"
)

// outboundDialer returns a dialer for transport traffic that must leave
// through the physical network, never through the VPN adapter.
func outboundDialer(timeout, keepAlive time.Duration) *net.Dialer {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: keepAlive}
	if control := socketCtrl.Load(); control != nil {
		dialer.Control = *control
	}
	return dialer
}

func listenOutboundUDP(network string, address *net.UDPAddr) (*net.UDPConn, error) {
	var config net.ListenConfig
	if control := socketCtrl.Load(); control != nil {
		config.Control = *control
	}
	conn, err := config.ListenPacket(context.Background(), network, address.String())
	if err != nil {
		return nil, err
	}
	return conn.(*net.UDPConn), nil
}
