import { describe, expect, it } from 'vitest';
import { hotkeyAction, MODAL_PICKER_SELECTOR, type HotkeyEvent } from './hotkeys';

/** A non-interactive target: the felt, the panel text, the body. */
const felt = { closest: () => null } as unknown as EventTarget;
/** An interactive target: closest() matches, as Element.closest does inside a button/input/textarea. */
const inWidget = { closest: (sel: string) => (sel.includes('button') ? {} : null) } as unknown as Element;

const ev = (over: Partial<HotkeyEvent>): HotkeyEvent => ({
  key: ' ', ctrlKey: false, shiftKey: false, metaKey: false, target: felt, ...over,
});

describe('hotkeys — the grammar', () => {
  it('Space passes, Enter arms End Turn, Shift+Enter arms the hard skip', () => {
    expect(hotkeyAction(ev({ key: ' ' }))).toBe('pass');
    expect(hotkeyAction(ev({ key: 'Enter' }))).toBe('end-turn');
    expect(hotkeyAction(ev({ key: 'Enter', shiftKey: true }))).toBe('hard-skip');
  });

  it('Escape cancels the run — even with focus in an input (the panic key)', () => {
    expect(hotkeyAction(ev({ key: 'Escape' }))).toBe('cancel-run');
    expect(hotkeyAction(ev({ key: 'Escape', target: inWidget }))).toBe('cancel-run');
  });

  it('Space and Enter are ignored inside a textarea / input / button / contenteditable — and Ctrl+Shift+F too', () => {
    for (const key of [' ', 'Enter']) {
      expect(hotkeyAction(ev({ key, target: inWidget }))).toBeNull();
    }
    // typing Ctrl+Shift+F in a text field is a find bar, not a preset toggle
    expect(hotkeyAction(ev({ key: 'f', ctrlKey: true, shiftKey: true, target: inWidget }))).toBeNull();
  });

  it('Ctrl+Shift+F toggles the preset; Ctrl alone, Ctrl+Shift alone and Meta+anything are never hotkeys', () => {
    expect(hotkeyAction(ev({ key: 'f', ctrlKey: true, shiftKey: true }))).toBe('toggle-full-control');
    expect(hotkeyAction(ev({ key: 'F', ctrlKey: true, shiftKey: true }))).toBe('toggle-full-control');
    // Ctrl held alone is the hold-priority click modifier, never a hotkey.
    expect(hotkeyAction(ev({ key: 'Enter', ctrlKey: true }))).toBeNull();
    expect(hotkeyAction(ev({ key: 'Enter', shiftKey: true, ctrlKey: true }))).toBeNull();
    // Ctrl+Shift alone (no F) is too easy to hit — never a hotkey.
    expect(hotkeyAction(ev({ key: 'Enter', ctrlKey: true, shiftKey: true }))).toBeNull();
    expect(hotkeyAction(ev({ key: ' ', metaKey: true }))).toBeNull();
  });

  it('an open modal picker swallows every hotkey, Escape included', () => {
    for (const key of [' ', 'Enter', 'Escape', 'f']) {
      expect(hotkeyAction(ev({ key }), () => true)).toBeNull();
    }
  });

  // The pure grammar test above probes with a synthetic predicate; this pins
  // the real selector's two markers so a surface that loses its marker is
  // caught here too. BOTH OptionPicker shapes carry the shared first marker;
  // HotkeyGuard.test.ts mounts and proves the radial and >6 list shapes.
  it('MODAL_PICKER_SELECTOR names every OptionPicker shape and the pile modal', () => {
    expect(MODAL_PICKER_SELECTOR).toContain('[data-option-picker]');
    expect(MODAL_PICKER_SELECTOR).toContain('[data-pile-modal]');
  });

  it('ordinary typing is never a hotkey', () => {
    for (const key of ['a', 'ArrowLeft', 'Tab', 'Shift', 'F1']) {
      expect(hotkeyAction(ev({ key }))).toBeNull();
    }
  });
});
