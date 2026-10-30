<script lang="ts">
  import { onDestroy } from 'svelte';
  import Modal from '../components/Modal.svelte';
  import { api, events, errorText } from '../lib/api';
  import type { HashCheckResult, Profile } from '../lib/types';

  let { profile, onclose }: { profile: Profile; onclose: () => void } = $props();

  const labels: Record<string, string> = {
    ok: 'Работает', auth_required: 'Нужен вход', captcha: 'Капча', dead: 'Не найден', blocked: 'Запрещён',
    full: 'Нет мест', limited: 'Лимит', network: 'Сеть', error: 'Ошибка',
  };
  // The dialog checks the hashes the profile had when it was opened.
  // svelte-ignore state_referenced_locally
  let results = $state<(HashCheckResult | null)[]>(profile.hashes.map(() => null));
  let running = $state(true);
  let error = $state('');

  const off = events.hashCheck((event) => {
    if (event.result !== undefined && event.index !== undefined) results[event.index] = event.result;
    if (event.error) error = event.error;
    if (event.done) running = false;
  });
  onDestroy(off);

  // svelte-ignore state_referenced_locally
  api.checkHashes(profile.hashes).catch((e) => {
    error = errorText(e);
    running = false;
  });
</script>

<Modal title="Проверка VK-хешей" {onclose}>
  <table>
    <tbody>
      {#each profile.hashes as hash, i (hash)}
        {@const result = results[i]}
        <tr>
          <td class="mono small hash">{hash}</td>
          <td>
            {#if result}
              <span class="badge" class:ok={result.status === 'ok'} class:bad={result.status !== 'ok'}>
                {labels[result.status] ?? result.status}
              </span>
              <div class="muted small">{result.message}</div>
            {:else}
              <span class="muted">{running ? 'Проверяется…' : '—'}</span>
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
  {#if error}<p class="error-text">{error}</p>{/if}

  {#snippet footer()}
    <button onclick={onclose}>{running ? 'Скрыть' : 'Закрыть'}</button>
  {/snippet}
</Modal>

<style>
  .hash { max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
