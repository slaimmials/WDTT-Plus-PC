<script lang="ts">
  import Segmented from '../components/Segmented.svelte';
  import Toggle from '../components/Toggle.svelte';
  import { api } from '../lib/api';
  import { store } from '../lib/store.svelte';
  import type { VK } from '../lib/types';

  let vk = $state<VK>(structuredClone($state.snapshot(store.settings.vk)));
  let theme = $state(store.settings.theme);
  let diagnostics = $state(store.settings.diagnostics);

  const themes: { value: 'system' | 'dark' | 'light'; label: string }[] = [
    { value: 'system', label: 'Как в системе' },
    { value: 'dark', label: 'Тёмная' },
    { value: 'light', label: 'Светлая' },
  ];

  const saveAppearance = () => store.run(() => api.saveAppearance(theme, diagnostics));
  const saveVK = () => store.run(() => api.saveVK($state.snapshot(vk)), 'Настройки VK сохранены');
</script>

<h1>Настройки</h1>

<div class="stack narrow">
  <div class="card stack">
    <h2>Оформление</h2>
    <Segmented bind:value={theme} options={themes} onchange={saveAppearance} />
    <Toggle
      bind:checked={diagnostics}
      onchange={saveAppearance}
      label="Подробные метрики транспорта в журнале"
      hint="Нужно только для диагностики проблем со скоростью."
    />
  </div>

  <div class="card stack">
    <h2>Получение доступа к VK</h2>
    <div class="stack">
      <span class="muted small">Способ получения доступа к TURN</span>
      <Segmented
        bind:value={vk.vkCallsPreflight}
        options={[
          { value: true, label: 'Быстрый (VK Calls)' },
          { value: false, label: 'Стандартный' },
        ]}
      />
      <span class="muted small">
        Быстрый обычно обходится без капчи. Стандартный — старый способ через приложения VK, иногда с капчей;
        пригодится, если быстрый перестал выдавать доступ.
      </span>
    </div>
    <Toggle
      bind:checked={vk.hashFallback}
      label="Резервные хеши"
      hint="Если звонок перестал работать, группы каналов переключаются на другие хеши профиля."
    />
    <label class="field">
      <span>TLS-отпечаток браузера</span>
      <select bind:value={vk.fingerprint}>
        <option value="firefox">Firefox</option>
        <option value="chrome">Chrome</option>
        <option value="safari">Safari</option>
        <option value="ios">iOS</option>
        <option value="android">Android</option>
      </select>
    </label>
    <label class="field">
      <span>Встроенные Client ID (через запятую, пусто — по умолчанию)</span>
      <input bind:value={vk.clientIds} />
    </label>
    <div class="grid-2">
      <label class="field"><span>Свой Client ID</span><input bind:value={vk.customClientId} /></label>
      <label class="field"><span>Свой Client Secret</span><input type="password" bind:value={vk.customSecret} /></label>
    </div>
    <div class="row"><button class="primary" onclick={saveVK}>Сохранить</button></div>
  </div>

  <div class="card stack">
    <h2>О программе</h2>
    <div>WDTT Plus для Windows · версия {store.version}</div>
    <div class="muted small">
      WireGuard поверх TURN-серверов VK. Основано на проекте WDTT, распространяется под лицензией GPLv3.
    </div>
    <div class="muted small mono">ID устройства: {store.settings.deviceId}</div>
  </div>
</div>

<style>
  .narrow { max-width: 680px; }
</style>
