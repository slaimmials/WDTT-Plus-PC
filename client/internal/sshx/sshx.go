// Package sshx is a small SSH client for managing the WDTT server: it runs
// commands as root through sudo and uploads files without SFTP.
package sshx

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Target struct {
	Host          string
	Port          int
	User          string
	Password      string
	PrivateKey    string
	KeyPassphrase string
	// HostKey is the base64 server key remembered from the first connection.
	// When empty, any key is accepted and reported through Client.HostKey.
	HostKey string
}

type Client struct {
	conn     *ssh.Client
	password string
	hostKey  string
}

var ErrHostKeyMismatch = errors.New("ключ SSH-сервера изменился: возможна подмена сервера")

func Dial(ctx context.Context, target Target) (*Client, error) {
	auth, err := authMethods(target)
	if err != nil {
		return nil, err
	}
	client := &Client{password: target.Password}
	config := &ssh.ClientConfig{
		User:    target.User,
		Auth:    auth,
		Timeout: 20 * time.Second,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			encoded := base64.StdEncoding.EncodeToString(key.Marshal())
			if target.HostKey != "" && target.HostKey != encoded {
				return ErrHostKeyMismatch
			}
			client.hostKey = encoded
			return nil
		},
	}

	address := net.JoinHostPort(target.Host, strconv.Itoa(target.Port))
	dialer := net.Dialer{Timeout: config.Timeout}
	raw, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("SSH %s: %w", address, err)
	}
	conn, chans, reqs, err := ssh.NewClientConn(raw, address, config)
	if err != nil {
		_ = raw.Close()
		if errors.Is(err, ErrHostKeyMismatch) {
			return nil, ErrHostKeyMismatch
		}
		return nil, fmt.Errorf("SSH %s: %w", address, err)
	}
	client.conn = ssh.NewClient(conn, chans, reqs)
	return client, nil
}

func authMethods(target Target) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if strings.TrimSpace(target.PrivateKey) != "" {
		var signer ssh.Signer
		var err error
		if target.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(target.PrivateKey), []byte(target.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(target.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("SSH-ключ: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if target.Password != "" {
		methods = append(methods,
			ssh.Password(target.Password),
			ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = target.Password
				}
				return answers, nil
			}))
	}
	if len(methods) == 0 {
		return nil, errors.New("укажите SSH-пароль или ключ")
	}
	return methods, nil
}

// HostKey returns the server key seen during the handshake.
func (c *Client) HostKey() string { return c.hostKey }

func (c *Client) Close() error { return c.conn.Close() }

// Run executes command and returns combined stdout and stderr.
func (c *Client) Run(ctx context.Context, command string, stdin []byte) (string, error) {
	return c.run(ctx, command, stdin, nil)
}

func (c *Client) run(ctx context.Context, command string, stdin []byte, onLine func(string)) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output := &lineBuffer{onLine: onLine}
	session.Stdout = output
	session.Stderr = output
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
		return output.String(), ctx.Err()
	}
	if err != nil {
		return output.String(), &CommandError{Err: err, Output: output.String()}
	}
	return output.String(), nil
}

type CommandError struct {
	Err    error
	Output string
}

func (e *CommandError) Error() string {
	if friendly := friendlySudoError(e.Output); friendly != "" {
		return friendly
	}
	output := strings.TrimSpace(e.Output)
	if len(output) > 2000 {
		output = output[len(output)-2000:]
	}
	if output == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%v: %s", e.Err, output)
}

func (e *CommandError) Unwrap() error { return e.Err }

const (
	frameStart = "__WDTT_STDIN_START__"
	frameEnd   = "__WDTT_STDIN_END__"
)

// RunRoot executes command with root privileges. When sudo is used, the SSH
// password is passed on stdin and framed so the command itself receives only
// the original stdin payload whether or not sudo asked for the password.
func (c *Client) RunRoot(ctx context.Context, command string, stdin []byte) (string, error) {
	return c.RunRootStream(ctx, command, stdin, nil)
}

// RunRootStream is RunRoot that also reports every output line as it arrives.
func (c *Client) RunRootStream(ctx context.Context, command string, stdin []byte, onLine func(string)) (string, error) {
	wrapped, payload, err := rootPlan(command, c.password, stdin)
	if err != nil {
		return "", err
	}
	return c.run(ctx, wrapped, payload, onLine)
}

func rootPlan(command, password string, stdin []byte) (string, []byte, error) {
	if strings.ContainsAny(password, "\r\n") {
		return "", nil, errors.New("пароль sudo не должен содержать перенос строки")
	}
	invalid := "echo 'error: invalid stdin frame' >&2; exit 97"
	forward := "IFS= read -r f1 || { " + invalid + "; }; " +
		"IFS= read -r f2 || { " + invalid + "; }; " +
		"if [ \"$f1\" = " + Quote(frameStart) + " ] && [ \"$f2\" = " + Quote(frameEnd) + " ]; then :; " +
		"elif [ \"$f2\" = " + Quote(frameStart) + " ]; then " +
		"IFS= read -r f3 || { " + invalid + "; }; [ \"$f3\" = " + Quote(frameEnd) + " ] || { " + invalid + "; }; " +
		"else " + invalid + "; fi; " +
		"exec bash -c " + Quote(command)
	wrapped := "if [ \"$(id -u)\" = 0 ]; then exec bash -c " + Quote(forward) + "; " +
		"elif command -v sudo >/dev/null 2>&1; then exec sudo -S -p '' bash -c " + Quote(forward) + "; " +
		"else echo 'error: root privileges required and sudo not found'; exit 1; fi"

	var payload bytes.Buffer
	payload.WriteString(password + "\n" + frameStart + "\n" + frameEnd + "\n")
	payload.Write(stdin)
	return wrapped, payload.Bytes(), nil
}

// lineBuffer collects output and reports every complete line.
type lineBuffer struct {
	mu      sync.Mutex
	all     bytes.Buffer
	partial []byte
	onLine  func(string)
}

func (b *lineBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.all.Write(p)
	if b.onLine == nil {
		return len(p), nil
	}
	b.partial = append(b.partial, p...)
	for {
		i := bytes.IndexByte(b.partial, '\n')
		if i < 0 {
			break
		}
		b.onLine(strings.TrimRight(string(b.partial[:i]), "\r"))
		b.partial = b.partial[i+1:]
	}
	return len(p), nil
}

func (b *lineBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.all.String()
}

// Upload writes data to path with the given octal mode, as root.
func (c *Client) Upload(ctx context.Context, path string, data []byte, mode string) error {
	command := "umask 077 && cat > " + Quote(path) + " && chmod " + mode + " " + Quote(path)
	_, err := c.RunRoot(ctx, command, data)
	return err
}

// Quote returns value as a single-quoted shell word.
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func friendlySudoError(output string) string {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "incorrect password"), strings.Contains(lower, "sorry, try again"),
		strings.Contains(lower, "a password is required"), strings.Contains(lower, "authentication failure"):
		return "sudo не принял пароль: проверьте SSH-пароль пользователя"
	case strings.Contains(lower, "is not in the sudoers file"), strings.Contains(lower, "is not allowed to execute"):
		return "у SSH-пользователя нет прав sudo"
	case strings.Contains(lower, "root privileges required"):
		return "нужен пользователь root или установленный sudo"
	}
	return ""
}

// Marker extracts KEY=value from command output.
func Marker(output, key string) string {
	prefix := key + "="
	value := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			value = strings.TrimPrefix(line, prefix)
		}
	}
	return value
}
