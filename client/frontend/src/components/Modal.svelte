<script lang="ts">
  import type { Snippet } from 'svelte';

  let { title, onclose, children, footer, wide = false }: {
    title: string;
    onclose: () => void;
    children: Snippet;
    footer?: Snippet;
    wide?: boolean;
  } = $props();
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dialog card" class:wide role="dialog" aria-modal="true" aria-label={title}>
    <div class="row spread">
      <h2>{title}</h2>
      <button class="ghost" onclick={onclose} aria-label="Закрыть">✕</button>
    </div>
    <div class="body">{@render children()}</div>
    {#if footer}
      <div class="row footer">{@render footer()}</div>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgb(0 0 0 / 0.55);
    display: grid;
    place-items: center;
    z-index: 20;
  }
  .dialog {
    width: min(520px, calc(100vw - 32px));
    max-height: calc(100vh - 48px);
    display: flex;
    flex-direction: column;
  }
  .dialog.wide { width: min(760px, calc(100vw - 32px)); }
  .body { overflow: auto; padding-right: 2px; }
  .footer { justify-content: flex-end; margin-top: 16px; }
</style>
