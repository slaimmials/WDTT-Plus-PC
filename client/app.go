package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"wdtt-plus/client/internal/admin"
	"wdtt-plus/client/internal/applog"
	"wdtt-plus/client/internal/captcha"
	"wdtt-plus/client/internal/connection"
	"wdtt-plus/client/internal/core"
	"wdtt-plus/client/internal/deploy"
	"wdtt-plus/client/internal/platform"
	"wdtt-plus/client/internal/profile"
	"wdtt-plus/client/internal/routing"
	"wdtt-plus/client/internal/sshx"
	"wdtt-plus/client/internal/tunnel"
)

// Events emitted to the frontend.
const (
	eventStatus   = "status"
	eventLog      = "log"
	eventLink     = "link"
	eventHash     = "hash-check"
	eventDeploy   = "deploy"
	eventSettings = "settings"
	eventPing     = "ping"
)

// App is the Wails-bound backend. Exported methods are callable from the UI.
type App struct {
	ctx     context.Context
	dataDir string
	logs    *applog.Log
	store   *profile.Store
	manager *connection.Manager

	mu       sync.Mutex
	settings profile.Settings

	pendingLink string
}

func NewApp(dataDir string, logs *applog.Log) *App {
	a := &App{dataDir: dataDir, logs: logs, store: profile.NewStore(dataDir)}
	a.manager = connection.NewManager(dataDir, version, a.solveCaptcha, func(status connection.Status) {
		a.emit(eventStatus, status)
	})
	settings, err := a.store.Load()
	if err != nil {
		log.Printf("[НАСТРОЙКИ] %v; используются настройки по умолчанию", err)
	}
	a.settings = settings
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.logs.OnEntry(func(entry applog.Entry) { a.emit(eventLog, entry) })
	if err := platform.RegisterURLScheme(); err != nil {
		log.Printf("[СИСТЕМА] Не удалось зарегистрировать ссылки wdtt://: %v", err)
	}
	a.mu.Lock()
	a.pendingLink = platform.LinkFromArgs(os.Args[1:])
	a.mu.Unlock()
}

func (a *App) shutdown(context.Context) {
	a.manager.Shutdown()
}

func (a *App) emit(name string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}

// onSecondInstance receives wdtt:// links opened while the app is running.
func (a *App) onSecondInstance(args []string) {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	if link := platform.LinkFromArgs(args); link != "" {
		a.emit(eventLink, link)
	}
}

func (a *App) solveCaptcha(ctx context.Context, redirectURI string) (string, error) {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
	}
	return captcha.Solve(ctx, redirectURI, filepath.Join(a.dataDir, "captcha"))
}

func (a *App) snapshot() profile.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, _ := json.Marshal(a.settings)
	var copy profile.Settings
	_ = json.Unmarshal(data, &copy)
	return copy
}

func (a *App) modify(change func(*profile.Settings) error) error {
	a.mu.Lock()
	if err := change(&a.settings); err != nil {
		a.mu.Unlock()
		return err
	}
	settings := a.settings
	err := a.store.Save(settings)
	a.mu.Unlock()
	if err != nil {
		return fmt.Errorf("не удалось сохранить настройки: %w", err)
	}
	a.emit(eventSettings, settings)
	return nil
}

type State struct {
	Version     string            `json:"version"`
	Elevated    bool              `json:"elevated"`
	Settings    profile.Settings  `json:"settings"`
	Status      connection.Status `json:"status"`
	PendingLink string            `json:"pendingLink"`
}

func (a *App) GetState() State {
	a.mu.Lock()
	link := a.pendingLink
	a.pendingLink = ""
	a.mu.Unlock()
	return State{
		Version:     version,
		Elevated:    tunnel.IsElevated(),
		Settings:    a.snapshot(),
		Status:      a.manager.Status(),
		PendingLink: link,
	}
}

func (a *App) RestartAsAdmin() error {
	if err := platform.RelaunchElevated(); err != nil {
		return fmt.Errorf("запуск от имени администратора отменён: %w", err)
	}
	a.manager.Shutdown()
	runtime.Quit(a.ctx)
	return nil
}

// Profiles

func (a *App) ParseLink(text string) (profile.Profile, error) {
	link, err := profile.ParseLink(text)
	if err != nil {
		return profile.Profile{}, err
	}
	return profile.FromLink(link), nil
}

func (a *App) SaveProfile(p profile.Profile) (profile.Profile, error) {
	p.Normalize()
	if err := p.Validate(); err != nil {
		return p, err
	}
	err := a.modify(func(s *profile.Settings) error {
		if existing, ok := s.Profile(p.ID); ok {
			*existing = p
		} else {
			s.Profiles = append(s.Profiles, p)
		}
		if s.ActiveProfile == "" {
			s.ActiveProfile = p.ID
		}
		return nil
	})
	return p, err
}

func (a *App) DeleteProfile(id string) error {
	if status := a.manager.Status(); status.ProfileID == id && status.State != connection.Disconnected {
		return errors.New("сначала отключитесь от этого профиля")
	}
	return a.modify(func(s *profile.Settings) error {
		s.Profiles = slices.DeleteFunc(s.Profiles, func(p profile.Profile) bool { return p.ID == id })
		if s.ActiveProfile == id {
			s.ActiveProfile = ""
			if len(s.Profiles) > 0 {
				s.ActiveProfile = s.Profiles[0].ID
			}
		}
		return nil
	})
}

func (a *App) SetActiveProfile(id string) error {
	return a.modify(func(s *profile.Settings) error {
		if _, ok := s.Profile(id); !ok {
			return errors.New("профиль не найден")
		}
		s.ActiveProfile = id
		return nil
	})
}

func (a *App) ProfileLink(id string) (string, error) {
	settings := a.snapshot()
	p, ok := settings.Profile(id)
	if !ok {
		return "", errors.New("профиль не найден")
	}
	return p.Link().String(), nil
}

func (a *App) NormalizeHashes(text string) ([]string, error) {
	return profile.NormalizeHashes(text)
}

// Connection

func (a *App) Connect(id string) error {
	settings := a.snapshot()
	p, ok := settings.Profile(id)
	if !ok {
		return errors.New("профиль не найден")
	}
	return a.manager.Connect(*p, settings)
}

func (a *App) Disconnect() {
	a.manager.Disconnect()
}

func (a *App) CheckHashes(hashes []string) error {
	settings := a.snapshot()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		err := a.manager.CheckHashes(ctx, hashes, settings, func(index int, result core.HashCheckResult) {
			a.emit(eventHash, map[string]any{"index": index, "result": result})
		})
		if err != nil {
			a.emit(eventHash, map[string]any{"error": err.Error()})
		}
		a.emit(eventHash, map[string]any{"done": true})
	}()
	return nil
}

// PingProfiles checks the given profiles one by one and reports each result
// with a "ping" event.
func (a *App) PingProfiles(ids []string) error {
	if a.manager.Status().State != connection.Disconnected {
		return errors.New("отключитесь, чтобы проверить профили")
	}
	settings := a.snapshot()
	go func() {
		for _, id := range ids {
			p, ok := settings.Profile(id)
			if !ok {
				continue
			}
			a.emit(eventPing, map[string]any{"profileId": id, "testing": true})
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			rtt, err := a.manager.Probe(ctx, *p, settings)
			cancel()
			result := map[string]any{"profileId": id}
			if err != nil {
				log.Printf("[ПИНГ] %s: %v", p.Name, err)
				result["error"] = err.Error()
			} else {
				log.Printf("[ПИНГ] %s: %d мс", p.Name, rtt.Milliseconds())
				result["latencyMs"] = max(1, rtt.Milliseconds())
			}
			a.emit(eventPing, result)
		}
	}()
	return nil
}

// Settings

func (a *App) SaveRouting(r profile.Routing) error {
	clean := func(rules []string) []string {
		out := make([]string, 0, len(rules))
		for _, rule := range rules {
			if rule = strings.TrimSpace(rule); rule != "" && !slices.Contains(out, rule) {
				out = append(out, rule)
			}
		}
		return out
	}
	r.Exclude = clean(r.Exclude)
	r.Include = clean(r.Include)
	return a.modify(func(s *profile.Settings) error {
		s.Routing = r
		return nil
	})
}

// ValidateRule returns an empty string for a valid routing rule.
func (a *App) ValidateRule(rule string) string {
	if err := routing.Validate(rule); err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) SaveVK(vk profile.VK) error {
	if err := core.ValidateCustomVKCredentials(vk.CustomClientID, vk.CustomSecret); err != nil {
		return err
	}
	return a.modify(func(s *profile.Settings) error {
		s.VK = vk
		return nil
	})
}

func (a *App) SaveAppearance(theme string, diagnostics bool) error {
	return a.modify(func(s *profile.Settings) error {
		s.Theme = theme
		s.Diagnostics = diagnostics
		return nil
	})
}

// Logs

func (a *App) GetLogs() []applog.Entry { return a.logs.Entries() }

func (a *App) ClearLogs() { a.logs.Clear() }

func (a *App) OpenDataFolder() {
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(a.dataDir))
}

// Servers

func (a *App) SaveServer(s profile.Server) (profile.Server, error) {
	s.Normalize()
	if !profile.ValidHost(s.Host) {
		return s, errors.New("неверный адрес сервера")
	}
	err := a.modify(func(settings *profile.Settings) error {
		if existing, ok := settings.Server(s.ID); ok {
			if existing.Host != s.Host || existing.SSHPort != s.SSHPort {
				s.HostKey = ""
			} else {
				s.HostKey = existing.HostKey
			}
			*existing = s
		} else {
			settings.Servers = append(settings.Servers, s)
		}
		return nil
	})
	return s, err
}

func (a *App) DeleteServer(id string) error {
	return a.modify(func(s *profile.Settings) error {
		s.Servers = slices.DeleteFunc(s.Servers, func(server profile.Server) bool { return server.ID == id })
		return nil
	})
}

func (a *App) withSSH(id string, timeout time.Duration, run func(context.Context, *sshx.Client, profile.Server) error) error {
	settings := a.snapshot()
	server, ok := settings.Server(id)
	if !ok {
		return errors.New("сервер не найден")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := sshx.Dial(ctx, sshx.Target{
		Host:          server.Host,
		Port:          server.SSHPort,
		User:          server.User,
		Password:      server.SSHPassword,
		PrivateKey:    server.PrivateKey,
		KeyPassphrase: server.KeyPassphrase,
		HostKey:       server.HostKey,
	})
	if err != nil {
		return err
	}
	defer client.Close()
	if server.HostKey == "" {
		key := client.HostKey()
		_ = a.modify(func(s *profile.Settings) error {
			if stored, ok := s.Server(id); ok {
				stored.HostKey = key
			}
			return nil
		})
	}
	return run(ctx, client, *server)
}

// AdminCall runs a wdtt-server admin command and returns its JSON response.
func (a *App) AdminCall(serverID string, args []string) (map[string]any, error) {
	var response map[string]any
	err := a.withSSH(serverID, 2*time.Minute, func(ctx context.Context, client *sshx.Client, server profile.Server) error {
		raw, err := admin.Call(ctx, client, server.MainPassword, args...)
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, &response)
	})
	return response, err
}

func (a *App) RestartServer(serverID string) error {
	return a.withSSH(serverID, time.Minute, func(ctx context.Context, client *sshx.Client, _ profile.Server) error {
		return admin.Restart(ctx, client)
	})
}

func (a *App) DeployServer(serverID string, mode string) error {
	go func() {
		err := a.withSSH(serverID, 20*time.Minute, func(ctx context.Context, client *sshx.Client, server profile.Server) error {
			result, err := deploy.Run(ctx, client, deploy.Options{
				Mode:         deploy.Mode(mode),
				MainPassword: server.MainPassword,
				DTLSPort:     server.DTLSPort,
				WGPort:       server.WGPort,
				LocalPort:    server.LocalPort,
				SSHPort:      server.SSHPort,
				DNS:          server.DNS,
				Progress: func(fraction float64, step string) {
					a.emit(eventDeploy, map[string]any{"serverId": serverID, "progress": fraction, "step": step})
				},
			})
			if err == nil {
				log.Printf("[ДЕПЛОЙ] wdtt-server %s установлен на %s", result.Version, server.Host)
			}
			return err
		})
		payload := map[string]any{"serverId": serverID, "done": true}
		if err != nil {
			log.Printf("[ДЕПЛОЙ] Ошибка: %v", err)
			payload["error"] = err.Error()
		}
		a.emit(eventDeploy, payload)
	}()
	return nil
}

func (a *App) UninstallServer(serverID string) error {
	return a.withSSH(serverID, 5*time.Minute, func(ctx context.Context, client *sshx.Client, _ profile.Server) error {
		return deploy.Uninstall(ctx, client)
	})
}

// ClientLink builds a wdtt:// link for a client of a managed server.
func (a *App) ClientLink(serverID, password, hash, name string) (string, error) {
	settings := a.snapshot()
	server, ok := settings.Server(serverID)
	if !ok {
		return "", errors.New("сервер не найден")
	}
	hashes, err := profile.NormalizeHashes(hash)
	if err != nil {
		return "", err
	}
	return profile.Link{
		Name:      name,
		Host:      server.Host,
		DTLSPort:  server.DTLSPort,
		WGPort:    server.WGPort,
		LocalPort: server.LocalPort,
		Password:  password,
		Hashes:    hashes,
	}.String(), nil
}

// SaveTextFile asks for a location and writes content there.
func (a *App) SaveTextFile(defaultName, content string) error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{DefaultFilename: defaultName})
	if err != nil || path == "" {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

// ReadTextFile asks for a file and returns its content.
func (a *App) ReadTextFile() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{})
	if err != nil || path == "" {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > 1<<20 {
		return "", errors.New("файл слишком большой")
	}
	data, err := os.ReadFile(path)
	return string(data), err
}
