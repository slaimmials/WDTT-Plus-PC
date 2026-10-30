<script lang="ts">
  import Segmented from '../components/Segmented.svelte';
  import { api, errorText } from '../lib/api';
  import { newServer } from '../lib/defaults';
  import { store } from '../lib/store.svelte';
  import type { AdminClient, AdminServerInfo, Server } from '../lib/types';
  import ServerBackups from './ServerBackups.svelte';
  import ServerClients from './ServerClients.svelte';
  import ServerDiagnostics from './ServerDiagnostics.svelte';
  import ServerEditor from './ServerEditor.svelte';
  import ServerInfo from './ServerInfo.svelte';
  import ServerInstall from './ServerInstall.svelte';

  type Tab = 'clients' | 'server' | 'backups' | 'install';
  const tabs: { value: Tab; label: string }[] = [
    { value: 'clients', label: 'Клиенты' },
    { value: 'server', label: 'Сервер' },
    { value: 'backups', label: 'Резервные копии' },
    { value: 'install', label: 'Установка' },
  ];

  let selectedId = $state(store.settings.servers[0]?.id ?? '');
  let tab = $state<Tab>('clients');
  let editing = $state<Server | null>(null);
  let diagnosing = $state<Server | null>(null);
  let loading = $state(false);
  let loadError = $state('');
  let info = $state<AdminServerInfo | null>(null);
  let clients = $state<AdminClient[]>([]);

  const selected = $derived(store.settings.servers.find((s) => s.id === selectedId));

  async function load() {
    if (!selected) return;
    loading = true;
    loadError = '';
    try {
      const response = await api.admin(selected.id, 'list');
      info = response.server ?? null;
      clients = response.passwords ?? [];
    } catch (e) {
      info = null;
      clients = [];
      loadError = errorText(e);
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    if (selectedId && (tab === 'clients' || tab === 'server')) load();
  });

  async function remove(server: Server) {
    const ok = await store.confirm('Удалить сервер из списка', `«${server.name}» исчезнет из программы. На самом сервере ничего не изменится.`, 'Удалить', true);
    if (!ok) return;
    await store.run(() => api.deleteServer(server.id));
    selectedId = store.settings.servers.find((s) => s.id !== server.id)?.id ?? '';
  }
</script>

<h1>Серверы</h1>

<div class="layout">
  <aside class="stack">
    {#each store.settings.servers as server (server.id)}
      <button class="card item" class:active={server.id === selectedId} onclick={() => (selectedId = server.id)}>
        <div class="name">{server.name}</div>
        <div class="muted small">{server.user}@{server.host}</div>
      </button>
    {/each}
    <button onclick={() => (editing = newServer())}>Добавить сервер</button>
  </aside>

  <section class="stack">
    {#if selected}
      <div class="row spread">
        <Segmented bind:value={tab} options={tabs} />
        <div class="row">
          {#if tab === 'clients' || tab === 'server'}
            <button class="ghost" onclick={load} disabled={loading}>Обновить</button>
          {/if}
          <button class="ghost" onclick={() => (diagnosing = selected)}>Диагностика</button>
          <button class="ghost" onclick={() => (editing = selected)}>Параметры</button>
          <button class="ghost danger" onclick={() => remove(selected)}>Удалить</button>
        </div>
      </div>

      {#if tab === 'install'}
        <ServerInstall server={selected} oninstalled={() => (tab = 'clients')} />
      {:else if tab === 'backups'}
        {#key selected.id}<ServerBackups server={selected} />{/key}
      {:else if loading && !info}
        <div class="muted">Подключение к серверу…</div>
      {:else if loadError}
        <div class="card stack">
          <div class="error-text">{loadError}</div>
          <div class="row">
            <button onclick={load}>Повторить</button>
            <button onclick={() => (tab = 'install')}>Перейти к установке</button>
          </div>
        </div>
      {:else if info}
        {#if tab === 'clients'}
          <div class="card"><ServerClients server={selected} {clients} onchange={load} /></div>
        {:else}
          <ServerInfo server={selected} {info} onchange={load} />
        {/if}
      {/if}
    {:else}
      <div class="card stack">
        <h2>Свой сервер WDTT Plus</h2>
        <p class="muted">
          Добавьте VPS с Linux, к которому есть доступ по SSH. Программа установит на него wdtt-server и позволит
          управлять клиентами, резервными копиями и настройками.
        </p>
        <div><button class="primary" onclick={() => (editing = newServer())}>Добавить сервер</button></div>
      </div>
    {/if}
  </section>
</div>

{#if diagnosing}
  <ServerDiagnostics server={diagnosing} onclose={() => (diagnosing = null)} />
{/if}

{#if editing}
  <ServerEditor
    server={editing}
    onclose={() => (editing = null)}
    onsaved={(server) => {
      editing = null;
      selectedId = server.id;
    }}
  />
{/if}

<style>
  .layout { display: grid; grid-template-columns: 220px 1fr; gap: 20px; align-items: start; }
  .item { text-align: left; padding: 10px 12px; }
  .item.active { border-color: var(--accent); }
  .name { font-weight: 600; }
</style>
