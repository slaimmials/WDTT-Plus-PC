<script lang="ts">
  import Modal from '../components/Modal.svelte';
  import { api, errorText } from '../lib/api';
  import { store } from '../lib/store.svelte';
  import type { Server } from '../lib/types';

  let { server, onclose, onsaved }: {
    server: Server;
    onclose: () => void;
    onsaved: (server: Server) => void;
  } = $props();

  // svelte-ignore state_referenced_locally
  let draft = $state<Server>(structuredClone($state.snapshot(server)));
  let useKey = $state(draft.privateKey !== '');
  let error = $state('');

  async function save() {
    error = '';
    if (!useKey) {
      draft.privateKey = '';
      draft.keyPassphrase = '';
    }
    try {
      const saved = await api.saveServer($state.snapshot(draft));
      store.notify('Сервер сохранён');
      onsaved(saved);
    } catch (e) {
      error = errorText(e);
    }
  }
</script>

<Modal title={server.id ? 'Параметры сервера' : 'Новый сервер'} {onclose} wide>
  <div class="stack">
    <div class="grid-2">
      <label class="field"><span>Название</span><input bind:value={draft.name} placeholder="Мой VPS" /></label>
      <label class="field"><span>Адрес</span><input bind:value={draft.host} placeholder="IPv4 или домен" /></label>
    </div>

    <h2>SSH</h2>
    <div class="grid-3">
      <label class="field"><span>Пользователь</span><input bind:value={draft.user} /></label>
      <label class="field"><span>Порт</span><input type="number" bind:value={draft.sshPort} /></label>
      <label class="field">
        <span>Пароль {useKey ? '(для sudo)' : ''}</span>
        <input type="password" bind:value={draft.sshPassword} />
      </label>
    </div>
    <label class="row small"><input type="checkbox" bind:checked={useKey} />Вход по ключу</label>
    {#if useKey}
      <label class="field">
        <span>Закрытый ключ (OpenSSH или PEM)</span>
        <textarea rows="5" bind:value={draft.privateKey} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"></textarea>
      </label>
      <label class="field"><span>Пароль ключа</span><input type="password" bind:value={draft.keyPassphrase} /></label>
    {/if}
    {#if draft.hostKey}
      <p class="muted small">Ключ сервера запомнен при первом подключении. При смене адреса или порта он будет сброшен.</p>
    {/if}

    <h2>WDTT</h2>
    <label class="field">
      <span>Главный пароль администратора</span>
      <input type="password" bind:value={draft.mainPassword} />
    </label>
    <div class="grid-3">
      <label class="field"><span>DTLS-порт</span><input type="number" bind:value={draft.dtlsPort} /></label>
      <label class="field"><span>WG-порт</span><input type="number" bind:value={draft.wgPort} /></label>
      <label class="field"><span>Локальный порт клиента</span><input type="number" bind:value={draft.localPort} /></label>
    </div>
    <label class="field"><span>DNS для клиентов (через запятую)</span><input bind:value={draft.dns} /></label>
    <p class="muted small">Порты и DNS применяются при установке. Для работающего сервера меняйте их на вкладке «Сервер».</p>
    {#if error}<div class="error-text">{error}</div>{/if}
  </div>

  {#snippet footer()}
    <button onclick={onclose}>Отмена</button>
    <button class="primary" onclick={save}>Сохранить</button>
  {/snippet}
</Modal>
