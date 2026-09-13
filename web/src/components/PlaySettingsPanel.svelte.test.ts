import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import { SeatPanelState } from '../lib/seatpanel.svelte';
import type { StoppableStep } from '../lib/playsettings';
import PlaySettingsPanel, { nextStop, presetPatch, stopPatch, stopWord } from './PlaySettingsPanel.svelte';

// SSR via svelte/server, the repo's component-test pattern. There is no DOM
// here, so an interaction is exercised the way the app actually performs it:
// the panel's click handlers call SeatPanelState's write paths (editSettings
// / setAuto / setActPass) through the module-script helpers the handlers
// themselves use (presetPatch, stopPatch, nextStop), and the next render is
// the component's answer to the new settings. The edit under test is the
// real one, not a stand-in.

const ctx = { seat: 0, token: 'tok' };

/** panel renders the editor bound to a fresh (or caller-prepared) seat state. */
function panel(state: SeatPanelState): string {
  return render(PlaySettingsPanel, { props: { state } }).html;
}

/** tag returns the opening tag of the element carrying one data attribute ('' when absent). */
function tag(html: string, attr: string): string {
  return new RegExp(`<([a-z]+)[^>]*${attr.replace(/"/g, '\\"')}[^>]*>`).exec(html)?.[0] ?? '';
}

/** elem returns one element with its content, matched by a data attribute ('' when absent). */
function elem(html: string, attr: string): string {
  const m = new RegExp(`<([a-z]+)[^>]*${attr.replace(/"/g, '\\"')}[^>]*>([\\s\\S]*?)</\\1>`).exec(html);
  return m === null ? '' : m[0];
}

/** cell returns the opening tag of one step-stop cell ('' when absent). */
function cell(html: string, step: string, side: 'yours' | 'opponents'): string {
  return tag(html, `data-step-cell="${step}:${side}"`);
}

/** selectHtml returns the whole <select> element with one data-select value. */
function selectHtml(html: string, which: string): string {
  const m = new RegExp(`<select[^>]*data-select="${which}"[^>]*>[\\s\\S]*?</select>`).exec(html);
  if (m === null) throw new Error(`no select for ${which}`);
  return m[0];
}

describe('PlaySettingsPanel — the GAME OPTIONS editor', () => {
  it('renders casual by default: preset pressed, its blurb shown, the opponent rules reflecting it', () => {
    const html = panel(new SeatPanelState('t1', 1, ctx, null));
    expect(tag(html, 'data-preset="casual"')).toContain('aria-pressed="true"');
    expect(html).not.toContain('data-preset="custom"');
    expect(elem(html, 'data-preset-blurb')).toContain('Stops only when you can do something');
    // casual's trigger rule is the targeting one; the spell/ability rules are
    // the respondable ones. SSR marks the selected option with `selected`.
    expect(selectHtml(html, 'opponent-trigger')).toContain('value="targets-me-if-respondable" selected');
    expect(selectHtml(html, 'opponent-spell')).toContain('value="if-respondable" selected');
  });

  it('clicking a preset applies it through withChange and shows its blurb', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    // The exact call the Full control segment's onclick makes.
    state.editSettings(presetPatch('full-control'));
    const html = panel(state);
    expect(tag(html, 'data-preset="full-control"')).toContain('aria-pressed="true"');
    expect(elem(html, 'data-preset-blurb')).toContain('Stops at every priority window');
    // Full control turns auto off and forces every cell.
    expect(tag(html, 'data-toggle="auto-pass"')).toContain('aria-checked="false"');
    expect([...html.matchAll(/data-stop-value="forced"/g)]).toHaveLength(20);
    // No tells likewise: every opponent rule becomes Always, blurb follows.
    state.editSettings(presetPatch('no-tells'));
    const tells = panel(state);
    expect(tag(tells, 'data-preset="no-tells"')).toContain('aria-pressed="true"');
    expect(elem(tells, 'data-preset-blurb')).toContain('Also stops for every opponent spell and ability');
    expect(selectHtml(tells, 'opponent-spell')).toContain('value="always" selected');
  });

  it('editing one cell flips the preset to Custom, and undoing the edit flips it back', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    // The exact call the cell's onclick makes (cycleCell's first step).
    state.editSettings(stopPatch('main1', 'yours', nextStop(state.settings.steps.yours['main1' as StoppableStep])));
    const custom = panel(state);
    expect(custom).toContain('data-preset="custom"');
    // Custom is a non-clickable segment: a span, not a button.
    expect(custom).toMatch(/<span[^>]*data-preset="custom"[^>]*>Custom<\/span>/);
    expect([...custom.matchAll(/data-preset="custom"/g)]).toHaveLength(1);
    // No preset button stays pressed.
    for (const id of ['casual', 'no-tells', 'full-control']) {
      expect(tag(custom, `data-preset="${id}"`)).not.toContain('aria-pressed="true"');
    }
    // Undoing the edit (back to casual's value) restores the label.
    state.editSettings(stopPatch('main1', 'yours', 'smart'));
    const restored = panel(state);
    expect(tag(restored, 'data-preset="casual"')).toContain('aria-pressed="true"');
    expect(restored).not.toContain('data-preset="custom"');
  });

  it('a cell cycles off → smart → forced → off, with word, glyph and aria-label at each state', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    const step: StoppableStep = 'upkeep'; // off on both sides in casual, so the cycle starts clean
    const cycle = (side: 'yours' | 'opponents'): string => {
      // The exact call the cell's onclick makes.
      const cur = state.settings.steps[side][step] ?? 'off';
      state.editSettings(stopPatch(step, side, nextStop(cur)));
      return panel(state);
    };
    const smart = cycle('yours');
    expect(tag(smart, 'data-step-cell="upkeep:yours"')).toContain('data-stop-value="smart"');
    expect(cell(smart, 'upkeep', 'yours')).toContain('aria-label="Upkeep, my turn: smart"');
    expect(elem(smart, 'data-step-cell="upkeep:yours"')).toContain('Smart');
    expect(elem(smart, 'data-step-cell="upkeep:yours"')).toContain('◐');
    const forced = cycle('yours');
    expect(tag(forced, 'data-step-cell="upkeep:yours"')).toContain('data-stop-value="forced"');
    expect(cell(forced, 'upkeep', 'yours')).toContain('aria-label="Upkeep, my turn: always"');
    expect(elem(forced, 'data-step-cell="upkeep:yours"')).toContain('Always');
    expect(elem(forced, 'data-step-cell="upkeep:yours"')).toContain('●');
    const off = cycle('yours');
    expect(tag(off, 'data-step-cell="upkeep:yours"')).toContain('data-stop-value="off"');
    expect(cell(off, 'upkeep', 'yours')).toContain('aria-label="Upkeep, my turn: off"');
    // The sides stay independent: the opponents cell never moved.
    expect(cell(smart, 'upkeep', 'opponents')).toContain('data-stop-value="off"');
    expect(cell(forced, 'upkeep', 'opponents')).toContain('data-stop-value="off"');
  });

  it('Reset to Casual returns a heavily edited configuration to the casual preset', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.editSettings({ opponentSpell: 'always', logAutoPasses: false, pacing: { stepMs: 100, resolveMs: 200 } });
    state.editSettings(stopPatch('upkeep', 'opponents', 'forced'));
    expect(state.settings.preset).toBe('custom');
    // The exact call the Reset button's onclick makes.
    state.editSettings(presetPatch('casual'));
    expect(state.settings.preset).toBe('casual');
    const html = panel(state);
    expect(tag(html, 'data-preset="casual"')).toContain('aria-pressed="true"');
    expect(html).not.toContain('data-preset="custom"');
    for (const [key, value] of Object.entries(presetPatch('casual'))) {
      expect((state.settings as unknown as Record<string, unknown>)[key]).toEqual(value);
    }
  });

  it('carries the machine-readable affordances: cell aria-labels, switch states, the Smart legend', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.editSettings(stopPatch('end', 'opponents', 'smart'));
    const html = panel(state);
    // One labelled cell per stoppable step per side.
    expect([...html.matchAll(/data-step-cell="[^"]+"/g)]).toHaveLength(20);
    expect(cell(html, 'end', 'opponents')).toContain('aria-label="End step, opponent’s turn: smart"');
    expect(cell(html, 'upkeep', 'yours')).toContain('aria-label="Upkeep, my turn: off"');
    // Switches are real switches, checked by the settings.
    expect(tag(html, 'data-toggle="auto-pass"')).toContain('aria-checked="true"');
    expect(tag(html, 'data-actpass-toggle')).toContain('aria-checked="true"');
    expect(tag(html, 'data-toggle="auto-order-triggers"')).toContain('aria-checked="true"');
    expect(tag(html, 'data-toggle="log-auto-passes"')).toContain('aria-checked="true"');
    // The plain-word legend for Smart.
    expect(html).toContain('Smart = only if I have a play');
    // The plain-word option labels the brief names.
    for (const label of ['If I can respond', 'Always', 'Never', 'If it targets me or my stuff and I can respond', "Don't stop", 'Stop if I can respond']) {
      expect(html).toContain(label);
    }
    // Reset is a plain button with its plain name.
    expect(elem(html, 'data-reset-settings')).toContain('Reset to Casual');
  });

  it('the Auto pass switch goes through setAuto (the runaway brake clears), not a raw patch', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    // Trip the brake the way the loop guard does, then perform the exact
    // call the Auto pass switch's onclick makes.
    state.suspendAuto('cap');
    expect(state.machinePaused).toBe(true);
    state.setAuto(!state.settings.autoPass);
    expect(state.machinePaused).toBe(false);
    expect(state.settings.autoPass).toBe(false);
    expect(state.settings.preset).toBe('custom');
    const html = panel(state);
    expect(tag(html, 'data-toggle="auto-pass"')).toContain('aria-checked="false"');
  });

  it('the own-objects segments carry both rules and write through editSettings', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    const html = panel(state);
    expect(tag(html, 'data-own="never"')).toContain('aria-pressed="true"');
    expect(tag(html, 'data-own="if-respondable"')).not.toContain('aria-pressed="true"');
    // The exact call the segment's onclick makes.
    state.editSettings({ ownObjects: 'if-respondable' });
    const next = panel(state);
    expect(tag(next, 'data-own="if-respondable"')).toContain('aria-pressed="true"');
  });

  it('the pacing picker matches an exact (step/resolve) pair and writes through editSettings', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    // casual is 200/400 = Normal.
    expect(tag(panel(state), 'data-pacing="normal"')).toContain('aria-pressed="true"');
    // The exact call the Short segment's onclick makes.
    state.editSettings({ pacing: { stepMs: 100, resolveMs: 200 } });
    expect(tag(panel(state), 'data-pacing="short"')).toContain('aria-pressed="true"');
    // An unmatched pair presses nothing (no Off/Short/Normal).
    state.editSettings({ pacing: { stepMs: 37, resolveMs: 91 } });
    const html = panel(state);
    for (const id of ['off', 'short', 'normal']) {
      expect(tag(html, `data-pacing="${id}"`)).not.toContain('aria-pressed="true"');
    }
  });
});

describe('PlaySettingsPanel helpers', () => {
  it('nextStop is the documented three-state cycle', () => {
    expect(nextStop('off')).toBe('smart');
    expect(nextStop('smart')).toBe('forced');
    expect(nextStop('forced')).toBe('off');
  });

  it('stopWord names every state, including the forced one the grid calls Always', () => {
    expect(stopWord('off')).toBe('Off');
    expect(stopWord('smart')).toBe('Smart');
    expect(stopWord('forced')).toBe('Always');
  });

  it('presetPatch covers every settings field, so withChange relabels the preset itself', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    for (const id of ['casual', 'no-tells', 'full-control'] as const) {
      state.editSettings(presetPatch(id));
      expect(state.settings.preset).toBe(id);
      const patch = presetPatch(id) as Record<string, unknown>;
      const live = state.settings as unknown as Record<string, unknown>;
      for (const key of Object.keys(patch)) {
        expect(live[key]).toEqual(patch[key]);
      }
    }
  });
});
