<script lang="ts">
  import { api } from '../lib/api';
  import { bytes } from '../lib/format';
  import { store } from '../lib/store.svelte';
  import type { AdminServerInfo, Server } from '../lib/types';

  let { server, info, onchange }: {
    server: Server;
    info: AdminServerInfo;
    onchange: () => Promise<void>;
  } = $props();

  async function call(success: string, ...args: string[]) {
    const result = await store.run(() => api.admin(server.id, ...args));
    if (result) {
      store.notify(result.message || success);
      await onchange();
    }
  }

  async function edit(title: string, label: string, value: string, command: string, type: 'text' | 'number' = 'text') {
    const next = await store.ask(title, label, value, type);
    if (next !== null && next !== value) await call('Сохранено', command, '--value', next);
  }

  async function restart() {
    if (await store.confirm('Перезапуск службы', 'Подключённые клиенты переподключатся автоматически.', 'Перезапустить')) {
      await store.run(() => api.restartServer(server.id), 'Служба перезапущена');
    }
  }
</script>

<div class="stack">
  <div class="grid-3">
    <div class="card"><div class="muted small">Версия сервера</div>{info.wdtt_plus_version}</div>
    <div class="card"><div class="muted small">Подключений сейчас</div>{info.active_connections}</div>
    <div class="card"><div class="muted small">Клиентов</div>{info.password_count} из {info.max_passwords}</div>
    <div class="card"><div class="muted small">CPU / память</div>{info.cpu_percent.toFixed(0)}% / {info.memory_percent.toFixed(0)}%</div>
    <div class="card"><div class="muted small">Сеть</div>↓ {info.network_rx_mbps.toFixed(1)} · ↑ {info.network_tx_mbps.toFixed(1)} Мбит/с</div>
    <div class="card"><div class="muted small">Трафик за месяц</div>↓ {bytes(info.traffic.month.down)} · ↑ {bytes(info.traffic.month.up)}</div>
  </div>

  <div class="card stack">
    <h2>Параметры</h2>
    <div class="setting row spread">
      <div><div>Публичный адрес в ссылках</div><div class="muted small">{info.public_ip || `авто (${info.effective_public_ip ?? '—'})`}</div></div>
      <button onclick={() => edit('Публичный адрес', 'Домен или IP (auto — определить автоматически)', info.public_ip ?? 'auto', 'set-public-ip')}>Изменить</button>
    </div>
    <div class="setting row spread">
      <div><div>DNS для клиентов</div><div class="muted small">{info.dns || '—'}</div></div>
      <button onclick={() => edit('DNS для клиентов', 'Адреса через запятую', info.dns ?? '', 'set-dns')}>Изменить</button>
    </div>
    <div class="setting row spread">
      <div><div>Лимит клиентов</div><div class="muted small">{info.max_passwords}</div></div>
      <button onclick={() => edit('Лимит клиентов', 'Максимум клиентов', String(info.max_passwords), 'set-limit', 'number')}>Изменить</button>
    </div>
    <div class="setting row spread">
      <div><div>Порты новых клиентов</div><div class="muted small">{info.default_ports}</div></div>
      <button onclick={() => edit('Порты по умолчанию', 'DTLS,WG,локальный', info.default_ports, 'set-default-ports')}>Изменить</button>
    </div>
  </div>

  <div class="card stack">
    <h2>Обслуживание</h2>
    <div class="row wrap">
      <button onclick={restart}>Перезапустить службу</button>
      <button onclick={() => call('Готово', 'cleanup-expired')} disabled={info.expired_count === 0}>Удалить истёкших ({info.expired_count})</button>
      <button onclick={() => call('Готово', 'cleanup-orphans')} disabled={info.orphan_device_count === 0}>Удалить неиспользуемые устройства ({info.orphan_device_count})</button>
      <button class="danger" onclick={async () => (await store.confirm('Сброс статистики', 'Счётчики трафика всех клиентов будут обнулены.', 'Сбросить', true)) && call('Статистика сброшена', 'reset-traffic')}>Сбросить статистику трафика</button>
    </div>
  </div>
</div>

<style>
  .setting { padding: 6px 0; border-top: 1px solid var(--border); }
  .setting:first-of-type { border-top: none; }
</style>
