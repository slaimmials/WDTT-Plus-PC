# Идеи на будущее

## Режим без своего VPS: VK TURN → Cloudflare WARP

TURN-сервер VK пересылает UDP на любой адрес, который клиент разрешит через
`CreatePermission`. Значит, WireGuard-пакеты до Cloudflare WARP
(`engage.cloudflareclient.com:2408`) можно отправлять через TURN-relay VK, и
тогда свой сервер не нужен: выходной IP будет у Cloudflare.

Что понадобится:

- регистрация WARP-устройства (ключи и адрес) — код для API уже есть в
  `client/internal/core/masque_warp.go`;
- TURN-allocation и `CreatePermission` на адрес WARP вместо адреса VPS;
- WireGuard поверх relay без WRAP/DTLS (WARP говорит на чистом WireGuard);
- обработка ограничений Cloudflare на MTU и keepalive.

Риски: VK может запретить relay на адреса Cloudflare, а чистый WireGuard через
TURN хуже маскируется, чем DTLS до своего сервера. Нужна проверка на
практике, прежде чем делать это режимом в интерфейсе.
