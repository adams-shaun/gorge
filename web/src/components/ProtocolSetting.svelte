<script lang="ts">
  import { onMount } from 'svelte';
  import { PROTOCOLS, activeWire, protocolLabel, saveProtocol, tableProtocol, wireOffered, type WireProtocol } from '../lib/wire/pref';
  import { loadCompat, saveCompat } from '../lib/manabrew/compat';

  // The client-side "Protocol" setting: which wire a seated table plays over. It is read when a table loads, so inside a
  // table (`active` given) a change offers a reconnect instead of switching
  // the live game's wire underneath the seat panel.
  const active = activeWire.current;
  let chosen = $state<WireProtocol>('native');
  let offered = $state<WireProtocol[]>(['native']);
  let compat = $state(true);
  onMount(() => {
    offered = PROTOCOLS.map((p) => p.value).filter(wireOffered);
    chosen = tableProtocol();
    compat = loadCompat();
  });
  function setCompat(on: boolean) {
    compat = on;
    saveCompat(on);
  }
  function choose(p: WireProtocol) {
    chosen = p;
    saveProtocol(p);
  }
</script>

<div class="protocol" data-protocol-setting>
  <span class="label" id="protocol-label">Protocol</span>
  <div class="segments" role="radiogroup" aria-labelledby="protocol-label">
    {#each PROTOCOLS as p (p.value)}
      <button type="button" role="radio" class="seg" class:on={chosen === p.value} aria-checked={chosen === p.value} data-protocol={p.value} disabled={!offered.includes(p.value)} onclick={() => choose(p.value)}>{p.label}</button>
    {/each}
  </div>
  {#if chosen === 'manabrew'}
    <label class="compat" title="ManaBrew's own engine never gives you priority when all you could do is make mana; this passes those for you. Applies when a table loads.">
      <input type="checkbox" data-mb-compat checked={compat} onchange={(e) => setCompat(e.currentTarget.checked)} />
      Auto-pass when all I could do is make mana
    </label>
  {/if}
  {#if active !== null && active !== chosen && activeWire.outOfScope && chosen === 'sbv2'}
    <p class="note" data-protocol-out-of-scope>This table plays over Native: SpellBench v2 covers two-player constructed tables.</p>
  {:else if active !== null && active !== chosen}
    <p class="note">This table is playing over {protocolLabel(active)}. <button type="button" class="link" data-protocol-reconnect onclick={() => location.reload()}>Reconnect now</button></p>
  {:else}
    <p class="note">Native is gorge's own stream. Applies when a table loads.</p>
  {/if}
  {#if !offered.includes('manabrew')}
    <p class="note" data-protocol-unavailable>ManaBrew needs a server started with <code>-manabrew</code>.</p>
  {/if}
  {#if !offered.includes('sbv2')}
    <p class="note" data-protocol-unavailable>SpellBench v2 needs a server started with <code>-sbv2</code>.</p>
  {/if}
</div>

<style>
  .protocol {
    display: grid;
    gap: var(--sp-1);
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  .label {
    color: var(--ink-dim);
  }
  .segments {
    display: flex;
    gap: 1px;
    border: 1px solid var(--edge-inst);
    background: var(--edge-inst);
    max-width: 22rem;
  }
  .seg {
    flex: 1 1 0;
    padding: var(--sp-1) var(--sp-2);
    border: 0;
    border-radius: 0;
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font: var(--t-11) var(--font-ui);
    cursor: pointer;
  }
  .seg:disabled {
    color: var(--ink-dim);
    cursor: not-allowed;
  }
  .seg.on {
    background: var(--offered);
    color: var(--felt-sunk);
  }
  .compat {
    display: flex;
    align-items: center;
    gap: var(--sp-1);
    color: var(--ink-inst);
    cursor: pointer;
  }
  .note {
    margin: 0;
    font-size: var(--t-11);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--offered);
    text-decoration: underline;
    cursor: pointer;
    font: inherit;
  }
</style>
