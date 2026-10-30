<script lang="ts">
  import Modal from '../components/Modal.svelte';
  import Segmented from '../components/Segmented.svelte';
  import Toggle from '../components/Toggle.svelte';
  import { api, errorText } from '../lib/api';
  import { store } from '../lib/store.svelte';
  import type { Mode, Profile } from '../lib/types';

  let { profile, onclose }: { profile: Profile; onclose: () => void } = $props();

  // svelte-ignore state_referenced_locally
  let draft = $state<Profile>(structuredClone($state.snapshot(profile)));
  let hashesText = $state(draft.hashes.join('\n'));
  let error = $state('');
  let saving = $state(false);
  let showAdvanced = $state(draft.streamOnly || draft.masque || draft.turnSni !== '');

  const modes: { value: Mode; label: string }[] = [
    { value: 'vpn', label: 'VPN' },
    { value: 'auto', label: 'Прокси' },
    { value: 'socks5', label: 'SOCKS5' },
    { value: 'http', label: 'HTTP' },
  ];
  const workerOptions = Array.from({ length: 12 }, (_, i) => (i + 1) * 9);

  async function save() {
    error = '';
    saving = true;
    try {
      draft.hashes = await api.normalizeHashes(hashesText);
      await api.saveProfile($state.snapshot(draft));
      store.notify('Профиль сохранён');
      onclose();
    } catch (e) {
      error = errorText(e);
    } finally {
      saving = false;
    }
  }
</script>

<Modal title={profile.host ? 'Профиль подключения' : 'Новый профиль'} {onclose} wide>
  <div class="stack">
    <div class="grid-2">
      <label class="field"><span>Название</span><input bind:value={draft.name} placeholder="Например, Дом" /></label>
      <label class="field"><span>Адрес сервера</span><input bind:value={draft.host} placeholder="vpn.example.com или IPv4" /></label>
    </div>
    <div class="grid-3">
      <label class="field"><span>DTLS-порт</span><input type="number" bind:value={draft.dtlsPort} /></label>
      <label class="field"><span>WG-порт</span><input type="number" bind:value={draft.wgPort} /></label>
      <label class="field"><span>Локальный порт</span><input type="number" bind:value={draft.localPort} /></label>
    </div>
    <label class="field"><span>Пароль подключения</span><input type="password" bind:value={draft.password} /></label>
    <label class="field">
      <span>VK-хеши или ссылки на звонки — по одному в строке</span>
      <textarea rows="4" bind:value={hashesText} placeholder="https://vk.com/call/join/…"></textarea>
    </label>
    <label class="field">
      <span>Потоков (каналов TURN)</span>
      <select bind:value={draft.workers}>
        {#each workerOptions as count (count)}<option value={count}>{count}</option>{/each}
      </select>
    </label>

    <div class="stack">
      <span class="muted small">Режим</span>
      <Segmented bind:value={draft.mode} options={modes} />
      {#if draft.mode === 'vpn'}
        <span class="muted small">Весь трафик Windows идёт через виртуальный адаптер. Нужны права администратора.</span>
      {:else}
        <span class="muted small">
          {draft.mode === 'auto'
            ? 'SOCKS5 и HTTP CONNECT на одном порту.'
            : draft.mode === 'socks5'
              ? 'Только SOCKS5 (TCP и UDP).'
              : 'Только HTTP CONNECT (TCP).'}
          Права администратора не нужны, приложения настраиваются на прокси вручную.
        </span>
        <div class="grid-3">
          <label class="field"><span>Порт прокси</span><input type="number" bind:value={draft.proxy.port} /></label>
          <label class="field"><span>Логин</span><input bind:value={draft.proxy.username} /></label>
          <label class="field"><span>Пароль</span><input type="password" bind:value={draft.proxy.password} /></label>
        </div>
        <Toggle bind:checked={draft.proxy.lan} label="Доступ из локальной сети" hint="Прокси будет доступен другим устройствам вашей сети; нужен логин и пароль." />
        {#if draft.mode === 'socks5'}
          <Toggle bind:checked={draft.proxy.udp} label="UDP через SOCKS5 (UDP ASSOCIATE)" />
        {/if}
      {/if}
    </div>

    <button class="ghost advanced" onclick={() => (showAdvanced = !showAdvanced)}>
      {showAdvanced ? '▾' : '▸'} Дополнительно
    </button>
    {#if showAdvanced}
      <div class="stack">
        <Toggle bind:checked={draft.streamOnly} label="Только TCP/TLS" hint="Для сетей, где UDP заблокирован или сильно ограничен." />
        <label class="field">
          <span>SNI для TURN/TLS (необязательно)</span>
          <input bind:value={draft.turnSni} placeholder="Домен из белого списка провайдера" />
        </label>
        <Toggle bind:checked={draft.masque} label="Резерв через Cloudflare WARP (MASQUE)" hint="Используется, если прямые TURN-подключения недоступны. Включая, вы принимаете условия Cloudflare WARP." />
      </div>
    {/if}

    {#if error}<div class="error-text">{error}</div>{/if}
  </div>

  {#snippet footer()}
    <button onclick={onclose}>Отмена</button>
    <button class="primary" onclick={save} disabled={saving}>Сохранить</button>
  {/snippet}
</Modal>

<style>
  .advanced { align-self: flex-start; padding-left: 0; }
</style>
