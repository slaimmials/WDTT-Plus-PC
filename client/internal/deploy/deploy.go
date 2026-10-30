// Package deploy installs and updates wdtt-server on a VPS over SSH using the
// bundled deploy.sh and server binaries.
package deploy

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"wdtt-plus/client/internal/admin"
	"wdtt-plus/client/internal/sshx"
)

//go:embed deploy.sh
var deployScript []byte

// assets holds wdtt-server-linux-<arch> binaries produced by the build script.
//
//go:embed all:assets
var assets embed.FS

type Mode string

const (
	// ModeInstall installs a new server or updates an existing one keeping data.
	ModeInstall Mode = "install"
	// ModeReset removes the existing installation with all clients first.
	ModeReset Mode = "reset"
)

type Options struct {
	Mode         Mode
	MainPassword string
	DTLSPort     int
	WGPort       int
	LocalPort    int
	SSHPort      int
	DNS          string
	Progress     func(fraction float64, step string)
}

type Result struct {
	Version string
	Fresh   bool
}

type state string

const (
	stateNone       state = "none"
	stateManaged    state = "managed"
	statePreserved  state = "preserved"
	stateStandalone state = "standalone"
	stateUnknown    state = "unknown"
)

const probeScript = `
standalone=0; managed=0; preserved=0; trace=0
if [ -f /var/lib/wdtt-server-installer/ownership ] &&
   grep -Fqx 'Managed by WDTT Plus standalone server installer' /var/lib/wdtt-server-installer/ownership; then
  standalone=1
fi
if [ -f /etc/systemd/system/wdtt.service ] &&
   grep -Fqx '# Managed by WDTT Plus Android deploy' /etc/systemd/system/wdtt.service; then
  managed=1
fi
if [ -f /etc/wdtt/.android-deploy-preserved ] &&
   grep -Fqx 'Preserved by WDTT Plus Android deploy' /etc/wdtt/.android-deploy-preserved; then
  preserved=1
fi
if [ -e /etc/systemd/system/wdtt.service ] || [ -e /usr/local/bin/wdtt-server ] || [ -e /etc/wdtt ]; then
  trace=1
fi
echo "WDTT_STANDALONE=$standalone"
echo "WDTT_MANAGED=$managed"
echo "WDTT_PRESERVED=$preserved"
echo "WDTT_TRACE=$trace"
echo "WDTT_ARCH=$(uname -m)"
`

func (o *Options) progress(fraction float64, step string) {
	log.Printf("[ДЕПЛОЙ] %s", step)
	if o.Progress != nil {
		o.Progress(fraction, step)
	}
}

func Run(ctx context.Context, client *sshx.Client, opts Options) (Result, error) {
	if err := validate(opts); err != nil {
		return Result{}, err
	}
	opts.progress(0.02, "Проверка прав и состояния сервера")
	output, err := client.RunRoot(ctx, probeScript, nil)
	if err != nil {
		return Result{}, err
	}
	current := classify(output)
	server, err := serverBinary(sshx.Marker(output, "WDTT_ARCH"))
	if err != nil {
		return Result{}, err
	}

	installMode, err := chooseInstallMode(current, opts.Mode)
	if err != nil {
		return Result{}, err
	}
	if installMode == "preserve" {
		opts.progress(0.05, "Резервная копия данных сервера")
		if _, err := admin.Call(ctx, client, opts.MainPassword, "backup-create", "--reason", "pre_deploy"); err != nil {
			log.Printf("[ДЕПЛОЙ] Резервная копия не создана: %v", err)
		}
	}

	opts.progress(0.08, "Загрузка файлов на сервер")
	staging, err := client.RunRoot(ctx, `d=$(mktemp -d /tmp/wdtt-deploy.XXXXXX) && chmod 700 "$d" && echo "WDTT_STAGING=$d"`, nil)
	if err != nil {
		return Result{}, err
	}
	dir := sshx.Marker(staging, "WDTT_STAGING")
	if !strings.HasPrefix(dir, "/tmp/wdtt-deploy.") {
		return Result{}, errors.New("не удалось создать временный каталог на сервере")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = client.RunRoot(cleanup, "rm -rf -- "+sshx.Quote(dir), nil)
	}()

	if err := client.Upload(ctx, dir+"/deploy.sh", deployScript, "600"); err != nil {
		return Result{}, err
	}
	if err := client.Upload(ctx, dir+"/server", server, "700"); err != nil {
		return Result{}, err
	}
	env := []string{
		"WDTT_DTLS_PORT=" + strconv.Itoa(opts.DTLSPort),
		"WDTT_WG_PORT=" + strconv.Itoa(opts.WGPort),
		"WDTT_SSH_PORT=" + strconv.Itoa(opts.SSHPort),
		"WDTT_INSTALL_MODE=" + installMode,
		"WDTT_STAGED_SERVER=" + sshx.Quote(dir+"/server"),
		"WDTT_DNS=" + sshx.Quote(opts.DNS),
	}
	if installMode == "preserve" {
		env = append(env, "WDTT_PRESERVE_DATA=1")
	} else {
		database, err := initialDatabase(opts)
		if err != nil {
			return Result{}, err
		}
		if err := client.Upload(ctx, dir+"/passwords.json", database, "600"); err != nil {
			return Result{}, err
		}
		env = append(env, "WDTT_PRESERVE_DATA=0", "WDTT_STAGED_DB="+sshx.Quote(dir+"/passwords.json"))
	}

	if installMode == "reset" {
		opts.progress(0.1, "Удаление старой установки")
		const wipe = "systemctl stop wdtt 2>/dev/null || true; pkill -x wdtt-server 2>/dev/null || true; " +
			"rm -rf /etc/wdtt; rm -f /etc/systemd/system/wdtt.service /usr/local/bin/wdtt-server; " +
			"systemctl daemon-reload"
		if _, err := client.RunRoot(ctx, wipe, nil); err != nil {
			return Result{}, err
		}
	}

	opts.progress(0.12, "Установка")
	command := "env " + strings.Join(env, " ") + " bash " + sshx.Quote(dir+"/deploy.sh")
	installCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	output, err = client.RunRootStream(installCtx, command, nil, func(line string) {
		if fraction, step, ok := parseProgress(line); ok {
			opts.progress(0.12+fraction*0.8, step)
		}
	})
	if err != nil {
		return Result{}, err
	}
	if sshx.Marker(output, "WDTT_INSTALL_RESULT") != "success" {
		return Result{}, errors.New("скрипт установки не подтвердил успешное завершение")
	}

	opts.progress(0.95, "Проверка установки")
	version, err := verify(ctx, client, server)
	if err != nil {
		return Result{}, err
	}
	opts.progress(1, "Сервер установлен")
	return Result{Version: version, Fresh: installMode != "preserve"}, nil
}

// Uninstall stops and removes the server while keeping /etc/wdtt.
func Uninstall(ctx context.Context, client *sshx.Client) error {
	output, err := client.RunRoot(ctx, probeScript, nil)
	if err != nil {
		return err
	}
	if classify(output) == stateStandalone {
		return errors.New("сервер установлен вручную через install.sh: удалите его тем же установщиком")
	}
	staging, err := client.RunRoot(ctx, `d=$(mktemp -d /tmp/wdtt-deploy.XXXXXX) && chmod 700 "$d" && echo "WDTT_STAGING=$d"`, nil)
	if err != nil {
		return err
	}
	dir := sshx.Marker(staging, "WDTT_STAGING")
	if !strings.HasPrefix(dir, "/tmp/wdtt-deploy.") {
		return errors.New("не удалось создать временный каталог на сервере")
	}
	defer func() { _, _ = client.RunRoot(context.Background(), "rm -rf -- "+sshx.Quote(dir), nil) }()
	if err := client.Upload(ctx, dir+"/deploy.sh", deployScript, "600"); err != nil {
		return err
	}
	_, err = client.RunRoot(ctx, "bash "+sshx.Quote(dir+"/deploy.sh")+" uninstall", nil)
	return err
}

func validate(opts Options) error {
	if opts.MainPassword == "" {
		return errors.New("задайте главный пароль администратора")
	}
	if strings.ContainsAny(opts.MainPassword, "\r\n") {
		return errors.New("главный пароль не должен содержать перенос строки")
	}
	for _, port := range []int{opts.DTLSPort, opts.WGPort, opts.LocalPort, opts.SSHPort} {
		if port < 1 || port > 65535 {
			return fmt.Errorf("некорректный порт %d", port)
		}
	}
	if opts.DTLSPort == opts.WGPort {
		return errors.New("DTLS- и WG-порты должны различаться")
	}
	for _, server := range strings.Split(opts.DNS, ",") {
		if strings.Trim(server, "0123456789abcdefABCDEF:.") != "" || server == "" {
			return fmt.Errorf("некорректный DNS %q", opts.DNS)
		}
	}
	return nil
}

func classify(output string) state {
	switch {
	case sshx.Marker(output, "WDTT_STANDALONE") == "1":
		return stateStandalone
	case sshx.Marker(output, "WDTT_MANAGED") == "1":
		return stateManaged
	case sshx.Marker(output, "WDTT_PRESERVED") == "1":
		return statePreserved
	case sshx.Marker(output, "WDTT_TRACE") == "0":
		return stateNone
	default:
		return stateUnknown
	}
}

func chooseInstallMode(current state, mode Mode) (string, error) {
	switch current {
	case stateStandalone:
		return "", errors.New("сервер установлен вручную через install.sh: обновляйте его тем же установщиком")
	case stateNone:
		return "fresh", nil
	}
	if mode == ModeReset {
		return "reset", nil
	}
	if current == stateUnknown {
		return "", errors.New("на сервере найдена неизвестная установка WDTT; выберите полную переустановку, если её можно удалить")
	}
	return "preserve", nil
}

func serverBinary(arch string) ([]byte, error) {
	var goarch string
	switch strings.TrimSpace(arch) {
	case "x86_64", "amd64":
		goarch = "amd64"
	case "aarch64", "arm64":
		goarch = "arm64"
	default:
		return nil, fmt.Errorf("архитектура сервера %q не поддерживается", arch)
	}
	data, err := assets.ReadFile("assets/wdtt-server-linux-" + goarch)
	if err != nil {
		return nil, fmt.Errorf("в эту сборку не включён wdtt-server для linux/%s", goarch)
	}
	return data, nil
}

func initialDatabase(opts Options) ([]byte, error) {
	return json.MarshalIndent(map[string]any{
		"main_password": opts.MainPassword,
		"admin_id":      "",
		"bot_token":     "",
		"dns":           opts.DNS,
		"default_ports": fmt.Sprintf("%d,%d,%d", opts.DTLSPort, opts.WGPort, opts.LocalPort),
		"max_passwords": 50,
		"passwords":     map[string]any{},
		"devices":       map[string]any{},
	}, "", "  ")
}

func parseProgress(line string) (float64, string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(line), "WDTT_PROGRESS|")
	if !ok {
		return 0, "", false
	}
	value, step, ok := strings.Cut(rest, "|")
	if !ok {
		return 0, "", false
	}
	fraction, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, "", false
	}
	return min(max(fraction, 0), 1), step, true
}

func verify(ctx context.Context, client *sshx.Client, server []byte) (string, error) {
	const script = `sleep 2
echo "SERVICE=$(systemctl is-active wdtt 2>/dev/null || true)"
echo "ADMIN_SOCKET=$([ -S /run/wdtt/admin.sock ] && echo 1 || echo 0)"
echo "SHA256=$(sha256sum /usr/local/bin/wdtt-server 2>/dev/null | cut -d' ' -f1)"
echo "VERSION=$(/usr/local/bin/wdtt-server --version 2>/dev/null | head -n 1)"`
	output, err := client.RunRoot(ctx, script, nil)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(server)
	var problems []string
	if sshx.Marker(output, "SERVICE") != "active" {
		problems = append(problems, "служба wdtt не запущена")
	}
	if sshx.Marker(output, "ADMIN_SOCKET") != "1" {
		problems = append(problems, "нет admin-сокета")
	}
	if sshx.Marker(output, "SHA256") != hex.EncodeToString(sum[:]) {
		problems = append(problems, "установлен не тот бинарник")
	}
	version := sshx.Marker(output, "VERSION")
	if version == "" {
		problems = append(problems, "не удалось определить версию")
	}
	if len(problems) > 0 {
		return "", errors.New("проверка после установки не прошла: " + strings.Join(problems, ", "))
	}
	return version, nil
}
