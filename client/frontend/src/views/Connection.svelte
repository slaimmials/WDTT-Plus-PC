<script lang="ts">
  import Modal from '../components/Modal.svelte';
  import { api, errorText } from '../lib/api';
  import { newProfile } from '../lib/defaults';
  import { bytes, duration, rate } from '../lib/format';
  import { store } from '../lib/store.svelte';
  import type { Profile } from '../lib/types';
  import HashCheck from './HashCheck.svelte';
  import ProfileEditor from './ProfileEditor.svelte';

  const modeLabels: Record<string, string> = { vpn: 'VPN', auto: 'Прокси', socks5: 'SOCKS5', http: 'HTTP' };

  let editing = $state<Profile | null>(null);
  let checking = $state<Profile | null>(null);
  let importing = $state(false);
  let linkText = $state('');
  let linkError = $state('');
  let now = $state(Date.now());

  const status = $derived(store.status);
  const busy = $derived(status.state !== 'disconnected');
  const active = $derived(store.activeProfile);
  const connectedProfile = $derived(store.settings.profiles.find((p) => p.id === status.profileId));
  const needsAdmin = $derived(active?.mode === 'vpn' && !store.elevated);

  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), 1000);
    return () => clearInterval(timer);
  });

  $effect(() => {
    if (store.incomingLink) {
      linkText = store.incomingLink;
      store.incomingLink = '';
      importing = true;
      parseLink();
    }
  });

  async function toggle() {
    if (busy) {
      await store.run(() => api.disconnect());
      return;
    }
    if (!active) return;
    await store.run(() => api.connect(active.id));
  }

  async function parseLink() {
    linkError = '';
    try {
      editing = await api.parseLink(linkText);
      importing = false;
      linkText = '';
    } catch (e) {
      linkError = errorText(e);
    }
  }

  async function pasteLink() {
    linkText = (await api.paste()) ?? '';
    await parseLink();
  }

  async function copyLink(profile: Profile) {
    const link = await store.run(() => api.profileLink(profile.id));
    if (link) await store.run(() => api.copy(link), 'Ссылка скопирована');
  }

  async function remove(profile: Profile) {
    if (await store.confirm('Удалить профиль', `Профиль «${profile.name}» будет удалён.`, 'Удалить', true)) {
      await store.run(() => api.deleteProfile(profile.id));
    }
  }
</script>

<h1>Подключение</h1>

{#if needsAdmin}
  <div class="card banner row spread">
    <span>Режиму VPN нужны права администратора, чтобы создать сетевой адаптер.</span>
    <button class="primary" onclick={() => store.run(() => api.restartAsAdmin())}>Перезапустить от администратора</button>
  </div>
{/if}

<div class="layout">
  <section class="card hero stack">
    {#if active}
      <div class="muted small">Профиль</div>
      <div class="title">{(busy ? connectedProfile : active)?.name}</div>
      <div class="muted">{(busy ? connectedProfile : active)?.host} · {modeLabels[(busy ? connectedProfile : active)?.mode ?? 'vpn']}</div>

      <button
        class="power {status.state}"
        onclick={toggle}
        disabled={status.state === 'disconnecting' || (!busy && needsAdmin)}
        aria-label={busy ? 'Отключиться' : 'Подключиться'}>⏻</button
      >
      <div class="state">
        {#if status.state === 'connected'}
          Подключено · {duration(now - status.connectedAt)}
        {:else if status.state === 'connecting'}
          Подключение… Получаем доступ к TURN-серверам VK
        {:else if status.state === 'disconnecting'}
          Отключение…
        {:else}
          Отключено
        {/if}
      </div>

      {#if status.state === 'connected' || status.state === 'connecting'}
        <div class="grid-3 stats">
          <div><div class="muted small">Загрузка</div>{rate(store.speed.down)}<div class="muted small">{bytes(status.stats.BytesDown)}</div></div>
          <div><div class="muted small">Отдача</div>{rate(store.speed.up)}<div class="muted small">{bytes(status.stats.BytesUp)}</div></div>
          <div><div class="muted small">Каналов</div>{status.stats.ActiveConnections}</div>
        </div>
      {/if}
      {#if status.proxyAddresses?.length}
        <div class="stack proxy">
          <div class="muted small">Прокси слушает</div>
          {#each status.proxyAddresses as address (address)}
            <div class="row">
              <span class="mono grow">{address}</span>
              <button class="ghost" onclick={() => store.run(() => api.copy(address), 'Адрес скопирован')}>Копировать</button>
            </div>
          {/each}
        </div>
      {/if}
    {:else}
      <div class="title">Нет профилей</div>
      <p class="muted">Добавьте профиль по ссылке wdtt:// от администратора сервера или заполните его вручную.</p>
      <button class="primary" onclick={() => (importing = true)}>Добавить по ссылке</button>
    {/if}
  </section>

  <section class="stack">
    <div class="row spread">
      <h2>Профили</h2>
      <div class="row">
        <button onclick={() => (importing = true)}>По ссылке</button>
        <button onclick={() => (editing = newProfile())}>Вручную</button>
      </div>
    </div>
    {#each store.settings.profiles as profile (profile.id)}
      <div class="card profile" class:selected={profile.id === active?.id}>
        <button
          class="ghost select"
          disabled={busy}
          onclick={() => store.run(() => api.setActiveProfile(profile.id))}
        >
          <div class="name">{profile.name}</div>
          <div class="muted small">{profile.host}:{profile.dtlsPort} · {modeLabels[profile.mode]} · хешей: {profile.hashes.length}</div>
        </button>
        <div class="row actions">
          <button class="ghost" onclick={() => (editing = profile)} disabled={busy && profile.id === status.profileId}>Изменить</button>
          <button class="ghost" onclick={() => (checking = profile)} disabled={busy}>Проверить хеши</button>
          <button class="ghost" onclick={() => copyLink(profile)}>Ссылка</button>
          <button class="ghost danger" onclick={() => remove(profile)} disabled={busy && profile.id === status.profileId}>Удалить</button>
        </div>
      </div>
    {/each}
  </section>
</div>

{#if importing}
  <Modal title="Добавить по ссылке" onclose={() => (importing = false)}>
    <div class="stack">
      <label class="field">
        <span>Ссылка wdtt://</span>
        <textarea rows="4" bind:value={linkText} placeholder="wdtt://connect?v=1&host=…"></textarea>
      </label>
      {#if linkError}<div class="error-text">{linkError}</div>{/if}
    </div>
    {#snippet footer()}
      <button onclick={pasteLink}>Вставить из буфера</button>
      <button class="primary" onclick={parseLink} disabled={!linkText.trim()}>Продолжить</button>
    {/snippet}
  </Modal>
{/if}

{#if editing}
  <ProfileEditor profile={editing} onclose={() => (editing = null)} />
{/if}

{#if checking}
  <HashCheck profile={checking} onclose={() => (checking = null)} />
{/if}

<style>
  .banner { margin-bottom: 16px; border-color: var(--warning); }
  .layout { display: grid; grid-template-columns: minmax(300px, 1fr) minmax(320px, 1.1fr); gap: 20px; align-items: start; }
  .hero { align-items: center; text-align: center; padding: 28px 20px; }
  .title { font-size: 20px; font-weight: 600; }
  .power {
    width: 120px;
    height: 120px;
    border-radius: 50%;
    font-size: 44px;
    margin: 18px 0 6px;
    border-width: 2px;
  }
  .power.connected { border-color: var(--success); color: var(--success); box-shadow: 0 0 32px color-mix(in srgb, var(--success) 25%, transparent); }
  .power.connecting, .power.disconnecting { border-color: var(--warning); color: var(--warning); }
  .state { min-height: 22px; }
  .stats { width: 100%; margin-top: 12px; }
  .proxy { width: 100%; text-align: left; }
  .profile { padding: 10px 12px; }
  .profile.selected { border-color: var(--accent); }
  .select { width: 100%; text-align: left; padding: 4px 4px 8px; display: block; }
  .name { font-weight: 600; }
  .actions { flex-wrap: wrap; gap: 2px; }
  .actions button { padding: 4px 8px; font-size: 13px; }
</style>
