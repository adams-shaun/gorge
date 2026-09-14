import { chromium, type Browser, type Page } from 'playwright';
import { createServer, type ViteDevServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import { SeatPanelState } from '../lib/seatpanel.svelte';
import { presetPatch, type StoppableStep } from '../lib/playsettings';
import PlaySettingsPanel, { nextStop, stopPatch, stopWord } from './PlaySettingsPanel.svelte';

// Two layers, because the editor has two halves to defend:
//
//  1. SSR (svelte/server, the repo's render-only component-test pattern)
//     pins what the editor SHOWS for a given settings state — labels, blurbs,
//     aria affordances, the Custom segment, the grid's words/glyphs.
//
//  2. The playwright fixture below (PileModal's pattern) mounts the panel
//     against a REAL SeatPanelState in a REAL browser and CLICKS the actual
//     controls, so a disconnected or broken onclick fails: the cell cycle,
//     the preset picker, the Reset button and the paused-to-Casual brake
//     regression are exercised through the component's own handlers.

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

describe('PlaySettingsPanel — Clear yields (prio6)', () => {
  // One DISTINCT table id per test: the yield store's in-memory layer is
  // module-level and shared by every SeatPanelState this file constructs.
  it('is disabled with nothing yielded, showing None', () => {
    const html = panel(new SeatPanelState('yt-none', 1, ctx, null));
    expect(html).toContain('data-clear-yields');
    expect(tag(html, 'data-clear-yields')).toContain('disabled');
    expect(elem(html, 'data-clear-yields')).toContain('None');
  });

  it('is enabled with yields, and the count says how many the clear would drop', () => {
    const state = new SeatPanelState('yt-two', 1, ctx, null);
    state.addYield('1:Blood Artist:drain');
    state.addYield('2:Soul Warden:gain');
    const html = panel(state);
    expect(tag(html, 'data-clear-yields')).not.toContain('disabled');
    expect(elem(html, 'data-clear-yields')).toContain('Clear yields (2)');
  });
});

describe('PlaySettingsPanel — the GAME OPTIONS editor (rendered)', () => {
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

  it('a cell cycles off → smart → forced → off, with word, glyph and aria-label at each state', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    const step: StoppableStep = 'upkeep'; // off on both sides in casual, so the cycle starts clean
    const cycle = (side: 'yours' | 'opponents'): string => {
      // cycleCell's body: the next rule on the current one, through editSettings.
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

  it('editing one cell flips the preset to Custom, and undoing the edit flips it back', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
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

  it('Reset to Casual returns a heavily edited configuration to the casual preset', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.editSettings({ opponentSpell: 'always', logAutoPasses: false, pacing: { stepMs: 100, resolveMs: 200 } });
    state.editSettings(stopPatch('upkeep', 'opponents', 'forced'));
    expect(state.settings.preset).toBe('custom');
    // The Reset button's onclick: applyPreset('casual') → state.applyNamedPreset.
    state.applyNamedPreset('casual');
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

  it('the Auto pass switch goes through pressAuto (the brake clears, auto REARMS), not a raw patch', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    // Trip the brake the way the loop guard does, then perform the exact
    // call the Auto pass switch's onclick makes.
    state.suspendAuto('cap');
    expect(state.machinePaused).toBe(true);
    state.pressAuto();
    expect(state.machinePaused).toBe(false);
    // The brake's own note promises "Press the Auto switch to rearm it" —
    // which the old setAuto(!autoPass) onclick BROKE for an auto-on player
    // (pressing turned auto off). pressAuto calls setAuto(true): the brake
    // lifts and auto is rearmed, exactly what the note says.
    expect(state.settings.autoPass).toBe(true);
    const html = panel(state);
    expect(tag(html, 'data-toggle="auto-pass"')).toContain('aria-checked="true"');
  });

  it('while the undo pause holds, the Auto pass switch reads Paused (fb-20260914T063523Z)', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.rewind();
    const html = panel(state);
    // The machine is paused but the preference is untouched: the switch
    // shows its paused state, not Off.
    expect(elem(html, 'data-toggle="auto-pass"')).toContain('Paused');
    expect(tag(html, 'data-toggle="auto-pass"')).toContain('aria-checked="true"');
  });

  it('applyNamedPreset re-arms a tripped runaway brake when the preset runs auto (paused-to-Casual)', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    // The brake tripped; the panel still reads auto-pass on (autoPass was
    // never flipped) but considerAuto keeps refusing to act.
    state.suspendAuto('cap');
    expect(state.machinePaused).toBe(true);
    expect(state.settings.autoPass).toBe(true);
    // The exact call the Casual segment's / Reset button's onclick makes.
    state.applyNamedPreset('casual');
    expect(state.machinePaused).toBe(false);
    expect(state.settings.autoPass).toBe(true);
    expect(state.settings.preset).toBe('casual');
    // The machine note is the armed one, as after setAuto(true).
    expect(state.note.kind).toBe('armed');
  });

  it('applyNamedPreset on full-control leaves the machine off, as setAuto(false) would', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.applyNamedPreset('full-control');
    expect(state.settings.autoPass).toBe(false);
    expect(state.settings.preset).toBe('full-control');
    expect(state.machinePaused).toBe(false);
    expect(state.note.kind).toBe('off');
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

describe('PlaySettingsPanel — real clicks in a real browser (PlaySettingsPanel.fixture.html)', () => {
  let server: ViteDevServer;
  let browser: Browser;
  let url = '';

  beforeAll(async () => {
    server = await createServer({ root: process.cwd(), configLoader: 'runner', server: { port: 0 } });
    await server.listen();
    url = server.resolvedUrls!.local[0];
    browser = await chromium.launch();
  });

  afterAll(async () => {
    await browser?.close();
    await server?.close();
  });

  /** open mounts the fixture page (a fresh SeatPanelState at casual, null storage). */
  async function open(): Promise<Page> {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PlaySettingsPanel.fixture.html`);
    return page;
  }

  /** stateOf reads the fixture's live SeatPanelState through the page. */
  async function stateOf(page: Page): Promise<{ paused: boolean; autoPass: boolean; preset: string }> {
    return page.evaluate(() => {
      const s = (window as unknown as { playSettingsState: { machinePaused: boolean; settings: { autoPass: boolean; preset: string } } }).playSettingsState;
      return { paused: s.machinePaused, autoPass: s.settings.autoPass, preset: s.settings.preset };
    });
  }

  it('clicking a cell cycles it off → smart → forced → off through the component handler', async () => {
    const page = await open();
    const cell = page.locator('[data-step-cell="upkeep:yours"]');
    expect(await cell.getAttribute('data-stop-value')).toBe('off');
    expect(await cell.getAttribute('aria-label')).toBe('Upkeep, my turn: off');

    await cell.click();
    expect(await cell.getAttribute('data-stop-value')).toBe('smart');
    expect(await cell.getAttribute('aria-label')).toBe('Upkeep, my turn: smart');
    expect(await cell.textContent()).toContain('Smart');

    await cell.click();
    expect(await cell.getAttribute('data-stop-value')).toBe('forced');
    expect(await cell.getAttribute('aria-label')).toBe('Upkeep, my turn: always');
    expect(await cell.textContent()).toContain('Always');

    await cell.click();
    expect(await cell.getAttribute('data-stop-value')).toBe('off');
    expect(await cell.getAttribute('aria-label')).toBe('Upkeep, my turn: off');
    // The opponents cell never moved.
    expect(await page.locator('[data-step-cell="upkeep:opponents"]').getAttribute('data-stop-value')).toBe('off');
    await page.close();
  });

  it('editing a cell with a real click flips the preset picker to Custom (and Enter works from the keyboard)', async () => {
    const page = await open();
    // Keyboard path first: a native button, focused, activated with Enter.
    const cell = page.locator('[data-step-cell="upkeep:yours"]');
    await cell.focus();
    await page.keyboard.press('Enter');
    expect(await cell.getAttribute('data-stop-value')).toBe('smart');
    // The picker now shows the non-clickable Custom segment and no pressed preset.
    expect(await page.locator('span[data-preset="custom"]').textContent()).toBe('Custom');
    expect(await page.locator('button[data-preset="casual"]').getAttribute('aria-pressed')).toBe('false');
    await page.close();
  });

  it('clicking the Full control segment applies the preset through the state write path', async () => {
    const page = await open();
    await page.locator('[data-preset="full-control"]').click();
    expect(await page.locator('[data-preset="full-control"]').getAttribute('aria-pressed')).toBe('true');
    expect(await page.locator('[data-preset-blurb]').textContent()).toContain('Stops at every priority window');
    expect(await page.locator('[data-toggle="auto-pass"]').getAttribute('aria-checked')).toBe('false');
    expect(await page.locator('[data-stop-value="forced"]').count()).toBe(20);
    const st = await stateOf(page);
    expect(st).toEqual({ paused: false, autoPass: false, preset: 'full-control' });
    await page.close();
  });

  it('choosing Casual after the runaway brake tripped re-arms auto (the machine actually resumes)', async () => {
    const page = await open();
    // Trip the brake the way the loop guard does. Before the fix the preset
    // click left machinePaused set: the panel read Casual/auto-pass while
    // considerAuto kept refusing to act.
    await page.evaluate(() => {
      (window as unknown as { playSettingsState: { suspendAuto: (r: string) => void } }).playSettingsState.suspendAuto('cap');
    });
    expect((await stateOf(page)).paused).toBe(true);
    await page.locator('[data-preset="casual"]').click();
    expect(await stateOf(page)).toEqual({ paused: false, autoPass: true, preset: 'casual' });
    expect(await page.locator('[data-toggle="auto-pass"]').getAttribute('aria-checked')).toBe('true');
    await page.close();
  });

  it('pressing the real Auto pass switch while the UNDO pause holds resumes the machine and preserves the enabled preference (r2 finding)', async () => {
    const page = await open();
    // Rewind the fixture's real state the way the stream's rewind frame
    // does: the machine pauses, the persisted preference stays on.
    await page.evaluate(() => {
      (window as unknown as { playSettingsState: { rewind: () => void } }).playSettingsState.rewind();
    });
    expect(await stateOf(page)).toEqual({ paused: true, autoPass: true, preset: 'casual' });
    // The switch itself reads Paused while the machine is paused.
    expect(await page.locator('[data-toggle="auto-pass"]').textContent()).toContain('Paused');
    // The r2 break: the old onclick was setAuto(!autoPass), which for the
    // default auto-ON player DISABLED auto, cleared the brake and left the
    // empty-window floor to instantly pass the restored window. The
    // pause-aware switch must resume instead — real click, real handler.
    await page.locator('[data-toggle="auto-pass"]').click();
    expect(await stateOf(page)).toEqual({ paused: false, autoPass: true, preset: 'casual' });
    expect(await page.locator('[data-toggle="auto-pass"]').getAttribute('aria-checked')).toBe('true');
    await page.close();
  });

  it('Reset to Casual returns a heavily edited panel to the casual preset', async () => {
    const page = await open();
    // Heavily edit by real clicks: two cells and the Short pacing segment.
    await page.locator('[data-step-cell="upkeep:yours"]').click();
    await page.locator('[data-step-cell="combat-damage:opponents"]').click();
    await page.locator('[data-step-cell="combat-damage:opponents"]').click(); // → forced
    await page.locator('[data-pacing="short"]').click();
    expect((await stateOf(page)).preset).toBe('custom');
    expect(await page.locator('[data-step-cell="combat-damage:opponents"]').getAttribute('data-stop-value')).toBe('forced');

    await page.locator('[data-reset-settings]').click();
    expect((await stateOf(page)).preset).toBe('casual');
    expect(await page.locator('[data-step-cell="upkeep:yours"]').getAttribute('data-stop-value')).toBe('off');
    expect(await page.locator('[data-step-cell="combat-damage:opponents"]').getAttribute('data-stop-value')).toBe('off');
    expect(await page.locator('[data-pacing="normal"]').getAttribute('aria-pressed')).toBe('true');
    expect(await page.locator('button[data-preset="casual"]').getAttribute('aria-pressed')).toBe('true');
    await page.close();
  });
});
