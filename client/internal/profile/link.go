package profile

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	DefaultDTLSPort  = 56000
	DefaultWGPort    = 56001
	DefaultLocalPort = 9000
)

// Link is the content of a wdtt:// connection link.
type Link struct {
	Name       string
	Host       string
	DTLSPort   int
	WGPort     int
	LocalPort  int
	Password   string
	Hashes     []string
	MaxWorkers int
}

// String encodes the link in the current wdtt://connect format.
func (l Link) String() string {
	values := []string{
		"v=1",
		"host=" + escape(l.Host),
		"dtls=" + strconv.Itoa(l.DTLSPort),
		"wg=" + strconv.Itoa(l.WGPort),
		"local=" + strconv.Itoa(l.LocalPort),
		"password=" + escape(l.Password),
		"hashes=" + escape(strings.Join(l.Hashes, ",")),
	}
	if name := strings.TrimSpace(l.Name); name != "" {
		values = append(values, "name="+escape(name))
	}
	if l.MaxWorkers > 0 {
		values = append(values, "max_workers="+strconv.Itoa(l.MaxWorkers))
	}
	return "wdtt://connect?" + strings.Join(values, "&")
}

func escape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

// ExtractLink finds a wdtt:// link inside arbitrary pasted text.
func ExtractLink(text string) string {
	lower := strings.ToLower(text)
	start := strings.Index(lower, "wdtt://")
	if start < 0 {
		return ""
	}
	link := text[start:]
	if end := strings.IndexAny(link, " \t\r\n"); end >= 0 {
		link = link[:end]
	}
	return strings.TrimRight(link, ".,;)]}»\"'")
}

// ParseLink accepts both the current wdtt://connect?... format and the legacy
// wdtt://host:dtls:wg:local:password:hashes format.
func ParseLink(text string) (Link, error) {
	raw := ExtractLink(text)
	if raw == "" {
		return Link{}, errors.New("ссылка должна начинаться с wdtt://")
	}
	var (
		link Link
		err  error
	)
	if strings.HasPrefix(strings.ToLower(raw), "wdtt://connect?") {
		link, err = parseModernLink(raw)
	} else {
		link, err = parseLegacyLink(raw)
	}
	if err != nil {
		return Link{}, err
	}
	return link, link.validate()
}

func parseModernLink(raw string) (Link, error) {
	query, err := url.ParseQuery(raw[strings.Index(raw, "?")+1:])
	if err != nil {
		return Link{}, fmt.Errorf("ссылка повреждена: %w", err)
	}
	if query.Get("v") != "1" {
		return Link{}, errors.New("неподдерживаемая версия ссылки")
	}
	link := Link{
		Name:     strings.TrimSpace(query.Get("name")),
		Host:     strings.TrimSpace(query.Get("host")),
		Password: strings.TrimSpace(query.Get("password")),
	}
	ports := []struct {
		key    string
		target *int
	}{{"dtls", &link.DTLSPort}, {"wg", &link.WGPort}, {"local", &link.LocalPort}}
	for _, port := range ports {
		value, err := parsePort(query.Get(port.key))
		if err != nil {
			return Link{}, fmt.Errorf("поле %s: %w", port.key, err)
		}
		*port.target = value
	}
	if raw := query.Get("max_workers"); raw != "" {
		workers, err := strconv.Atoi(raw)
		if err != nil || workers <= 0 {
			return Link{}, errors.New("некорректное значение max_workers")
		}
		link.MaxWorkers = workers
	}
	if link.Hashes, err = NormalizeHashes(query.Get("hashes")); err != nil {
		return Link{}, err
	}
	return link, nil
}

func parseLegacyLink(raw string) (Link, error) {
	parts := strings.SplitN(raw[len("wdtt://"):], ":", 6)
	if len(parts) != 6 {
		return Link{}, fmt.Errorf("в старой ссылке должно быть 6 полей, найдено %d", len(parts))
	}
	link := Link{Host: strings.TrimSpace(parts[0]), Password: strings.TrimSpace(parts[4])}
	var err error
	for i, target := range []*int{&link.DTLSPort, &link.WGPort, &link.LocalPort} {
		if *target, err = parsePort(parts[i+1]); err != nil {
			return Link{}, err
		}
	}
	if link.Hashes, err = NormalizeHashes(parts[5]); err != nil {
		return Link{}, err
	}
	return link, nil
}

func (l Link) validate() error {
	if !ValidHost(l.Host) {
		return errors.New("неверный адрес сервера: нужен домен или IPv4 без порта и пути")
	}
	if l.Password == "" {
		return errors.New("в ссылке нет пароля подключения")
	}
	return nil
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("некорректный порт %q", value)
	}
	return port, nil
}

var hostnamePattern = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

func ValidHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.To4() != nil
	}
	return len(host) <= 253 && hostnamePattern.MatchString(host)
}

var hashPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,512}$`)

// ExtractHash returns the call hash from a VK call link or a bare hash.
func ExtractHash(input string) (string, bool) {
	value := strings.Trim(strings.TrimSpace(input), `<>"'`)
	lower := strings.ToLower(value)
	if i := strings.Index(lower, "/call/join/"); i >= 0 {
		value = value[i+len("/call/join/"):]
	} else if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return "", false
	}
	if i := strings.IndexAny(value, "?#/ \t"); i >= 0 {
		value = value[:i]
	}
	return value, hashPattern.MatchString(value)
}

// NormalizeHashes splits a list of VK hashes or call links separated by
// commas or whitespace and removes duplicates.
func NormalizeHashes(input string) ([]string, error) {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\r' || r == '\t'
	})
	seen := make(map[string]bool, len(fields))
	hashes := make([]string, 0, len(fields))
	for _, field := range fields {
		hash, ok := ExtractHash(field)
		if !ok {
			return nil, fmt.Errorf("не удалось распознать VK-хеш %q", field)
		}
		if !seen[hash] {
			seen[hash] = true
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}
