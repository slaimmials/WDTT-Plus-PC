<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, events } from '../lib/api';
  import { store } from '../lib/store.svelte';
  import type { Server } from '../lib/types';

  let { server, oninstalled }: { server: Server; oninstalled: () => void } = $props();

  let running = $state(false);
  let progress = $state(0);
  let step = $state('');
  let error = $state('');

  const off = events.deploy((event) => {
    if (event.serverId !== server.id) return;
    if (event.progress !== undefined) progress = event.progress;
    if (event.step) step = event.step;
    if (event.done) {
      running = false;
      error = event.error ?? '';
      if (!event.error) {
        store.notify('Сервер установлен');
        oninstalled();
      }
    }
  });
  onDestroy(off);

  async function start(mode: 'install' | 'reset') {
    if (!server.mainPassword) {
      store.notify('Задайте главный пароль администратора в параметрах сервера', 'error');
      return;
    }
    if (mode === 'reset') {
      const ok = await store.confirm(
        'Переустановка с нуля',
        'Все клиенты, их статистика и ключи WireGuard на сервере будут удалены. Выданные ссылки перестанут работать.',
        'Удалить и установить',
        true,
      );
      if (!ok) return;
    }
    error = '';
    progress = 0;
    step = 'Подключение по SSH';
    running = true;
    await store.run(() => api.deployServer(server.id, mode));
  }

  async function uninstall() {
    const ok = await store.confirm(
      'Удаление сервера',
      'Служба wdtt и правила межсетевого экрана будут удалены. База клиентов в /etc/wdtt сохранится.',
      'Удалить',
      true,
    );
    if (ok) await store.run(() => api.uninstallServer(server.id), 'Сервер удалён');
  }
</script>

<div class="card stack">
  <h2>Установка и обновление</h2>
  <p class="muted">
    Программа подключится к {server.host} по SSH, установит необходимые пакеты, wdtt-server и службу systemd,
    откроет порты {server.dtlsPort}/udp и {server.wgPort}/udp. Если сервер уже установлен этой программой или
    приложением WDTT Plus для Android, он обновится с сохранением клиентов; перед обновлением создаётся резервная копия.
  </p>
  <p class="muted small">Поддерживаются Debian 11+, Ubuntu 20.04+, CentOS/RHEL/Fedora/AlmaLinux/Rocky на x86_64 и ARM64.</p>

  {#if running || step}
    <div class="stack">
      <div class="bar"><div class="fill" style:width="{Math.round(progress * 100)}%"></div></div>
      <div class="small">{step}</div>
    </div>
  {/if}
  {#if error}<div class="error-text">{error}</div>{/if}

  <div class="row wrap">
    <button class="primary" onclick={() => start('install')} disabled={running}>Установить или обновить</button>
    <button class="danger" onclick={() => start('reset')} disabled={running}>Переустановить с нуля</button>
    <button class="danger" onclick={uninstall} disabled={running}>Удалить с сервера</button>
  </div>
</div>

<style>
  .bar { height: 8px; border-radius: 4px; background: var(--surface-2); overflow: hidden; }
  .fill { height: 100%; background: var(--accent); transition: width 0.3s; }
</style>
