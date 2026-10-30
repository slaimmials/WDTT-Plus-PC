<script lang="ts">
  import { onMount } from 'svelte';
  import Toggle from '../components/Toggle.svelte';
  import { api } from '../lib/api';
  import { bytes, dateTime } from '../lib/format';
  import { store } from '../lib/store.svelte';
  import type { BackupStatus, Server } from '../lib/types';

  let { server }: { server: Server } = $props();

  const reasons: Record<string, string> = {
    manual: 'вручную',
    scheduled: 'по расписанию',
    enabled: 'при включении',
    pre_deploy: 'перед обновлением',
    pre_restore: 'перед восстановлением',
  };

  let status = $state<BackupStatus | null>(null);
  let loading = $state(false);

  async function call(...args: string[]) {
    loading = true;
    const result = await store.run(() => api.admin(server.id, ...args));
    loading = false;
    if (result?.backup_status) status = result.backup_status;
    return result;
  }

  onMount(() => {
    call('backup-status');
  });

  async function create() {
    const result = await call('backup-create', '--reason', 'manual');
    if (result) store.notify('Резервная копия создана');
  }

  async function verify(id: string) {
    const result = await call('backup-verify', '--id', id);
    if (result) store.notify(result.message || 'Копия проверена');
    await call('backup-status');
  }

  async function exportBackup(id: string) {
    const result = await call('backup-export', '--id', id);
    if (result?.backup_document) {
      await store.run(
        () => api.saveTextFile(`wdtt-backup-${id}.json`, JSON.stringify(result.backup_document, null, 2)),
      );
    }
  }

  async function remove(id: string) {
    if (await store.confirm('Удалить копию', 'Резервная копия будет удалена с сервера.', 'Удалить', true)) {
      await call('backup-delete', '--id', id, '--confirm', 'УДАЛИТЬ');
    }
  }

  async function configure() {
    if (!status) return;
    const { enabled, interval_hours, retention_count } = status.policy;
    await call('backup-configure', '--enabled', String(enabled), '--interval-hours', String(interval_hours), '--retention', String(retention_count));
  }
</script>

<div class="stack">
  {#if status}
    <div class="card stack">
      <h2>Автоматические копии</h2>
      <Toggle bind:checked={status.policy.enabled} label="Создавать копии по расписанию" onchange={configure} />
      <div class="grid-2">
        <label class="field"><span>Интервал, часов</span><input type="number" min="6" max="168" bind:value={status.policy.interval_hours} onchange={configure} /></label>
        <label class="field"><span>Хранить копий</span><input type="number" min="2" max="90" bind:value={status.policy.retention_count} onchange={configure} /></label>
      </div>
      {#if status.policy.enabled && status.next_run_at}
        <div class="muted small">Следующая копия: {dateTime(status.next_run_at)}</div>
      {/if}
    </div>
  {/if}

  <div class="card stack">
    <div class="row spread">
      <h2>Копии на сервере {status ? `· ${bytes(status.total_bytes)}` : ''}</h2>
      <button class="primary" onclick={create} disabled={loading}>Создать копию</button>
    </div>
    <table>
      <thead><tr><th>Создана</th><th>Причина</th><th>Клиентов</th><th>Размер</th><th>Состояние</th><th></th></tr></thead>
      <tbody>
        {#each status?.backups ?? [] as backup (backup.id)}
          <tr>
            <td>{dateTime(backup.created_at)}</td>
            <td class="small">{reasons[backup.reason] ?? backup.reason}</td>
            <td>{backup.password_count}</td>
            <td class="small">{bytes(backup.size_bytes)}</td>
            <td>
              <span class="badge" class:ok={backup.valid} class:bad={!backup.valid} title={backup.error ?? ''}>
                {backup.valid ? 'Цела' : 'Повреждена'}
              </span>
            </td>
            <td class="actions">
              <button class="ghost" onclick={() => verify(backup.id)}>Проверить</button>
              <button class="ghost" onclick={() => exportBackup(backup.id)}>Скачать</button>
              <button class="ghost danger" onclick={() => remove(backup.id)}>Удалить</button>
            </td>
          </tr>
        {:else}
          <tr><td colspan="6" class="muted">{loading ? 'Загрузка…' : 'Копий нет'}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<style>
  .actions { text-align: right; white-space: nowrap; }
  .actions button { padding: 3px 7px; font-size: 12.5px; }
</style>
