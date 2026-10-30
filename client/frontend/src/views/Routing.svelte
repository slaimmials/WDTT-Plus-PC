<script lang="ts">
  import Segmented from '../components/Segmented.svelte';
  import Toggle from '../components/Toggle.svelte';
  import { api } from '../lib/api';
  import { store } from '../lib/store.svelte';
  import type { Routing, RoutingMode } from '../lib/types';

  let routing = $state<Routing>(structuredClone($state.snapshot(store.settings.routing)));
  let input = $state('');
  let inputErrors = $state<string[]>([]);

  const list = $derived(routing.mode === 'include' ? routing.include : routing.exclude);
  const killSwitch = $derived(routing.mode === 'all' && !routing.bypassLan);

  const modes: { value: RoutingMode; label: string }[] = [
    { value: 'all', label: 'Весь трафик' },
    { value: 'exclude', label: 'Кроме списка' },
    { value: 'include', label: 'Только список' },
  ];

  async function save() {
    await store.run(() => api.saveRouting($state.snapshot(routing)));
  }

  async function add() {
    const items = input.split(/[\s,;]+/).map((s) => s.trim()).filter(Boolean);
    const errors: string[] = [];
    const valid: string[] = [];
    for (const item of items) {
      const problem = await api.validateRule(item);
      if (problem) errors.push(`${item}: ${problem}`);
      else if (!list.includes(item)) valid.push(item);
    }
    inputErrors = errors;
    if (valid.length === 0) return;
    list.push(...valid);
    input = errors.length ? input : '';
    await save();
  }

  async function remove(rule: string) {
    const index = list.indexOf(rule);
    if (index >= 0) list.splice(index, 1);
    await save();
  }

  async function exportList() {
    await store.run(() => api.saveTextFile(`wdtt-${routing.mode}.txt`, list.join('\n')));
  }

  async function importList() {
    const text = await store.run(() => api.readTextFile());
    if (text) {
      input = text;
      await add();
    }
  }
</script>

<h1>Маршруты</h1>

<div class="stack">
  <div class="card stack">
    <Segmented bind:value={routing.mode} options={modes} onchange={save} />
    <p class="muted small">
      {#if routing.mode === 'all'}
        Весь интернет-трафик компьютера идёт через VPN.
      {:else if routing.mode === 'exclude'}
        Через VPN идёт всё, кроме адресов из списка ниже — они открываются напрямую.
      {:else}
        Через VPN идут только адреса из списка ниже, остальное открывается напрямую.
      {/if}
      Изменения применяются при следующем подключении.
    </p>
    <Toggle
      bind:checked={routing.bypassLan}
      onchange={save}
      label="Локальная сеть напрямую"
      hint="Роутер, принтеры и другие устройства дома доступны без VPN."
    />
    {#if killSwitch}
      <p class="small ok-text">Защита от утечек включена: трафик мимо VPN и DNS-запросы к чужим серверам блокируются.</p>
    {:else}
      <p class="small muted">Защита от утечек работает только в режиме «Весь трафик» без доступа к локальной сети.</p>
    {/if}
  </div>

  {#if routing.mode !== 'all'}
    <div class="card stack">
      <div class="row spread">
        <h2>{routing.mode === 'include' ? 'Через VPN' : 'Мимо VPN'} · {list.length}</h2>
        <div class="row">
          <button class="ghost" onclick={importList}>Импорт</button>
          <button class="ghost" onclick={exportList} disabled={list.length === 0}>Экспорт</button>
        </div>
      </div>
      <div class="row">
        <input
          class="grow"
          bind:value={input}
          placeholder="IPv4, подсеть 10.0.0.0/8, диапазон 1.1.1.1-1.1.1.9, домен или URL"
          onkeydown={(e) => e.key === 'Enter' && add()}
        />
        <button class="primary" onclick={add} disabled={!input.trim()}>Добавить</button>
      </div>
      {#each inputErrors as problem (problem)}<div class="error-text small">{problem}</div>{/each}
      <p class="muted small">Можно вставить сразу несколько значений через пробел, запятую или с новой строки. Домены превращаются в IP-адреса при подключении.</p>
      <div class="rules">
        {#each list as rule (rule)}
          <div class="rule row">
            <span class="mono grow">{rule}</span>
            <button class="ghost danger" onclick={() => remove(rule)} aria-label="Удалить">✕</button>
          </div>
        {:else}
          <div class="muted">Список пуст</div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .rules { display: flex; flex-direction: column; gap: 2px; max-height: 420px; overflow: auto; }
  .rule { padding: 2px 4px 2px 8px; border-radius: 6px; }
  .rule:hover { background: var(--surface-2); }
  .rule button { padding: 2px 8px; }
</style>
