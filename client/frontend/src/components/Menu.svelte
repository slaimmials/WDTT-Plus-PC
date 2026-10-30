<script lang="ts">
  export type MenuItem = { label: string; action: () => void; danger?: boolean; disabled?: boolean };

  let { items, label = 'Действия', icon = '⋯' }: { items: MenuItem[]; label?: string; icon?: string } = $props();
  let open = $state(false);
  let root: HTMLDivElement | undefined = $state();

  function onWindowClick(event: MouseEvent) {
    if (open && root && !root.contains(event.target as Node)) open = false;
  }
</script>

<svelte:window onclick={onWindowClick} onkeydown={(e) => e.key === 'Escape' && (open = false)} />

<div class="menu" bind:this={root}>
  <button class="ghost trigger" aria-label={label} aria-expanded={open} onclick={() => (open = !open)}>{icon}</button>
  {#if open}
    <div class="popup card" role="menu">
      {#each items as item (item.label)}
        <button
          role="menuitem"
          class="ghost"
          class:danger={item.danger}
          disabled={item.disabled}
          onclick={() => {
            open = false;
            item.action();
          }}>{item.label}</button
        >
      {/each}
    </div>
  {/if}
</div>

<style>
  .menu { position: relative; }
  .trigger { padding: 2px 10px; font-size: 18px; line-height: 1; }
  .popup {
    position: absolute;
    right: 0;
    top: calc(100% + 4px);
    z-index: 10;
    padding: 4px;
    min-width: 180px;
    display: flex;
    flex-direction: column;
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.35);
  }
  .popup button { text-align: left; padding: 7px 10px; }
</style>
