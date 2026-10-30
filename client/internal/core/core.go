// Package core implements the WDTT transport: it obtains VK TURN credentials,
// keeps a pool of DTLS sessions to the WDTT server through VK TURN relays and
// exposes the result either as a local UDP endpoint for WireGuard or as a
// local SOCKS5/HTTP proxy backed by userspace WireGuard.
package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Mode string

const (
	// ModeVPN exposes a local UDP endpoint and reports the WireGuard config
	// through Hooks.OnConfig; the caller runs WireGuard itself.
	ModeVPN Mode = "vpn"
	// ModeProxy serves SOCKS5 and HTTP CONNECT on the same port.
	ModeProxy  Mode = "auto"
	ModeSOCKS5 Mode = "socks5"
	ModeHTTP   Mode = "http"
)

const (
	DefaultWorkers    = 24
	MaxWorkers        = 108
	WorkersPerGroup   = workersPerGroup
	defaultListenAddr = "127.0.0.1:9000"
)

type ProxyConfig struct {
	Listen      string
	ExtraListen string
	AllowCIDR   string
	Username    string
	Password    string
	UDP         bool
}

type Config struct {
	Mode     Mode
	Peer     string
	Password string
	Hashes   []string
	Workers  int

	// ListenAddr is the local UDP address WireGuard connects to in ModeVPN.
	ListenAddr string

	DeviceID   string
	DeviceInfo string

	Fingerprint          string
	ClientIDs            string
	CustomVKClientID     string
	CustomVKClientSecret string
	VKCallsPreflight     bool
	HashFallback         bool

	// StreamOnly restricts TURN to TCP/TLS and MASQUE to HTTP/2.
	StreamOnly bool
	TURNSNI    string
	TURNHost   string
	TURNPort   string

	Masque           bool
	MasqueConfigPath string
	MasqueAcceptTOS  bool

	Proxy ProxyConfig

	// Diagnostics logs transport queue and ordering metrics.
	Diagnostics bool

	// SocketControl is applied to every outbound socket the transport opens.
	// The Windows tunnel uses it to pin sockets to the physical interface so
	// that transport traffic never loops back into the VPN adapter.
	SocketControl func(network, address string, conn syscall.RawConn) error
}

type CaptchaKind string

const (
	CaptchaManual CaptchaKind = "manual"
)

type CaptchaRequest struct {
	ID           string
	Kind         CaptchaKind
	RedirectURI  string
	SessionToken string
}

type Stats struct {
	ActiveConnections int32
	BytesUp           int64
	BytesDown         int64
}

type Hooks struct {
	// OnConfig receives the WireGuard client config issued by the server.
	OnConfig func(config string)
	// OnProxyReady reports the addresses the local proxy listens on.
	OnProxyReady func(addresses []string)
	OnStats      func(Stats)
	// SolveCaptcha shows the VK captcha to the user and returns success_token.
	SolveCaptcha func(ctx context.Context, request CaptchaRequest) (string, error)
}

var (
	runMu        sync.Mutex
	activeHooks  atomic.Pointer[Hooks]
	socketCtrl   atomic.Pointer[func(network, address string, conn syscall.RawConn) error]
	resolverOnce sync.Once
)

// Session is a running transport.
type Session struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
	policy *connectionPolicy
	disp   atomic.Pointer[Dispatcher]
}

func (s *Session) Stop() { s.cancel() }

func (s *Session) Done() <-chan struct{} { return s.done }

// Wait blocks until the session ends and returns the reason it stopped.
func (s *Session) Wait() error {
	<-s.done
	return s.err
}

// NetworkChanged tells the transport that the host network changed, so stale
// paths are dropped and reconnection starts immediately.
func (s *Session) NetworkChanged() {
	s.policy.command("CONNECTION_NETWORK")
}

// Suspend pauses health timeouts while the computer sleeps.
func (s *Session) Suspend() {
	s.policy.command("DEVICE_SLEEP")
	if d := s.disp.Load(); d != nil {
		d.noteDeviceSleep()
	}
}

func (s *Session) Resume() {
	s.policy.command("DEVICE_WAKE")
	if d := s.disp.Load(); d != nil {
		d.noteDeviceWake(time.Now())
	}
}

func (cfg *Config) normalize() error {
	switch cfg.Mode {
	case "":
		cfg.Mode = ModeVPN
	case ModeVPN, ModeProxy, ModeSOCKS5, ModeHTTP:
	default:
		return fmt.Errorf("неизвестный режим %q", cfg.Mode)
	}
	cfg.Peer = strings.TrimSpace(cfg.Peer)
	if cfg.Peer == "" {
		return errors.New("не указан адрес сервера")
	}
	if cfg.Password == "" {
		return errors.New("не указан пароль подключения")
	}
	if len(cfg.Hashes) == 0 {
		return errors.New("не указан ни один VK-хеш")
	}
	if cfg.Workers <= 0 {
		cfg.Workers = DefaultWorkers
	}
	cfg.Workers = min(max(cfg.Workers, workersPerGroup), MaxWorkers) / workersPerGroup * workersPerGroup
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = "unknown"
	}
	if cfg.Mode != ModeVPN {
		auth := cfg.Proxy.Username != "" || cfg.Proxy.Password != ""
		if auth && (cfg.Proxy.Username == "" || cfg.Proxy.Password == "") {
			return errors.New("для прокси нужны и логин, и пароль")
		}
		if _, _, err := validateProxyListenAddresses(cfg.proxyAddresses(), cfg.Proxy.AllowCIDR, auth); err != nil {
			return err
		}
	}
	sni, err := normalizeTURNFrontSNI(cfg.TURNSNI)
	if err != nil {
		return fmt.Errorf("некорректный TURN SNI: %w", err)
	}
	cfg.TURNSNI = sni
	return nil
}

func (cfg *Config) proxyAddresses() []string {
	addresses := []string{cfg.Proxy.Listen}
	if strings.TrimSpace(cfg.Proxy.ExtraListen) != "" {
		addresses = append(addresses, cfg.Proxy.ExtraListen)
	}
	return addresses
}

func (cfg *Config) applyGlobals() error {
	if cfg.Fingerprint != "" {
		SetActiveFingerprint(cfg.Fingerprint)
	}
	if cfg.ClientIDs != "" {
		SetActiveClientIds(cfg.ClientIDs)
	}
	if err := SetCustomVKCredentials(cfg.CustomVKClientID, cfg.CustomVKClientSecret); err != nil {
		return fmt.Errorf("некорректные реквизиты VK: %w", err)
	}
	vkCallsPreflightEnabled.Store(cfg.VKCallsPreflight)
	transportMetricsEnabled = cfg.Diagnostics
	if cfg.SocketControl != nil {
		socketCtrl.Store(&cfg.SocketControl)
	} else {
		socketCtrl.Store(nil)
	}
	resolverOnce.Do(setupGlobalResolver)
	return nil
}

// Start validates cfg and launches the transport. Only one session may run at
// a time because VK client settings are process-wide.
func Start(parent context.Context, cfg Config, hooks Hooks) (*Session, error) {
	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	if !runMu.TryLock() {
		return nil, errors.New("транспорт уже запущен")
	}
	if err := cfg.applyGlobals(); err != nil {
		runMu.Unlock()
		return nil, err
	}
	activeHooks.Store(&hooks)

	ctx, cancel := context.WithCancel(parent)
	session := &Session{
		cancel: cancel,
		done:   make(chan struct{}),
		policy: newConnectionPolicy("", false),
	}
	go func() {
		defer runMu.Unlock()
		defer activeHooks.Store(nil)
		defer close(session.done)
		defer cancel()
		session.err = session.run(ctx, cfg, hooks)
	}()
	return session, nil
}

func (s *Session) run(ctx context.Context, cfg Config, hooks Hooks) error {
	peerEndpoint, err := parsePeerEndpoint(cfg.Peer)
	if err != nil {
		return fmt.Errorf("адрес сервера: %w", err)
	}
	peer, err := waitForPeerUDPAddr(ctx, peerEndpoint)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("не удалось определить адрес сервера: %w", err)
	}
	wrapKey, err := deriveWrapKey(cfg.Password)
	if err != nil {
		return fmt.Errorf("ключ WRAP: %w", err)
	}

	var masque *warpMasqueManager
	if cfg.Masque {
		masque, err = newWarpMasqueManager(ctx, cfg.MasqueConfigPath, cfg.TURNSNI, cfg.MasqueAcceptTOS)
		if err != nil {
			log.Printf("[MASQUE] Резерв недоступен: %v", err)
			masque = nil
		} else {
			defer masque.Close()
			masque.singleEnrollment = true
		}
	}

	s.policy.configure("", cfg.StreamOnly)
	tp := &TurnParams{
		Experiment:  experimentAuto,
		Host:        cfg.TURNHost,
		Port:        cfg.TURNPort,
		Hashes:      cfg.Hashes,
		TLSFrontSNI: cfg.TURNSNI,
		Masque:      masque,
		WrapKey:     wrapKey,
		NoUDP:       cfg.StreamOnly,
		Auto:        s.policy,
	}

	localConn, err := listenLocalUDP(cfg.ListenAddr)
	if err != nil {
		return err
	}
	stopLocal := context.AfterFunc(ctx, func() { _ = localConn.Close() })
	defer stopLocal()
	_, localPort, _ := net.SplitHostPort(localConn.LocalAddr().String())

	groups := cfg.Workers / workersPerGroup
	log.Printf("[КЛИЕНТ] Сервер %s, хешей %d, потоков %d (групп %d)", cfg.Peer, len(cfg.Hashes), cfg.Workers, groups)
	log.Printf("[КЛИЕНТ] TLS-отпечаток %s, VK Client ID: %s", GetActiveFingerprint(), GetActiveClientIdsString())
	if cfg.StreamOnly {
		log.Printf("[ПОДКЛЮЧЕНИЕ] Только TURN по TCP/TLS")
	}

	stats := newTransferStats()
	go stats.run(ctx, hooks.OnStats)

	disp := NewDispatcher(ctx, localConn, stats, experimentAuto)
	disp.setUnansweredUserTrafficHealthEnabled(cfg.Mode == ModeVPN)
	s.disp.Store(disp)
	defer disp.Shutdown()

	configCh := make(chan string, 1)
	runErr := make(chan error, 1)
	configDone := make(chan struct{})
	go func() {
		defer close(configDone)
		select {
		case raw, ok := <-configCh:
			if !ok || raw == "" {
				return
			}
			if err := s.deliverConfig(ctx, cfg, hooks, withDefaultMTU(raw)); err != nil {
				runErr <- err
				s.cancel()
			}
		case <-ctx.Done():
		}
	}()

	transportSession := newTransportSession()
	starts := newStartPacer(workerStartInterval(len(cfg.Hashes), cfg.StreamOnly))
	credentialRequests := newCredentialRequestGate(credentialRequestCooldown)
	configGate := newConfigFirstStartGate(true)
	primaryReady := make(chan struct{})
	var pauseFlag int32

	var wg sync.WaitGroup
	workerID := 1
	for g := range groups {
		first := g == 0
		ids := make([]int, workersPerGroup)
		for i := range ids {
			ids[i] = workerID
			workerID++
		}
		var (
			cc            chan<- string
			waitPrimary   <-chan struct{}
			signalPrimary chan<- struct{}
		)
		if first {
			cc = configCh
			signalPrimary = primaryReady
		} else {
			waitPrimary = primaryReady
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			WorkerGroup(ctx, s.cancel, g+1, g, tp, peer, disp, localPort,
				first, cc, ids, cfg.Workers, cfg.HashFallback, &pauseFlag,
				cfg.DeviceID, cfg.Password, cfg.DeviceInfo, transportSession, stats, cfg.StreamOnly,
				configGate, starts, credentialRequests, waitPrimary, signalPrimary)
		}()
	}
	wg.Wait()
	close(configCh)
	<-configDone

	select {
	case err := <-runErr:
		return err
	default:
	}
	if ctx.Err() == nil || errors.Is(context.Cause(ctx), context.Canceled) {
		return nil
	}
	return context.Cause(ctx)
}

func (s *Session) deliverConfig(ctx context.Context, cfg Config, hooks Hooks, wgConfig string) error {
	if cfg.Mode == ModeVPN {
		if hooks.OnConfig != nil {
			hooks.OnConfig(wgConfig)
		}
		return nil
	}

	wgDevice, tunnelNet, err := startSocksWireGuard(wgConfig)
	if err != nil {
		return fmt.Errorf("userspace WireGuard: %w", err)
	}
	defer wgDevice.Close()

	auth := cfg.Proxy.Username != ""
	udp := cfg.Proxy.UDP || cfg.Mode == ModeProxy
	err = runUnifiedProxyServer(ctx, string(cfg.Mode), cfg.proxyAddresses(),
		netstackSocksDialer{network: tunnelNet}, auth, cfg.Proxy.Username, cfg.Proxy.Password,
		udp, cfg.Proxy.AllowCIDR,
		func(addresses []string) {
			log.Printf("[ПРОКСИ] Готов: %s (UDP=%t, авторизация=%t)", strings.Join(addresses, ", "), udp, auth)
			if hooks.OnProxyReady != nil {
				hooks.OnProxyReady(addresses)
			}
		})
	if err != nil && ctx.Err() == nil {
		if address, inUse := proxyPortInUseAddress(err); inUse {
			return fmt.Errorf("порт прокси %s уже занят", address)
		}
		return fmt.Errorf("прокси: %w", err)
	}
	return nil
}

func withDefaultMTU(config string) string {
	if strings.Contains(config, "MTU =") {
		return config
	}
	lines := strings.Split(config, "\n")
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		out = append(out, line)
		if strings.TrimSpace(line) == "[Interface]" {
			out = append(out, "MTU = 1280")
		}
	}
	return strings.Join(out, "\n")
}

// listenLocalUDP retries briefly because a previous session may still be
// releasing the port.
func listenLocalUDP(address string) (net.PacketConn, error) {
	var lastErr error
	for range 5 {
		conn, err := net.ListenPacket("udp", address)
		if err == nil {
			tuneUDPBuffers(conn)
			return conn, nil
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	log.Printf("[КЛИЕНТ] Порт %s занят (%v), выбираю свободный", address, lastErr)
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("локальный UDP-порт: %w", err)
	}
	tuneUDPBuffers(conn)
	return conn, nil
}

func tuneUDPBuffers(conn net.PacketConn) {
	if uc, ok := conn.(*net.UDPConn); ok {
		_ = uc.SetReadBuffer(socketBufSize)
		_ = uc.SetWriteBuffer(socketBufSize)
	}
}

func newTransportSession() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value[:])
}
