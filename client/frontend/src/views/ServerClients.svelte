<script lang="ts">
  import Modal from '../components/Modal.svelte';
  import { api, errorText } from '../lib/api';
  import { bytes, dateTime } from '../lib/format';
  import { store } from '../lib/store.svelte';
  import type { AdminClient, Server } from '../lib/types';

  let { server, clients, onchange }: {
    server: Server;
    clients: AdminClient[];
    onchange: () => Promise<void>;
  } = $props();

  let creating = $state(false);
  let form = $state({ label: '', days: 30, hash: '', password: '' });
  let formError = $state('');

  const statusLabels: Record<string, [string, string]> = {
    active: ['Активен', 'ok'],
    expired: ['Истёк', 'bad'],
    expired_retained: ['Истёк, хранится', 'bad'],
    deactivated: ['Отключён', 'warn'],
    broken: ['Повреждён', 'bad'],
  };

  async function call(...args: string[]) {
    const result = await store.run(() => api.admin(server.id, ...args));
    if (result) {
      if (result.message) store.notify(result.message);
      await onchange();
    }
    return result;
  }

  async function create() {
    formError = '';
    const args = ['create', '--label', form.label.trim(), '--days', String(form.days)];
    if (form.hash.trim()) args.push('--vk-hash', form.hash.trim());
    if (form.password.trim()) args.push('--client-password', form.password.trim());
    try {
      const result = await api.admin(server.id, ...args);
      creating = false;
      form = { label: '', days: 30, hash: '', password: '' };
      await onchange();
      if (result.password) await copyLink(result.password);
    } catch (e) {
      formError = errorText(e);
    }
  }

  async function copyLink(client: AdminClient) {
    let hash = client.vk_hash ?? '';
    if (!hash) {
      const entered = await store.ask('VK-хеш для ссылки', 'У клиента нет VK-хеша. Хеш или ссылка на звонок:');
      if (!entered) return;
      hash = entered;
    }
    const link = await store.run(() => api.clientLink(server.id, client.password, hash, client.label ?? ''));
    if (link) await store.run(() => api.copy(link), 'Ссылка подключения скопирована');
  }

  async function edit(client: AdminClient, field: 'label' | 'hash' | 'expiry') {
    if (field === 'label') {
      const value = await store.ask('Название клиента', 'Название', client.label ?? '');
      if (value !== null) await call('set-label', '--password', client.password, '--label', value);
    } else if (field === 'hash') {
      const value = await store.ask('VK-хеш клиента', 'Хеш или ссылка на звонок', client.vk_hash ?? '');
      if (value !== null) await call('set-hash', '--password', client.password, '--vk-hash', value);
    } else {
      const value = await store.ask('Срок доступа', 'Дней от сегодняшнего дня (0 — бессрочно)', '30', 'number');
      if (value !== null) await call('set-expiry', '--password', client.password, '--days', value);
    }
  }

  async function remove(client: AdminClient) {
    const ok = await store.confirm('Удалить клиента', `Доступ «${client.label || client.password}» будет удалён без возможности восстановления.`, 'Удалить', true);
    if (ok) await call('delete', '--password', client.password);
  }
</script>

<div class="row spread">
  <h2>Клиенты · {clients.length}</h2>
  <button class="primary" onclick={() => (creating = true)}>Новый клиент</button>
</div>

<table>
  <thead>
    <tr><th>Клиент</th><th>Статус</th><th>До</th><th>Трафик</th><th>Устройство</th><th></th></tr>
  </thead>
  <tbody>
    {#each clients as client (client.password)}
      {@const [label, kind] = statusLabels[client.status] ?? [client.status, '']}
      <tr>
        <td>
          <div>{client.label || 'Без названия'}</div>
          <div class="muted small mono">{client.password}</div>
        </td>
        <td><span class="badge {kind}">{label}</span></td>
        <td class="small">{client.expires_at ? dateTime(client.expires_at) : 'бессрочно'}</td>
        <td class="small">↓ {bytes(client.down_bytes)}<br />↑ {bytes(client.up_bytes)}</td>
        <td class="small">{client.device_name || '—'}<div class="muted">{client.device_ip ?? ''}</div></td>
        <td>
          <div class="actions">
            <button class="ghost" onclick={() => copyLink(client)}>Ссылка</button>
            <button class="ghost" onclick={() => edit(client, 'label')}>Название</button>
            <button class="ghost" onclick={() => edit(client, 'hash')}>Хеш</button>
            <button class="ghost" onclick={() => edit(client, 'expiry')}>Срок</button>
            {#if client.device_id}
              <button class="ghost" onclick={() => call('unbind', '--password', client.password)}>Отвязать</button>
            {/if}
            {#if client.status === 'deactivated'}
              <button class="ghost" onclick={() => call('activate', '--password', client.password)}>Включить</button>
            {:else}
              <button class="ghost" onclick={() => call('deactivate', '--password', client.password)}>Отключить</button>
            {/if}
            <button class="ghost danger" onclick={() => remove(client)}>Удалить</button>
          </div>
        </td>
      </tr>
    {:else}
      <tr><td colspan="6" class="muted">Клиентов пока нет</td></tr>
    {/each}
  </tbody>
</table>

{#if creating}
  <Modal title="Новый клиент" onclose={() => (creating = false)}>
    <div class="stack">
      <label class="field"><span>Название</span><input bind:value={form.label} maxlength="40" /></label>
      <label class="field"><span>Срок, дней (0 — бессрочно)</span><input type="number" min="0" max="365" bind:value={form.days} /></label>
      <label class="field"><span>VK-хеш или ссылка на звонок (можно позже)</span><input bind:value={form.hash} /></label>
      <label class="field"><span>Пароль (пусто — сгенерировать)</span><input bind:value={form.password} /></label>
      <p class="muted small">После создания ссылка подключения будет скопирована в буфер обмена.</p>
      {#if formError}<div class="error-text">{formError}</div>{/if}
    </div>
    {#snippet footer()}
      <button onclick={() => (creating = false)}>Отмена</button>
      <button class="primary" onclick={create}>Создать</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .actions { display: flex; flex-wrap: wrap; gap: 2px; justify-content: flex-end; }
  .actions button { padding: 3px 7px; font-size: 12.5px; }
</style>
