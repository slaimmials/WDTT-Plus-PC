// Package connection runs one WDTT connection at a time: the transport core
// plus either the system VPN adapter or a local proxy.
package connection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"wdtt-plus/client/internal/core"
	"wdtt-plus/client/internal/profile"
	"wdtt-plus/client/internal/routing"
	"wdtt-plus/client/internal/tunnel"
)

type State string

const (
	Disconnected  State = "disconnected"
	Connecting    State = "connecting"
	Connected     State = "connected"
	Disconnecting State = "disconnecting"
)

type Status struct {
	State          State      `json:"state"`
	ProfileID      string     `json:"profileId"`
	Mode           string     `json:"mode"`
	Error          string     `json:"error"`
	ProxyAddresses []string   `json:"proxyAddresses"`
	Stats          core.Stats `json:"stats"`
	ConnectedAt    int64      `json:"connectedAt"`
}

var ErrNeedsElevation = errors.New("для режима VPN нужны права администратора")

type CaptchaSolver func(ctx context.Context, redirectURI string) (string, error)

type Manager struct {
	dataDir    string
	appVersion string
	onStatus   func(Status)
	captcha    CaptchaSolver

	mu      sync.Mutex
	status  Status
	session *core.Session
	tunnel  *tunnel.Tunnel
	binder  *tunnel.Binder
}

func NewManager(dataDir, appVersion string, captcha CaptchaSolver, onStatus func(Status)) *Manager {
	return &Manager{
		dataDir:    dataDir,
		appVersion: appVersion,
		captcha:    captcha,
		onStatus:   onStatus,
		status:     Status{State: Disconnected},
	}
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) update(change func(*Status)) {
	m.mu.Lock()
	change(&m.status)
	status := m.status
	m.mu.Unlock()
	m.emit(status)
}

func (m *Manager) emit(status Status) {
	if m.onStatus != nil {
		m.onStatus(status)
	}
}

func (m *Manager) Connect(p profile.Profile, settings profile.Settings) error {
	if err := p.Validate(); err != nil {
		return err
	}
	vpn := p.Mode == profile.ModeVPN
	if vpn && !tunnel.IsElevated() {
		return ErrNeedsElevation
	}

	m.mu.Lock()
	if m.status.State != Disconnected {
		m.mu.Unlock()
		return errors.New("подключение уже активно")
	}
	m.status = Status{State: Connecting, ProfileID: p.ID, Mode: string(p.Mode)}
	status := m.status
	m.mu.Unlock()
	m.emit(status)

	cfg, err := m.coreConfig(p, settings)
	if err != nil {
		m.fail(err)
		return err
	}
	if vpn {
		binder, err := m.ensureBinder()
		if err != nil {
			m.fail(err)
			return err
		}
		cfg.SocketControl = binder.Control
	}

	hooks := core.Hooks{
		OnStats: func(stats core.Stats) {
			m.update(func(s *Status) { s.Stats = stats })
		},
		OnProxyReady: func(addresses []string) {
			m.update(func(s *Status) {
				s.State = Connected
				s.ProxyAddresses = addresses
				s.ConnectedAt = time.Now().UnixMilli()
			})
		},
		SolveCaptcha: func(ctx context.Context, request core.CaptchaRequest) (string, error) {
			log.Printf("[КАПЧА] VK просит подтвердить, что вы не робот")
			return m.captcha(ctx, request.RedirectURI)
		},
	}
	if vpn {
		hooks.OnConfig = func(config string) {
			if err := m.startTunnel(config, settings.Routing); err != nil {
				log.Printf("[VPN] %v", err)
				m.update(func(s *Status) { s.Error = err.Error() })
				m.mu.Lock()
				session := m.session
				m.mu.Unlock()
				if session != nil {
					session.Stop()
				}
			}
		}
	}

	session, err := core.Start(context.Background(), cfg, hooks)
	if err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	m.session = session
	m.mu.Unlock()
	go m.wait(session)
	return nil
}

func (m *Manager) startTunnel(config string, rules profile.Routing) error {
	wg, err := tunnel.ParseWireGuardConfig(config)
	if err != nil {
		return err
	}
	plan := routing.Plan{
		Mode:      routing.Mode(rules.Mode),
		Rules:     rules.ActiveRules(),
		BypassLAN: rules.BypassLAN,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	routes, warnings := plan.Build(ctx, routing.SystemResolver)
	for _, warning := range warnings {
		log.Printf("[МАРШРУТЫ] Правило пропущено: %v", warning)
	}
	if len(routes) == 0 {
		return errors.New("нет ни одного адреса для маршрутизации через VPN")
	}
	t, err := tunnel.Start(tunnel.Options{
		Config:     wg,
		Routes:     routes,
		KillSwitch: rules.Mode == profile.RoutingAll && !rules.BypassLAN,
		Binder:     m.binder,
	})
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.tunnel = t
	m.mu.Unlock()
	log.Printf("[VPN] Адаптер поднят, маршрутов: %d", len(routes))
	m.update(func(s *Status) {
		s.State = Connected
		s.ConnectedAt = time.Now().UnixMilli()
	})
	return nil
}

func (m *Manager) wait(session *core.Session) {
	err := session.Wait()
	m.mu.Lock()
	t := m.tunnel
	m.tunnel = nil
	m.session = nil
	m.mu.Unlock()
	if t != nil {
		t.Close()
	}
	if err != nil {
		log.Printf("[КЛИЕНТ] Подключение остановлено: %v", err)
	} else {
		log.Printf("[КЛИЕНТ] Подключение остановлено")
	}
	m.update(func(s *Status) {
		previous := s.Error
		*s = Status{State: Disconnected, Error: previous}
		if err != nil {
			s.Error = err.Error()
		}
	})
}

func (m *Manager) fail(err error) {
	m.update(func(s *Status) { *s = Status{State: Disconnected, Error: err.Error()} })
}

func (m *Manager) Disconnect() {
	m.mu.Lock()
	session := m.session
	if session == nil {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	m.update(func(s *Status) { s.State = Disconnecting })
	session.Stop()
}

// Shutdown stops the connection and waits until the adapter is removed.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	session := m.session
	m.mu.Unlock()
	if session != nil {
		session.Stop()
		select {
		case <-session.Done():
		case <-time.After(5 * time.Second):
		}
	}
	m.mu.Lock()
	if m.tunnel != nil {
		m.tunnel.Close()
		m.tunnel = nil
	}
	if m.binder != nil {
		m.binder.Close()
		m.binder = nil
	}
	m.mu.Unlock()
}

// Suspend and Resume forward computer sleep events to the transport.
func (m *Manager) Suspend() {
	if session := m.currentSession(); session != nil {
		session.Suspend()
	}
}

func (m *Manager) Resume() {
	if session := m.currentSession(); session != nil {
		session.Resume()
	}
}

func (m *Manager) currentSession() *core.Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session
}

func (m *Manager) ensureBinder() (*tunnel.Binder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.binder != nil {
		return m.binder, nil
	}
	binder, err := tunnel.NewBinder(func() {
		if session := m.currentSession(); session != nil {
			log.Printf("[СЕТЬ] Сменился основной сетевой интерфейс, переподключаю каналы")
			session.NetworkChanged()
		}
	})
	if err != nil {
		return nil, fmt.Errorf("отслеживание сети: %w", err)
	}
	m.binder = binder
	return binder, nil
}

// Probe measures the round trip through VK TURN to the profile's server.
// It cannot run while a connection is active.
func (m *Manager) Probe(ctx context.Context, p profile.Profile, settings profile.Settings) (time.Duration, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if m.Status().State != Disconnected {
		return 0, errors.New("отключитесь, чтобы проверить другой профиль")
	}
	cfg, err := m.coreConfig(p, settings)
	if err != nil {
		return 0, err
	}
	cfg.Proxy = core.ProxyConfig{}
	return core.Probe(ctx, cfg, core.Hooks{
		SolveCaptcha: func(ctx context.Context, request core.CaptchaRequest) (string, error) {
			return m.captcha(ctx, request.RedirectURI)
		},
	})
}

// CheckHashes reports for every hash whether VK issues TURN credentials.
func (m *Manager) CheckHashes(ctx context.Context, hashes []string, settings profile.Settings, report func(int, core.HashCheckResult)) error {
	cfg := core.Config{Hashes: hashes}
	applyVK(&cfg, settings.VK)
	hooks := core.Hooks{
		SolveCaptcha: func(ctx context.Context, request core.CaptchaRequest) (string, error) {
			return m.captcha(ctx, request.RedirectURI)
		},
	}
	return core.CheckHashes(ctx, cfg, hooks, report)
}

func (m *Manager) coreConfig(p profile.Profile, settings profile.Settings) (core.Config, error) {
	cfg := core.Config{
		Mode:             core.Mode(p.Mode),
		Peer:             p.PeerAddress(),
		Password:         p.Password,
		Hashes:           p.Hashes,
		Workers:          p.Workers,
		ListenAddr:       net.JoinHostPort("127.0.0.1", strconv.Itoa(p.LocalPort)),
		DeviceID:         settings.DeviceID,
		DeviceInfo:       m.deviceInfo(),
		StreamOnly:       p.StreamOnly,
		TURNSNI:          p.TURNSNI,
		Masque:           p.Masque,
		MasqueConfigPath: filepath.Join(m.dataDir, "warp-masque.json"),
		MasqueAcceptTOS:  p.Masque,
		Diagnostics:      settings.Diagnostics,
	}
	applyVK(&cfg, settings.VK)
	if p.Mode != profile.ModeVPN {
		cfg.Proxy = core.ProxyConfig{
			Listen:   net.JoinHostPort("127.0.0.1", strconv.Itoa(p.Proxy.Port)),
			Username: p.Proxy.Username,
			Password: p.Proxy.Password,
			UDP:      p.Proxy.UDP,
		}
		if p.Proxy.LAN {
			address, subnet, err := lanAddress()
			if err != nil {
				return cfg, err
			}
			cfg.Proxy.ExtraListen = net.JoinHostPort(address.String(), strconv.Itoa(p.Proxy.Port))
			cfg.Proxy.AllowCIDR = subnet.String()
		}
	}
	return cfg, nil
}

func applyVK(cfg *core.Config, vk profile.VK) {
	cfg.Fingerprint = vk.Fingerprint
	cfg.ClientIDs = vk.ClientIDs
	cfg.CustomVKClientID = vk.CustomClientID
	cfg.CustomVKClientSecret = vk.CustomSecret
	cfg.VKCallsPreflight = vk.VKCallsPreflight
	cfg.HashFallback = vk.HashFallback
}

func (m *Manager) deviceInfo() string {
	host, _ := os.Hostname()
	info, _ := json.Marshal(map[string]any{
		"name":            host,
		"manufacturer":    "PC",
		"brand":           runtime.GOOS,
		"model":           host,
		"android_version": osVersion(),
		"abi":             runtime.GOARCH,
		"app_version":     m.appVersion,
		"time_zone":       time.Now().Location().String(),
	})
	return string(info)
}

// lanAddress returns the private IPv4 address and subnet of the interface that
// carries the default route.
func lanAddress() (netip.Addr, netip.Prefix, error) {
	conn, err := net.Dial("udp4", "192.0.2.1:9")
	if err != nil {
		return netip.Addr{}, netip.Prefix{}, fmt.Errorf("не удалось определить адрес в локальной сети: %w", err)
	}
	local := conn.LocalAddr().(*net.UDPAddr).IP
	_ = conn.Close()

	ifaces, err := net.Interfaces()
	if err != nil {
		return netip.Addr{}, netip.Prefix{}, err
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || !ipNet.IP.Equal(local) {
				continue
			}
			ip, _ := netip.AddrFromSlice(ipNet.IP.To4())
			if !ip.IsPrivate() {
				return netip.Addr{}, netip.Prefix{}, errors.New("компьютер не находится в частной локальной сети")
			}
			ones, _ := ipNet.Mask.Size()
			return ip, netip.PrefixFrom(ip, ones).Masked(), nil
		}
	}
	return netip.Addr{}, netip.Prefix{}, errors.New("не найден адрес в локальной сети")
}
