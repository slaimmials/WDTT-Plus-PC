<script lang="ts">
  import { tick } from 'svelte';
  import { api } from '../lib/api';
  import { time } from '../lib/format';
  import { store } from '../lib/store.svelte';

  let filter = $state('');
  let follow = $state(true);
  let container: HTMLDivElement | undefined = $state();

  const visible = $derived(
    filter ? store.logs.filter((e) => e.message.toLowerCase().includes(filter.toLowerCase())) : store.logs,
  );

  $effect(() => {
    visible.length;
    if (follow && container) tick().then(() => container!.scrollTo({ top: container!.scrollHeight }));
  });

  async function copyAll() {
    const text = visible.map((e) => `${time(e.time)} ${e.message}`).join('\n');
    await store.run(() => api.copy(text), 'Журнал скопирован');
  }

  async function clear() {
    await api.clearLogs();
    store.logs = [];
  }
</script>

<div class="page">
  <div class="row spread">
    <h1>Журнал</h1>
    <div class="row">
      <input class="search" bind:value={filter} placeholder="Поиск" />
      <label class="row small"><input type="checkbox" bind:checked={follow} />Прокрутка</label>
      <button onclick={copyAll}>Копировать</button>
      <button onclick={() => api.openDataFolder()}>Папка</button>
      <button class="danger" onclick={clear}>Очистить</button>
    </div>
  </div>
  <div class="log card mono" bind:this={container}>
    {#each visible as entry, i (i)}
      <div class="line {entry.level}"><span class="muted">{time(entry.time)}</span> {entry.message}</div>
    {:else}
      <div class="muted">Записей нет</div>
    {/each}
  </div>
</div>

<style>
  .page { display: flex; flex-direction: column; height: 100%; }
  .search { width: 200px; }
  .log { flex: 1; overflow: auto; font-size: 12.5px; padding: 10px 12px; min-height: 0; }
  .line { white-space: pre-wrap; word-break: break-word; padding: 1px 0; }
  .line.error { color: var(--danger); }
  .line.warn { color: var(--warning); }
</style>
