package connection

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// InternetCheck is the result of a request made through the tunnel right
// after connecting.
type InternetCheck struct {
	State   string `json:"state"` // "", "checking", "ok", "failed"
	IP      string `json:"ip"`
	Country string `json:"country"`
	Message string `json:"message"`
}

const (
	traceByIP     = "https://1.1.1.1/cdn-cgi/trace"
	traceByDomain = "https://www.cloudflare.com/cdn-cgi/trace"
)

// checkInternet fetches Cloudflare's trace page through the tunnel: first by
// IP to test the path itself, then by domain name to test DNS.
func (m *Manager) checkInternet(client *http.Client) {
	m.update(func(s *Status) { s.Internet = InternetCheck{State: "checking"} })

	var ip, country string
	var err error
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(3 * time.Second)
		}
		if m.Status().State != Connected {
			return
		}
		ip, country, err = fetchTrace(client, traceByIP)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Printf("[ПРОВЕРКА] Через туннель нет доступа в интернет: %v", err)
		m.update(func(s *Status) {
			s.Internet = InternetCheck{State: "failed", Message: "нет доступа в интернет через сервер: " + shortError(err)}
		})
		return
	}
	check := InternetCheck{State: "ok", IP: ip, Country: country}
	if _, _, dnsErr := fetchTrace(client, traceByDomain); dnsErr != nil {
		log.Printf("[ПРОВЕРКА] Интернет через туннель есть (IP %s), но DNS не работает: %v", ip, dnsErr)
		check.State = "failed"
		check.Message = "интернет есть, но DNS не отвечает: " + shortError(dnsErr)
	} else {
		log.Printf("[ПРОВЕРКА] Интернет через туннель работает, внешний IP %s (%s)", ip, country)
	}
	m.update(func(s *Status) { s.Internet = check })
}

func fetchTrace(client *http.Client, target string) (ip, country string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d", response.StatusCode)
	}
	scanner := bufio.NewScanner(io.LimitReader(response.Body, 4096))
	for scanner.Scan() {
		key, value, _ := strings.Cut(scanner.Text(), "=")
		switch key {
		case "ip":
			ip = value
		case "loc":
			country = value
		}
	}
	if ip == "" {
		return "", "", errors.New("неожиданный ответ сервера проверки")
	}
	return ip, country, nil
}

func shortError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "нет ответа"
	}
	return err.Error()
}

// vpnCheckClient sends requests through the VPN adapter. Name resolution uses
// the tunnel DNS servers, not the transport's own resolver.
func vpnCheckClient(dns []netip.Addr) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if len(dns) > 0 {
		server := netip.AddrPortFrom(dns[0], 53).String()
		dialer.Resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, server)
			},
		}
	}
	return &http.Client{Transport: &http.Transport{
		Proxy:               nil,
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   true,
	}}
}

// proxyCheckClient sends requests through the local proxy, so the proxy
// resolves domain names on the far side of the tunnel.
func proxyCheckClient(scheme, address, username, password string) *http.Client {
	proxyURL := &url.URL{Scheme: scheme, Host: address}
	if username != "" {
		proxyURL.User = url.UserPassword(username, password)
	}
	return &http.Client{Transport: &http.Transport{
		Proxy:               http.ProxyURL(proxyURL),
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   true,
	}}
}
