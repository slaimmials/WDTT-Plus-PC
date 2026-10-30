<script lang="ts">
  import { store } from '../lib/store.svelte';
  import Modal from './Modal.svelte';

  let value = $state('');

  $effect(() => {
    value = store.dialog?.input?.value ?? '';
  });

  function close(result: string | null) {
    const dialog = store.dialog;
    store.dialog = null;
    dialog?.resolve(result);
  }
</script>

{#if store.dialog}
  {@const dialog = store.dialog}
  <Modal title={dialog.title} onclose={() => close(null)}>
    <div class="stack">
      {#if dialog.message}<p>{dialog.message}</p>{/if}
      {#if dialog.input}
        <label class="field">
          <span>{dialog.input.label}</span>
          <!-- svelte-ignore a11y_autofocus -->
          <input
            type={dialog.input.type ?? 'text'}
            bind:value
            autofocus
            onkeydown={(e) => e.key === 'Enter' && close(String(value))}
          />
        </label>
      {/if}
    </div>
    {#snippet footer()}
      <button onclick={() => close(null)}>Отмена</button>
      <button class={dialog.danger ? 'danger' : 'primary'} onclick={() => close(String(value))}>{dialog.confirmLabel}</button>
    {/snippet}
  </Modal>
{/if}
