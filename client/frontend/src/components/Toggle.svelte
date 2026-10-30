<script lang="ts">
  let { checked = $bindable(), label, hint = '', disabled = false, onchange }: {
    checked: boolean;
    label: string;
    hint?: string;
    disabled?: boolean;
    onchange?: (value: boolean) => void;
  } = $props();
</script>

<label class="toggle" class:disabled>
  <input type="checkbox" bind:checked {disabled} onchange={() => onchange?.(checked)} />
  <span class="track"><span class="thumb"></span></span>
  <span class="text">
    {label}
    {#if hint}<span class="muted small">{hint}</span>{/if}
  </span>
</label>

<style>
  .toggle { display: flex; gap: 10px; align-items: flex-start; cursor: pointer; }
  .toggle.disabled { opacity: 0.5; cursor: default; }
  input { position: absolute; opacity: 0; pointer-events: none; }
  .track {
    flex: none;
    width: 34px;
    height: 20px;
    margin-top: 1px;
    border-radius: 10px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    position: relative;
    transition: background 0.15s;
  }
  .thumb {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--muted);
    transition: transform 0.15s, background 0.15s;
  }
  input:checked + .track { background: var(--accent); border-color: var(--accent); }
  input:checked + .track .thumb { transform: translateX(14px); background: #fff; }
  .text { display: flex; flex-direction: column; }
</style>
