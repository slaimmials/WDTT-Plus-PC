<script lang="ts">
  import { onMount } from 'svelte';
  import { store } from './lib/store.svelte';
  import Connection from './views/Connection.svelte';
  import RoutingView from './views/Routing.svelte';
  import Servers from './views/Servers.svelte';
  import Logs from './views/Logs.svelte';
  import SettingsView from './views/Settings.svelte';
  import Dialogs from './components/Dialogs.svelte';

  type Page = 'connection' | 'routing' | 'servers' | 'logs' | 'settings';
  const pages: { id: Page; label: string; icon: string }[] = [
    { id: 'connection', label: 'Подключение', icon: '⏻' },
    { id: 'routing', label: 'Маршруты', icon: '⇄' },
    { id: 'servers', label: 'Серверы', icon: '☰' },
    { id: 'logs', label: 'Журнал', icon: '≡' },
    { id: 'settings', label: 'Настройки', icon: '⚙' },
  ];
  let page = $state<Page>('connection');

  onMount(() => {
    store.init();
  });

  $effect(() => {
    const media = window.matchMedia('(prefers-color-scheme: light)');
    const apply = () => {
      const theme = store.settings.theme === 'system' ? (media.matches ? 'light' : 'dark') : store.settings.theme;
      document.documentElement.dataset.theme = theme;
    };
    apply();
    media.addEventListener('change', apply);
    return () => media.removeEventListener('change', apply);
  });

  $effect(() => {
    if (store.incomingLink) page = 'connection';
  });

  const stateLabel = $derived(
    {
      disconnected: 'Отключено',
      connecting: 'Подключение…',
      connected: 'Подключено',
      disconnecting: 'Отключение…',
    }[store.status.state],
  );
</script>

<div class="layout">
  <nav>
    <div class="brand">WDTT Plus</div>
    {#each pages as item (item.id)}
      <button class="nav-item" class:active={page === item.id} onclick={() => (page = item.id)}>
        <span class="icon">{item.icon}</span>{item.label}
      </button>
    {/each}
    <div class="status {store.status.state}">
      <span class="dot"></span>{stateLabel}
    </div>
  </nav>

  <main>
    {#if store.ready}
      {#if page === 'connection'}
        <Connection />
      {:else if page === 'routing'}
        <RoutingView />
      {:else if page === 'servers'}
        <Servers />
      {:else if page === 'logs'}
        <Logs />
      {:else}
        <SettingsView />
      {/if}
    {/if}
  </main>
</div>

<Dialogs />

<div class="toasts">
  {#each store.toasts as toast (toast.id)}
    <div class="toast card" class:error={toast.kind === 'error'}>{toast.text}</div>
  {/each}
</div>

<style>
  .layout { display: grid; grid-template-columns: 200px 1fr; height: 100%; }
  nav {
    background: var(--surface);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 16px 10px;
  }
  .brand { font-weight: 700; font-size: 16px; padding: 4px 10px 16px; }
  .nav-item {
    display: flex;
    gap: 10px;
    align-items: center;
    border: none;
    background: transparent;
    text-align: left;
    padding: 9px 10px;
  }
  .nav-item:hover { background: var(--surface-2); }
  .nav-item.active { background: var(--surface-2); color: var(--accent); }
  .icon { width: 18px; text-align: center; }
  .status { margin-top: auto; display: flex; gap: 8px; align-items: center; padding: 10px; color: var(--muted); }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--muted); }
  .status.connected .dot { background: var(--success); }
  .status.connecting .dot, .status.disconnecting .dot { background: var(--warning); }
  main { overflow: auto; padding: 24px 28px; }
  .toasts { position: fixed; right: 16px; bottom: 16px; display: flex; flex-direction: column; gap: 8px; z-index: 30; }
  .toast { padding: 10px 14px; max-width: 420px; box-shadow: 0 6px 24px rgb(0 0 0 / 0.3); user-select: text; }
  .toast.error { border-color: var(--danger); }
</style>
