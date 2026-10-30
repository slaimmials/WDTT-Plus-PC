package tunnel

import (
	"errors"
	"fmt"
	"log"
	"net/netip"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
	"wdtt-plus/client/internal/firewall"
)

const adapterName = "WDTT Plus"

// A fixed adapter GUID keeps Windows from creating a new network profile on
// every connection.
var adapterGUID = windows.GUID{
	Data1: 0x7a1e2b3c,
	Data2: 0x4d5e,
	Data3: 0x4f60,
	Data4: [8]byte{0x8a, 0x9b, 0xac, 0xbd, 0xce, 0xdf, 0xe0, 0xf1},
}

func init() {
	tun.WintunTunnelType = adapterName
	tun.WintunStaticRequestedGUID = &adapterGUID
}

type Options struct {
	Config WireGuardConfig
	// Routes are the IPv4 destinations sent through the adapter.
	Routes []netip.Prefix
	// KillSwitch blocks all traffic that bypasses the adapter, except for this
	// process, and restricts DNS to the tunnel servers.
	KillSwitch bool
	Binder     *Binder
}

type Tunnel struct {
	// Warning describes a non-fatal setup problem shown to the user.
	Warning string

	device   *device.Device
	luid     winipcfg.LUID
	binder   *Binder
	firewall bool
}

func Start(opts Options) (_ *Tunnel, err error) {
	uapi, err := opts.Config.UAPI()
	if err != nil {
		return nil, err
	}
	if err := ensureWintun(); err != nil {
		return nil, err
	}
	tunDevice, err := tun.CreateTUN(adapterName, opts.Config.MTU)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать адаптер Wintun: %w", err)
	}
	native := tunDevice.(*tun.NativeTun)
	t := &Tunnel{luid: winipcfg.LUID(native.LUID()), binder: opts.Binder}
	if t.binder != nil {
		t.binder.setTunnel(t.luid)
	}

	logger := &device.Logger{
		Verbosef: device.DiscardLogf,
		Errorf: func(format string, args ...any) {
			log.Printf("[WIREGUARD] "+format, args...)
		},
	}
	t.device = device.NewDevice(tunDevice, conn.NewStdNetBind(), logger)
	defer func() {
		if err != nil {
			t.Close()
		}
	}()
	if err = t.device.IpcSet(uapi); err != nil {
		return nil, fmt.Errorf("настройка WireGuard: %w", err)
	}
	if err = t.device.Up(); err != nil {
		return nil, fmt.Errorf("запуск WireGuard: %w", err)
	}
	if err = t.configureInterface(opts); err != nil {
		return nil, err
	}
	if opts.KillSwitch {
		if fwErr := firewall.EnableFirewall(uint64(t.luid), false, opts.Config.DNS); fwErr != nil {
			t.Warning = fmt.Sprintf("защита от утечек не включилась: %v", fwErr)
			log.Printf("[VPN] %s", t.Warning)
		} else {
			t.firewall = true
		}
	}
	return t, nil
}

func (t *Tunnel) configureInterface(opts Options) error {
	var addresses []netip.Prefix
	for _, prefix := range opts.Config.Addresses {
		if prefix.Addr().Is4() {
			addresses = append(addresses, prefix)
		}
	}
	if len(addresses) == 0 {
		return errors.New("сервер не выдал IPv4-адрес туннеля")
	}
	if err := t.luid.SetIPAddressesForFamily(windows.AF_INET, addresses); err != nil {
		return fmt.Errorf("адрес адаптера: %w", err)
	}

	routes := make([]*winipcfg.RouteData, 0, len(opts.Routes))
	for _, prefix := range opts.Routes {
		routes = append(routes, &winipcfg.RouteData{
			Destination: prefix,
			NextHop:     netip.IPv4Unspecified(),
		})
	}
	if err := t.luid.SetRoutesForFamily(windows.AF_INET, routes); err != nil {
		return fmt.Errorf("маршруты: %w", err)
	}

	iface, err := t.luid.IPInterface(windows.AF_INET)
	if err != nil {
		return err
	}
	iface.UseAutomaticMetric = false
	iface.Metric = 0
	iface.NLMTU = uint32(opts.Config.MTU)
	if err := iface.Set(); err != nil {
		return fmt.Errorf("параметры адаптера: %w", err)
	}

	if err := t.luid.SetDNS(windows.AF_INET, opts.Config.DNS, nil); err != nil {
		return fmt.Errorf("DNS: %w", err)
	}
	return nil
}

func (t *Tunnel) Close() {
	if t.firewall {
		firewall.DisableFirewall()
		t.firewall = false
	}
	if t.device != nil {
		t.device.Close()
		t.device = nil
	}
	if t.binder != nil {
		t.binder.setTunnel(0)
	}
}

// IsElevated reports whether the process can create network adapters.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
