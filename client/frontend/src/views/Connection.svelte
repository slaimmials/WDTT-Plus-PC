<script lang="ts">
  import Menu from '../components/Menu.svelte';
  import Modal from '../components/Modal.svelte';
  import { api, errorText } from '../lib/api';
  import { newProfile } from '../lib/defaults';
  import { bytes, duration, rate } from '../lib/format';
  import { store } from '../lib/store.svelte';
  import type { Mode, PingResult, Profile } from '../lib/types';
  import HashCheck from './HashCheck.svelte';
  import ProfileEditor from './ProfileEditor.svelte';

  const modeLabels: Record<Mode, string> = { vpn: 'VPN', auto: 'Прокси', socks5: 'SOCKS5', http: 'HTTP' };

  let editing = $state<Profile | null>(null);
  let checking = $state<Profile | null>(null);
  let importing = $state(false);
  let linkText = $state('');
  let linkError = $state('');
  let search = $state('');
  let now = $state(Date.now());

  const status = $derived(store.status);
  const busy = $derived(status.state !== 'disconnected');
  const active = $derived(store.activeProfile);
  const shown = $derived(busy ? (store.settings.profiles.find((p) => p.id === status.profileId) ?? active) : active);
  const needsAdmin = $derived(!busy && active?.mode === 'vpn' && !store.elevated);
  const testing = $derived(Object.values(store.pings).some((p) => p.testing));
  const profiles = $derived(
    search.trim()
      ? store.settings.profiles.filter((p) =>
          `${p.name} ${p.host}`.toLowerCase().includes(search.trim().toLowerCase()),
        )
      : store.settings.profiles,
  );

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

  function pingOf(profile: Profile): PingResult | undefined {
    if (busy && profile.id === status.profileId && status.stats.LatencyMs > 0) {
      return { latencyMs: status.stats.LatencyMs };
    }
    return store.pings[profile.id];
  }

  function pingClass(ping: PingResult | undefined): string {
    if (!ping || ping.testing) return '';
    if (ping.error) return 'bad';
    const ms = ping.latencyMs ?? 0;
    return ms < 150 ? 'ok' : ms < 400 ? 'warn' : 'bad';
  }

  async function toggle() {
    if (busy) {
      await store.run(() => api.disconnect());
    } else if (active) {
      await store.run(() => api.connect(active.id));
    }
  }

  async function runPing(ids: string[]) {
    await store.run(() => api.pingProfiles(ids));
  }

  async function setMode(mode: Mode) {
    if (!active || active.mode === mode) return;
    await store.run(() => api.saveProfile({ ...$state.snapshot(active), mode }));
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

<div class="page">
  <section class="list">
    <h1>Профили</h1>
    <div class="row toolbar">
      <input class="grow" bind:value={search} placeholder="Поиск" />
      <button
        class="icon"
        title="Проверить все профили"
        disabled={busy || testing || store.settings.profiles.length === 0}
        onclick={() => runPing(store.settings.profiles.map((p) => p.id))}>⟳</button
      >
      <Menu
        label="Добавить профиль"
        icon="+"
        items={[
          { label: 'Добавить по ссылке', action: () => (importing = true) },
          { label: 'Создать вручную', action: () => (editing = newProfile()) },
        ]}
      />
    </div>

    <div class="profiles">
      {#each profiles as profile (profile.id)}
        {@const ping = pingOf(profile)}
        {@const current = busy && profile.id === status.profileId}
        <div class="profile" class:selected={profile.id === active?.id}>
          <button
            class="select"
            disabled={busy}
            onclick={() => store.run(() => api.setActiveProfile(profile.id))}
          >
            <span class="avatar" class:on={current}>{profile.name.slice(0, 1).toUpperCase()}</span>
            <span class="info">
              <span class="name">{profile.name}</span>
              <span class="muted small">{modeLabels[profile.mode]} · {profile.host}:{profile.dtlsPort} · хешей: {profile.hashes.length}</span>
            </span>
          </button>
          <span class="ping {pingClass(ping)}" title={ping?.error ?? ''}>
            {#if ping?.testing}…{:else if ping?.error}ошибка{:else if ping?.latencyMs}{ping.latencyMs} мс{:else}—{/if}
          </span>
          <Menu
            items={[
              { label: 'Проверить пинг', action: () => runPing([profile.id]), disabled: busy || testing },
              { label: 'Изменить', action: () => (editing = profile), disabled: current },
              { label: 'Проверить хеши', action: () => (checking = profile), disabled: busy },
              { label: 'Скопировать ссылку', action: () => copyLink(profile) },
              { label: 'Удалить', action: () => remove(profile), danger: true, disabled: current },
            ]}
          />
        </div>
      {:else}
        <div class="empty card stack">
          {#if store.settings.profiles.length}
            <span class="muted">Ничего не найдено</span>
          {:else}
            <span>Профилей пока нет</span>
            <span class="muted small">Добавьте профиль по ссылке wdtt:// от администратора сервера или создайте вручную.</span>
            <div class="row">
              <button class="primary" onclick={() => (importing = true)}>По ссылке</button>
              <button onclick={() => (editing = newProfile())}>Вручную</button>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </section>

  <section class="hero">
    <div class="row top">
      {#if needsAdmin}
        <button class="admin" onclick={() => store.run(() => api.restartAsAdmin())}>
          Нужны права администратора — перезапустить
        </button>
      {/if}
      <select
        class="mode"
        value={shown?.mode ?? 'vpn'}
        disabled={busy || !active}
        onchange={(e) => setMode(e.currentTarget.value as Mode)}
      >
        {#each Object.entries(modeLabels) as [value, label] (value)}
          <option {value}>{label}</option>
        {/each}
      </select>
    </div>

    <div class="center">
      <div class="rings {status.state}">
        <button
          class="power {status.state}"
          onclick={toggle}
          disabled={!active || status.state === 'disconnecting' || needsAdmin}
          aria-label={busy ? 'Отключиться' : 'Подключиться'}>⏻</button
        >
      </div>
      <div class="state">
        {#if status.state === 'connected' && status.reconnecting}
          Каналы потеряны, переподключение…
        {:else if status.state === 'connected'}
          Подключено · {duration(now - status.connectedAt)}
        {:else if status.state === 'connecting'}
          Подключение…
        {:else if status.state === 'disconnecting'}
          Отключение…
        {:else}
          Отключено
        {/if}
      </div>
      {#if status.state === 'connected' && status.internet.state}
        <div class="internet {status.internet.state}">
          {#if status.internet.state === 'checking'}
            Проверяем доступ в интернет…
          {:else if status.internet.state === 'ok'}
            Интернет работает · IP {status.internet.ip}{status.internet.country ? ` (${status.internet.country})` : ''}
          {:else}
            {status.internet.message}
          {/if}
        </div>
      {/if}

      {#if status.state === 'connected' || status.state === 'connecting'}
        <div class="stats">
          <div><span class="muted small">Загрузка</span>{rate(store.speed.down)}<span class="muted small">{bytes(status.stats.BytesDown)}</span></div>
          <div><span class="muted small">Отдача</span>{rate(store.speed.up)}<span class="muted small">{bytes(status.stats.BytesUp)}</span></div>
          <div><span class="muted small">Пинг</span>{status.stats.LatencyMs ? `${status.stats.LatencyMs} мс` : '—'}<span class="muted small">каналов: {status.stats.ActiveConnections}</span></div>
        </div>
      {/if}
      {#if status.proxyAddresses?.length}
        <div class="proxy">
          {#each status.proxyAddresses as address (address)}
            <button class="ghost mono" title="Скопировать" onclick={() => store.run(() => api.copy(address), 'Адрес скопирован')}>{address}</button>
          {/each}
        </div>
      {/if}
    </div>

    {#if shown}
      <div class="bottom">
        <div class="title">{shown.name}</div>
        <div class="muted small">{shown.host}</div>
        {#if !busy}
          <button class="primary" disabled={testing} onclick={() => runPing([shown.id])}>
            {store.pings[shown.id]?.testing ? 'Проверка…' : 'Тест пинга'}
          </button>
        {/if}
      </div>
    {/if}
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
  .page {
    display: grid;
    grid-template-columns: minmax(340px, 1fr) minmax(360px, 1fr);
    gap: 20px;
    height: 100%;
    min-height: 520px;
  }
  .list { display: flex; flex-direction: column; min-height: 0; }
  .toolbar { margin-bottom: 12px; }
  .icon { padding: 7px 11px; }
  .profiles { display: flex; flex-direction: column; gap: 6px; overflow: auto; min-height: 0; padding-bottom: 8px; }

  .profile {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px 4px 4px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
  }
  .profile.selected { border-color: var(--accent); background: var(--surface-2); }
  .select {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 12px;
    align-items: center;
    border: none;
    background: transparent;
    text-align: left;
    padding: 8px;
  }
  .select:disabled { opacity: 1; cursor: default; }
  .avatar {
    flex: none;
    width: 34px;
    height: 34px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    font-weight: 600;
    background: var(--surface-2);
    border: 1px solid var(--border);
  }
  .avatar.on { border-color: var(--success); color: var(--success); }
  .info { display: flex; flex-direction: column; min-width: 0; }
  .info > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name { font-weight: 600; }
  .ping { flex: none; min-width: 64px; text-align: right; font-size: 13px; color: var(--muted); }
  .ping.ok { color: var(--success); }
  .ping.warn { color: var(--warning); }
  .ping.bad { color: var(--danger); }
  .empty { margin-top: 4px; }

  .hero {
    display: flex;
    flex-direction: column;
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 18px;
    background:
      radial-gradient(circle at 50% 42%, color-mix(in srgb, var(--accent) 12%, transparent), transparent 55%),
      var(--surface);
  }
  .top { justify-content: flex-end; }
  .mode { width: auto; min-width: 140px; }
  .admin { border-color: var(--warning); color: var(--warning); margin-right: auto; white-space: normal; text-align: left; }
  .center { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 14px; }
  .rings {
    padding: 22px;
    border-radius: 50%;
    border: 1px solid color-mix(in srgb, var(--accent) 25%, transparent);
    box-shadow: 0 0 0 18px color-mix(in srgb, var(--accent) 5%, transparent);
    transition: border-color 0.3s, box-shadow 0.3s;
  }
  .rings.connected {
    border-color: color-mix(in srgb, var(--success) 45%, transparent);
    box-shadow: 0 0 0 18px color-mix(in srgb, var(--success) 7%, transparent);
  }
  .rings.connecting, .rings.disconnecting { border-color: color-mix(in srgb, var(--warning) 45%, transparent); }
  .power {
    width: 132px;
    height: 132px;
    border-radius: 50%;
    font-size: 46px;
    border-width: 2px;
    background: var(--surface-2);
    color: var(--muted);
  }
  .power.connected { border-color: var(--success); color: var(--success); }
  .power.connecting, .power.disconnecting { border-color: var(--warning); color: var(--warning); }
  .state { font-size: 15px; }
  .internet { font-size: 13px; color: var(--muted); text-align: center; max-width: 360px; }
  .internet.ok { color: var(--success); }
  .internet.failed { color: var(--danger); }
  .stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 18px; text-align: center; }
  .stats > div { display: flex; flex-direction: column; }
  .proxy { display: flex; flex-wrap: wrap; justify-content: center; gap: 4px; }
  .bottom { display: flex; flex-direction: column; align-items: center; gap: 4px; }
  .title { font-size: 17px; font-weight: 600; }
  .bottom button { margin-top: 10px; min-width: 200px; padding: 9px 18px; }
</style>
