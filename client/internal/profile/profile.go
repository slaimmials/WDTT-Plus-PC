// Package profile holds the user's connection profiles, managed servers and
// application settings, and persists them encrypted for the current user.
package profile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Mode string

const (
	ModeVPN    Mode = "vpn"
	ModeProxy  Mode = "auto"
	ModeSOCKS5 Mode = "socks5"
	ModeHTTP   Mode = "http"
)

type Proxy struct {
	Port int  `json:"port"`
	LAN  bool `json:"lan"`
	// Username and Password enable proxy authentication; LAN access requires it.
	Username string `json:"username"`
	Password string `json:"password"`
	UDP      bool   `json:"udp"`
}

type Profile struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Host       string   `json:"host"`
	DTLSPort   int      `json:"dtlsPort"`
	WGPort     int      `json:"wgPort"`
	LocalPort  int      `json:"localPort"`
	Password   string   `json:"password"`
	Hashes     []string `json:"hashes"`
	Workers    int      `json:"workers"`
	Mode       Mode     `json:"mode"`
	Proxy      Proxy    `json:"proxy"`
	StreamOnly bool     `json:"streamOnly"`
	TURNSNI    string   `json:"turnSni"`
	Masque     bool     `json:"masque"`
}

func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func FromLink(link Link) Profile {
	p := Profile{
		ID:        NewID(),
		Name:      link.Name,
		Host:      link.Host,
		DTLSPort:  link.DTLSPort,
		WGPort:    link.WGPort,
		LocalPort: link.LocalPort,
		Password:  link.Password,
		Hashes:    link.Hashes,
		Workers:   link.MaxWorkers,
	}
	p.Normalize()
	return p
}

func (p Profile) Link() Link {
	return Link{
		Name:      p.Name,
		Host:      p.Host,
		DTLSPort:  p.DTLSPort,
		WGPort:    p.WGPort,
		LocalPort: p.LocalPort,
		Password:  p.Password,
		Hashes:    p.Hashes,
	}
}

func (p *Profile) Normalize() {
	if p.ID == "" {
		p.ID = NewID()
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = p.Host
	}
	if p.DTLSPort == 0 {
		p.DTLSPort = DefaultDTLSPort
	}
	if p.WGPort == 0 {
		p.WGPort = DefaultWGPort
	}
	if p.LocalPort == 0 {
		p.LocalPort = DefaultLocalPort
	}
	if p.Workers == 0 {
		p.Workers = 24
	}
	if p.Mode == "" {
		p.Mode = ModeVPN
	}
	if p.Proxy.Port == 0 {
		p.Proxy.Port = 1080
	}
	if p.Hashes == nil {
		p.Hashes = []string{}
	}
}

func (p Profile) Validate() error {
	if !ValidHost(p.Host) {
		return errors.New("неверный адрес сервера")
	}
	if p.Password == "" {
		return errors.New("не указан пароль подключения")
	}
	if len(p.Hashes) == 0 {
		return errors.New("добавьте хотя бы один VK-хеш")
	}
	switch p.Mode {
	case ModeVPN, ModeProxy, ModeSOCKS5, ModeHTTP:
	default:
		return fmt.Errorf("неизвестный режим %q", p.Mode)
	}
	if p.Mode != ModeVPN {
		if p.Proxy.Port < 1 || p.Proxy.Port > 65535 {
			return errors.New("некорректный порт прокси")
		}
		if p.Proxy.LAN && (p.Proxy.Username == "" || p.Proxy.Password == "") {
			return errors.New("для доступа к прокси из локальной сети задайте логин и пароль")
		}
	}
	return nil
}

func (p Profile) PeerAddress() string {
	return net.JoinHostPort(p.Host, strconv.Itoa(p.DTLSPort))
}

// RoutingMode selects which destinations go through the VPN.
type RoutingMode string

const (
	RoutingAll     RoutingMode = "all"
	RoutingExclude RoutingMode = "exclude"
	RoutingInclude RoutingMode = "include"
)

type Routing struct {
	Mode RoutingMode `json:"mode"`
	// Exclude and Include hold IPv4 addresses, CIDR subnets, ranges and domains.
	Exclude   []string `json:"exclude"`
	Include   []string `json:"include"`
	BypassLAN bool     `json:"bypassLan"`
}

func (r Routing) ActiveRules() []string {
	switch r.Mode {
	case RoutingExclude:
		return r.Exclude
	case RoutingInclude:
		return r.Include
	default:
		return nil
	}
}

// VK holds advanced settings of the VK credential provider.
type VK struct {
	Fingerprint      string `json:"fingerprint"`
	ClientIDs        string `json:"clientIds"`
	CustomClientID   string `json:"customClientId"`
	CustomSecret     string `json:"customSecret"`
	VKCallsPreflight bool   `json:"vkCallsPreflight"`
	HashFallback     bool   `json:"hashFallback"`
}

// Server is a VPS managed over SSH.
type Server struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	SSHPort       int    `json:"sshPort"`
	User          string `json:"user"`
	SSHPassword   string `json:"sshPassword"`
	PrivateKey    string `json:"privateKey"`
	KeyPassphrase string `json:"keyPassphrase"`
	HostKey       string `json:"hostKey"`
	MainPassword  string `json:"mainPassword"`
	DTLSPort      int    `json:"dtlsPort"`
	WGPort        int    `json:"wgPort"`
	LocalPort     int    `json:"localPort"`
	DNS           string `json:"dns"`
}

func (s *Server) Normalize() {
	if s.ID == "" {
		s.ID = NewID()
	}
	s.Host = strings.TrimSpace(s.Host)
	if s.Name == "" {
		s.Name = s.Host
	}
	if s.SSHPort == 0 {
		s.SSHPort = 22
	}
	if s.User == "" {
		s.User = "root"
	}
	if s.DTLSPort == 0 {
		s.DTLSPort = DefaultDTLSPort
	}
	if s.WGPort == 0 {
		s.WGPort = DefaultWGPort
	}
	if s.LocalPort == 0 {
		s.LocalPort = DefaultLocalPort
	}
	if s.DNS == "" {
		s.DNS = "1.1.1.1"
	}
}

type Settings struct {
	Profiles      []Profile `json:"profiles"`
	ActiveProfile string    `json:"activeProfile"`
	Servers       []Server  `json:"servers"`
	Routing       Routing   `json:"routing"`
	VK            VK        `json:"vk"`
	DeviceID      string    `json:"deviceId"`
	Theme         string    `json:"theme"`
	Diagnostics   bool      `json:"diagnostics"`
}

func DefaultSettings() Settings {
	s := Settings{
		Routing:  Routing{Mode: RoutingAll},
		VK:       VK{Fingerprint: "firefox", VKCallsPreflight: true},
		DeviceID: "pc-" + NewID(),
		Theme:    "system",
	}
	s.Normalize()
	return s
}

// Normalize fills defaults and replaces nil lists with empty ones, so the UI
// always receives arrays.
func (s *Settings) Normalize() {
	if s.Profiles == nil {
		s.Profiles = []Profile{}
	}
	for i := range s.Profiles {
		s.Profiles[i].Normalize()
	}
	if s.Servers == nil {
		s.Servers = []Server{}
	}
	for i := range s.Servers {
		s.Servers[i].Normalize()
	}
	if s.Routing.Mode == "" {
		s.Routing.Mode = RoutingAll
	}
	if s.Routing.Exclude == nil {
		s.Routing.Exclude = []string{}
	}
	if s.Routing.Include == nil {
		s.Routing.Include = []string{}
	}
	if s.DeviceID == "" {
		s.DeviceID = "pc-" + NewID()
	}
	if s.Theme == "" {
		s.Theme = "system"
	}
}

func (s *Settings) Profile(id string) (*Profile, bool) {
	for i := range s.Profiles {
		if s.Profiles[i].ID == id {
			return &s.Profiles[i], true
		}
	}
	return nil, false
}

func (s *Settings) Server(id string) (*Server, bool) {
	for i := range s.Servers {
		if s.Servers[i].ID == id {
			return &s.Servers[i], true
		}
	}
	return nil, false
}
