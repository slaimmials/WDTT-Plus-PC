package admin

import (
	"context"

	"wdtt-plus/client/internal/sshx"
)

// diagnoseScript prints the server state relevant to "connected but no
// internet" problems. It reads nothing secret: no keys and no passwords.
const diagnoseScript = `
section() { printf '\n== %s\n' "$1"; }
section "Версия wdtt-server"
/usr/local/bin/wdtt-server --version 2>&1 | head -n 1
section "Служба wdtt"
systemctl is-active wdtt 2>&1
section "Пересылка IPv4 (должно быть 1)"
sysctl -n net.ipv4.ip_forward 2>&1
section "Интерфейс wdtt0"
ip -brief addr show wdtt0 2>&1
section "NAT (MASQUERADE)"
{ iptables -t nat -S POSTROUTING 2>/dev/null; nft list ruleset 2>/dev/null; } | grep -i masquerade || echo "правил MASQUERADE не найдено"
section "Пиры WireGuard"
if command -v wg >/dev/null 2>&1; then
  wg show wdtt0 2>&1 | grep -E 'peer:|allowed ips|latest handshake|transfer'
else
  echo "утилита wg не установлена"
fi
section "Выход в интернет с сервера"
if command -v curl >/dev/null 2>&1; then
  curl -4 -s -m 6 https://1.1.1.1/cdn-cgi/trace | grep -E '^(ip|loc)=' || echo "нет ответа"
else
  ping -4 -c 2 -W 3 1.1.1.1 2>&1 | tail -n 2
fi
section "Журнал службы"
journalctl -u wdtt -n 40 --no-pager 2>&1
`

// Diagnose runs read-only checks on the server and returns their output.
func Diagnose(ctx context.Context, client *sshx.Client) (string, error) {
	return client.RunRoot(ctx, diagnoseScript, nil)
}
