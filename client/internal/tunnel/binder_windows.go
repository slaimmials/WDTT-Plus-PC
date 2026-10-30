package tunnel

import (
	"encoding/binary"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

const (
	ipUnicastIf   = 31
	ipv6UnicastIf = 31
)

// Binder tracks the physical default interface and pins transport sockets to
// it, so traffic to VK and the WDTT server never enters the VPN adapter.
type Binder struct {
	mu        sync.Mutex
	tunnel    winipcfg.LUID
	index4    atomic.Uint32
	index6    atomic.Uint32
	onChange  func()
	callbacks []winipcfg.ChangeCallback
}

// NewBinder starts watching default routes; onChange is called when the
// physical default interface changes.
func NewBinder(onChange func()) (*Binder, error) {
	b := &Binder{onChange: onChange}
	b.refresh()
	routes, err := winipcfg.RegisterRouteChangeCallback(func(_ winipcfg.MibNotificationType, route *winipcfg.MibIPforwardRow2) {
		if route != nil && route.DestinationPrefix.PrefixLength == 0 {
			b.refresh()
		}
	})
	if err != nil {
		return nil, err
	}
	ifaces, err := winipcfg.RegisterInterfaceChangeCallback(func(notification winipcfg.MibNotificationType, _ *winipcfg.MibIPInterfaceRow) {
		if notification == winipcfg.MibParameterNotification {
			b.refresh()
		}
	})
	if err != nil {
		_ = routes.Unregister()
		return nil, err
	}
	b.callbacks = []winipcfg.ChangeCallback{routes, ifaces}
	return b, nil
}

func (b *Binder) Close() {
	for _, cb := range b.callbacks {
		_ = cb.Unregister()
	}
}

func (b *Binder) setTunnel(luid winipcfg.LUID) {
	b.mu.Lock()
	b.tunnel = luid
	b.mu.Unlock()
	b.refresh()
}

func (b *Binder) refresh() {
	b.mu.Lock()
	tunnel := b.tunnel
	b.mu.Unlock()

	changed := false
	if index, err := defaultInterface(windows.AF_INET, tunnel); err == nil {
		changed = b.index4.Swap(index) != index || changed
	}
	if index, err := defaultInterface(windows.AF_INET6, tunnel); err == nil {
		changed = b.index6.Swap(index) != index || changed
	}
	if changed && b.onChange != nil {
		go b.onChange()
	}
}

func defaultInterface(family winipcfg.AddressFamily, exclude winipcfg.LUID) (uint32, error) {
	routes, err := winipcfg.GetIPForwardTable2(family)
	if err != nil {
		return 0, err
	}
	lowest := ^uint64(0)
	index := uint32(0)
	for i := range routes {
		route := &routes[i]
		if route.DestinationPrefix.PrefixLength != 0 || route.InterfaceLUID == exclude {
			continue
		}
		row, err := route.InterfaceLUID.Interface()
		if err != nil || row.OperStatus != winipcfg.IfOperStatusUp {
			continue
		}
		iface, err := route.InterfaceLUID.IPInterface(family)
		if err != nil {
			continue
		}
		if metric := uint64(route.Metric) + uint64(iface.Metric); metric < lowest {
			lowest = metric
			index = route.InterfaceIndex
		}
	}
	return index, nil
}

// Control is a net.Dialer/ListenConfig control function.
func (b *Binder) Control(network, address string, conn syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	addr, err := netip.ParseAddr(host)
	if err == nil && addr.IsLoopback() {
		return nil
	}
	ipv6 := network == "tcp6" || network == "udp6" || (err == nil && addr.Is6() && !addr.Is4In6())

	var sockErr error
	controlErr := conn.Control(func(fd uintptr) {
		handle := windows.Handle(fd)
		if ipv6 {
			if index := b.index6.Load(); index != 0 {
				sockErr = windows.SetsockoptInt(handle, windows.IPPROTO_IPV6, ipv6UnicastIf, int(index))
			}
			return
		}
		if index := b.index4.Load(); index != 0 {
			var be [4]byte
			binary.BigEndian.PutUint32(be[:], index)
			sockErr = windows.SetsockoptInt(handle, windows.IPPROTO_IP, ipUnicastIf, int(binary.NativeEndian.Uint32(be[:])))
		}
	})
	if controlErr != nil {
		return controlErr
	}
	return sockErr
}
