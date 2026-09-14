<script module lang="ts">
  import { type PlaySettings, type PresetName, type StepStop, type StoppableStep } from '../lib/playsettings';
  import type { TurnSide } from '../lib/autopilot';

  /**
   * PRESET_LIST is the three clickable presets in picker order, each with
   * its one-line plain description (the blurb shown under the picker).
   * 'custom' is deliberately absent from the clickable list: it is rendered
   * as a non-clickable segment only while the settings carry it.
   */
  export const PRESET_LIST: { id: PresetName; label: string; blurb: string }[] = [
    {
      id: 'casual',
      label: 'Casual',
      blurb: 'Stops only when you can do something: your main phases, opponent attacks and end step, and opponent spells you can answer.',
    },
    {
      id: 'no-tells',
      label: 'No tells',
      blurb: "Also stops for every opponent spell and ability, so your pauses don't reveal your hand.",
    },
    {
      id: 'full-control',
      label: 'Full control',
      blurb: 'Stops at every priority window.',
    },
  ];

  /** nextStop is one step-stop cell's three-state cycle: Off → Smart → Always → Off. */
  export function nextStop(rule: StepStop): StepStop {
    return rule === 'off' ? 'smart' : rule === 'smart' ? 'forced' : 'off';
  }

  /** stopPatch is one cell's change as a withChange patch (deep-merged into steps field-wise). */
  export function stopPatch(step: StoppableStep, side: TurnSide, rule: StepStop): Partial<PlaySettings> {
    return { steps: { [side]: { [step]: rule } } } as unknown as Partial<PlaySettings>;
  }

  /** stopWord names each cell state in plain words — the state is text, never colour alone. */
  export function stopWord(rule: StepStop): string {
    return rule === 'off' ? 'Off' : rule === 'smart' ? 'Smart' : 'Always';
  }

  /** stopGlyph marks each cell state with a shape beside its word. */
  export function stopGlyph(rule: StepStop): string {
    return rule === 'off' ? '·' : rule === 'smart' ? '◐' : '●';
  }
</script>

<script lang="ts">
  import { STOPPABLE_STEPS } from '../lib/autopilot';
  import { stepFullName } from '../lib/phases';
  import type {
    OpponentObjectRule,
    OpponentTriggerRule,
    OwnObjectRule,
  } from '../lib/playsettings';
  import type { SeatPanelState } from '../lib/seatpanel.svelte';

  /**
   * The GAME OPTIONS editor for the whole play-settings model
   * (lib/playsettings.ts). It is bound to the seat panel's settings object:
   * every change goes through a SeatPanelState write path — the same write
   * path decide() reads: editSettings for the ordinary edits (withChange
   * relabels the preset Custom while the configuration matches none, and
   * back to a named preset when an edit is undone), pressAuto /
   * setActPass for the two switches that carry
   * machine-side consequences (re-arming a tripped runaway brake, disarming
   * an armed pass), and applyNamedPreset for the preset picker and the
   * Reset button, which need the same re-arm effects setAuto carries.
   */
  let { state }: { state: SeatPanelState } = $props();

  const s = $derived(state.settings);
  const STEPPABLE = STOPPABLE_STEPS as readonly StoppableStep[];

  const OBJECT_OPTIONS: { value: OpponentObjectRule; label: string }[] = [
    { value: 'if-respondable', label: 'If I can respond' },
    { value: 'always', label: 'Always' },
    { value: 'never', label: 'Never' },
  ];
  const TRIGGER_OPTIONS: { value: OpponentTriggerRule; label: string }[] = [
    { value: 'targets-me-if-respondable', label: 'If it targets me or my stuff and I can respond' },
    { value: 'if-respondable', label: 'If I can respond' },
    { value: 'always', label: 'Always' },
    { value: 'never', label: 'Never' },
  ];
  const OWN_OPTIONS: { value: OwnObjectRule; label: string }[] = [
    { value: 'never', label: "Don't stop" },
    { value: 'if-respondable', label: 'Stop if I can respond' },
  ];

  /** PACING_OPTIONS is the pause between auto-passes: (step/resolve) ms pairs. */
  const PACING_OPTIONS = [
    { id: 'off', label: 'Off', stepMs: 0, resolveMs: 0 },
    { id: 'short', label: 'Short', stepMs: 100, resolveMs: 200 },
    { id: 'normal', label: 'Normal', stepMs: 200, resolveMs: 400 },
  ] as const;

  const blurb = $derived(PRESET_LIST.find((p) => p.id === s.preset)?.blurb ?? null);
  const pacingId = $derived(
    PACING_OPTIONS.find((p) => p.stepMs === s.pacing.stepMs && p.resolveMs === s.pacing.resolveMs)?.id ?? null,
  );
  const rows = $derived(
    STEPPABLE.map((step) => ({
      step,
      name: stepFullName(step),
      yours: s.steps.yours[step],
      opponents: s.steps.opponents[step],
    })),
  );

  function applyPreset(id: PresetName): void {
    // applyNamedPreset, not editSettings: a named preset that runs auto must
    // also clear the machine's runaway brake, or the panel would read
    // auto-pass on while the loop keeps refusing to act.
    state.applyNamedPreset(id);
  }
  function cycleCell(step: StoppableStep, side: TurnSide): void {
    const cur = s.steps[side][step] ?? 'off';
    state.editSettings(stopPatch(step, side, nextStop(cur)));
  }
  function setOwn(v: OwnObjectRule): void {
    state.editSettings({ ownObjects: v });
  }
</script>

<div class="panel" data-settings-panel>
  <section class="sec">
    <h3>Preset</h3>
    <div class="segments" data-preset-picker>
      {#each PRESET_LIST as p (p.id)}
        <button
          type="button"
          class="seg"
          class:on={s.preset === p.id}
          aria-pressed={s.preset === p.id}
          data-preset={p.id}
          onclick={() => applyPreset(p.id)}
        >{p.label}</button>
      {/each}
      {#if s.preset === 'custom'}
        <span class="seg custom" data-preset="custom">Custom</span>
      {/if}
    </div>
    {#if blurb !== null}
      <p class="blurb" data-preset-blurb>{blurb}</p>
    {/if}
  </section>

  <section class="sec">
    <button
      type="button"
      role="switch"
      class="row"
      class:on={s.autoPass}
      aria-checked={s.autoPass}
      data-toggle="auto-pass"
      onclick={() => state.pressAuto()}
    >
      <span>Auto pass</span><span class="state" aria-hidden="true">{state.machinePaused ? 'Paused' : s.autoPass ? 'On' : 'Off'}</span>
    </button>
  </section>

  <section class="sec">
    <h3>Stop when an opponent…</h3>
    <label class="row sel">
      <span>casts a spell</span>
      <select
        data-select="opponent-spell"
        onchange={(e) => state.editSettings({ opponentSpell: e.currentTarget.value as OpponentObjectRule })}
      >
        {#each OBJECT_OPTIONS as o (o.value)}
          <option value={o.value} selected={s.opponentSpell === o.value}>{o.label}</option>
        {/each}
      </select>
    </label>
    <label class="row sel">
      <span>activates an ability</span>
      <select
        data-select="opponent-ability"
        onchange={(e) => state.editSettings({ opponentAbility: e.currentTarget.value as OpponentObjectRule })}
      >
        {#each OBJECT_OPTIONS as o (o.value)}
          <option value={o.value} selected={s.opponentAbility === o.value}>{o.label}</option>
        {/each}
      </select>
    </label>
    <label class="row sel">
      <span>has a trigger</span>
      <select
        data-select="opponent-trigger"
        onchange={(e) => state.editSettings({ opponentTrigger: e.currentTarget.value as OpponentTriggerRule })}
      >
        {#each TRIGGER_OPTIONS as o (o.value)}
          <option value={o.value} selected={s.opponentTrigger === o.value}>{o.label}</option>
        {/each}
      </select>
    </label>
  </section>

  <section class="sec">
    <h3>My own spells and abilities</h3>
    <div class="segments" data-own-picker>
      {#each OWN_OPTIONS as o (o.value)}
        <button
          type="button"
          class="seg"
          class:on={s.ownObjects === o.value}
          aria-pressed={s.ownObjects === o.value}
          data-own={o.value}
          onclick={() => setOwn(o.value)}
        >{o.label}</button>
      {/each}
    </div>
  </section>

  <section class="sec">
    <h3>Step stops</h3>
    <table class="grid">
      <thead>
        <tr>
          <th scope="col" class="stepcol"><span class="vh">Step</span></th>
          <th scope="col">My turn</th>
          <th scope="col">Opponent’s turn</th>
        </tr>
      </thead>
      <tbody>
        {#each rows as r (r.step)}
          <tr>
            <th scope="row" class="stepcol">{r.name}</th>
            <td>
              <button
                type="button"
                class="cell {r.yours}"
                data-step-cell={`${r.step}:yours`}
                data-stop-value={r.yours}
                aria-label={`${r.name}, my turn: ${stopWord(r.yours).toLowerCase()}`}
                onclick={() => cycleCell(r.step, 'yours')}
              ><span class="glyph" aria-hidden="true">{stopGlyph(r.yours)}</span>{stopWord(r.yours)}</button>
            </td>
            <td>
              <button
                type="button"
                class="cell {r.opponents}"
                data-step-cell={`${r.step}:opponents`}
                data-stop-value={r.opponents}
                aria-label={`${r.name}, opponent’s turn: ${stopWord(r.opponents).toLowerCase()}`}
                onclick={() => cycleCell(r.step, 'opponents')}
              ><span class="glyph" aria-hidden="true">{stopGlyph(r.opponents)}</span>{stopWord(r.opponents)}</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <p class="legend">Smart = only if I have a play. Always stops even at an empty window; Off never stops.</p>
  </section>

  <section class="sec">
    <button
      type="button"
      role="switch"
      class="row"
      class:on={s.passAfterAct}
      aria-checked={s.passAfterAct}
      data-toggle="pass-after-cast"
      data-actpass-toggle
      onclick={() => state.setActPass(!s.passAfterAct)}
    >
      <span>Pass after I cast</span><span class="state" aria-hidden="true">{s.passAfterAct ? 'On' : 'Off'}</span>
    </button>
    <button
      type="button"
      role="switch"
      class="row"
      class:on={s.autoOrderIdenticalTriggers}
      aria-checked={s.autoOrderIdenticalTriggers}
      data-toggle="auto-order-triggers"
      onclick={() => state.editSettings({ autoOrderIdenticalTriggers: !s.autoOrderIdenticalTriggers })}
    >
      <span>Auto-order identical triggers</span><span class="state" aria-hidden="true">{s.autoOrderIdenticalTriggers ? 'On' : 'Off'}</span>
    </button>
    <div class="row sel">
      <span id="pacing-label">Pause between auto-passes</span>
      <div class="segments" role="group" aria-labelledby="pacing-label" data-pacing-picker>
        {#each PACING_OPTIONS as p (p.id)}
          <button
            type="button"
            class="seg"
            class:on={pacingId === p.id}
            aria-pressed={pacingId === p.id}
            data-pacing={p.id}
            onclick={() => state.editSettings({ pacing: { stepMs: p.stepMs, resolveMs: p.resolveMs } })}
          >{p.label}</button>
        {/each}
      </div>
    </div>
    <p class="legend">Short = 100/200 ms, Normal = 200/400 ms between automatic passes (step/resolve).</p>
    <button
      type="button"
      role="switch"
      class="row"
      class:on={s.logAutoPasses}
      aria-checked={s.logAutoPasses}
      data-toggle="log-auto-passes"
      onclick={() => state.editSettings({ logAutoPasses: !s.logAutoPasses })}
    >
      <span>Log auto-passes</span><span class="state" aria-hidden="true">{s.logAutoPasses ? 'On' : 'Off'}</span>
    </button>
    <!-- Clear yields (prio6): the GAME OPTIONS action for the stack tile
         menus' "Always pass for …" set — game-scoped, per table, so this is
         a state write (state.clearYields), not a settings edit. Disabled
         with nothing yielded, and the count says how much it would clear. -->
    <button
      type="button"
      class="row"
      data-clear-yields
      disabled={state.yieldList.length === 0}
      onclick={() => state.clearYields()}
    >
      <span>Clear yields{state.yieldList.length > 0 ? ` (${state.yieldList.length})` : ''}</span>
      <span class="state" aria-hidden="true">{state.yieldList.length > 0 ? 'Clear' : 'None'}</span>
    </button>
    <button type="button" class="reset" data-reset-settings onclick={() => applyPreset('casual')}>Reset to Casual</button>
  </section>
</div>

<style>
  .panel {
    font-family: var(--font-ui);
    font-size: var(--t-12);
  }
  .sec {
    padding: var(--sp-2);
    border-bottom: 1px solid var(--edge-inst);
  }
  .sec:last-child { border-bottom: 0; }
  h3 {
    margin: 0 0 var(--sp-1);
    color: var(--ink-faint);
    font-size: var(--t-10);
    font-weight: 700;
    letter-spacing: 0.02em;
  }
  .segments {
    display: flex;
    gap: 1px;
    max-width: 100%;
    border: 1px solid var(--edge-inst);
    background: var(--edge-inst);
  }
  .seg {
    flex: 1 1 0;
    min-width: 0;
    padding: var(--sp-1) var(--sp-2);
    border: 0;
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-11);
    text-align: center;
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .seg:hover { color: var(--ink); }
  .seg.on {
    background: var(--offered);
    color: var(--felt-sunk);
  }
  .seg.custom {
    cursor: default;
    font-style: italic;
    background: var(--instrument);
    color: var(--ink-dim);
  }
  .blurb {
    margin: var(--sp-1) 0 0;
    color: var(--ink-dim);
    font-size: var(--t-11);
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-2);
    width: 100%;
    padding: var(--sp-2);
    border: 1px solid var(--edge-inst);
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    text-align: left;
    cursor: pointer;
  }
  button.row:hover { color: var(--ink); }
  button.row.on {
    background: var(--offered);
    color: var(--felt-sunk);
  }
  .row:disabled {
    opacity: 0.5;
    cursor: default;
  }
  button.row:disabled:hover { color: var(--ink-inst); }
  button.row + button.row { margin-top: 1px; }
  .row.sel {
    padding: var(--sp-1) 0;
    border: 0;
    background: none;
  }
  .row.sel select {
    flex: 1 1 auto;
    min-width: 0;
    max-width: 62%;
    padding: var(--sp-1);
    border: 1px solid var(--edge-inst);
    background: var(--instrument-raised);
    color: var(--ink);
    font-family: var(--font-ui);
    font-size: var(--t-11);
  }
  table.grid {
    width: 100%;
    border-collapse: collapse;
  }
  .grid th,
  .grid td {
    padding: 2px 0;
    text-align: center;
    font-weight: 400;
  }
  .grid thead th {
    color: var(--ink-faint);
    font-size: var(--t-10);
  }
  .grid .stepcol {
    width: 42%;
    text-align: left;
    color: var(--ink-dim);
    font-size: var(--t-10);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding-right: var(--sp-1);
  }
  .cell {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 3px;
    min-width: 4.4rem;
    max-width: 100%;
    padding: var(--sp-1);
    border: 1px solid var(--edge-inst);
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-10);
    cursor: pointer;
  }
  .cell:hover { color: var(--ink); }
  .cell.smart {
    background: color-mix(in srgb, var(--offered) 28%, var(--instrument-raised));
  }
  .cell.forced {
    background: var(--offered);
    color: var(--felt-sunk);
  }
  .legend {
    margin: var(--sp-1) 0 0;
    color: var(--ink-faint);
    font-size: var(--t-10);
  }
  .reset {
    width: 100%;
    margin-top: var(--sp-2);
    padding: var(--sp-2);
    border: 1px solid var(--edge-inst);
    background: var(--instrument-raised);
    color: var(--ink);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    font-weight: 700;
    cursor: pointer;
  }
  .reset:hover {
    background: var(--offered);
    color: var(--felt-sunk);
  }
  .vh {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
</style>
