<script lang="ts">
  import Modal from '../components/Modal.svelte';
  import { api, errorText } from '../lib/api';
  import { store } from '../lib/store.svelte';
  import type { Server } from '../lib/types';

  let { server, onclose }: { server: Server; onclose: () => void } = $props();

  let output = $state('');
  let error = $state('');
  let running = $state(false);

  async function run() {
    running = true;
    error = '';
    try {
      output = await api.diagnoseServer(server.id);
    } catch (e) {
      error = errorText(e);
    } finally {
      running = false;
    }
  }

  run();
</script>

<Modal title="Диагностика сервера" {onclose} wide>
  <div class="stack">
    <p class="muted small">
      Только чтение: служба, пересылка трафика, NAT, выход сервера в интернет и последние записи журнала.
      Ключи и пароли не показываются.
    </p>
    {#if running}
      <div class="muted">Подключение к {server.host}…</div>
    {/if}
    {#if error}<div class="error-text">{error}</div>{/if}
    {#if output}<pre class="mono output">{output.trim()}</pre>{/if}
  </div>
  {#snippet footer()}
    <button onclick={() => store.run(() => api.copy(output), 'Скопировано')} disabled={!output}>Копировать</button>
    <button onclick={run} disabled={running}>Повторить</button>
    <button class="primary" onclick={onclose}>Закрыть</button>
  {/snippet}
</Modal>

<style>
  .output {
    margin: 0;
    padding: 10px 12px;
    max-height: 55vh;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-word;
    font-size: 12.5px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 8px;
  }
</style>
