<script lang="ts" generics="T extends string">
  let { value = $bindable(), options, onchange }: {
    value: T;
    options: { value: T; label: string }[];
    onchange?: (value: T) => void;
  } = $props();
</script>

<div class="segmented" role="radiogroup">
  {#each options as option (option.value)}
    <button
      role="radio"
      aria-checked={value === option.value}
      class:active={value === option.value}
      onclick={() => {
        value = option.value;
        onchange?.(option.value);
      }}>{option.label}</button
    >
  {/each}
</div>

<style>
  .segmented {
    display: inline-flex;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 9px;
    padding: 2px;
    gap: 2px;
  }
  button { border: none; background: transparent; padding: 6px 12px; border-radius: 7px; }
  button.active { background: var(--accent); color: var(--accent-text); }
</style>
