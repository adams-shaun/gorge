/**
 * hotkeys is the pure keyboard grammar behind the table's document-level
 * hotkeys (prio3): Space passes once, Enter arms End Turn, Shift+Enter arms
 * the hard skip, Escape cancels a live run, Ctrl+Shift+F toggles the
 * full-control preset. It is pure TypeScript — no DOM, no Svelte — so the
 * mapping and its guards are testable without a browser: the caller passes
 * the event-shaped object and a picker-open probe.
 *
 * The guards, in order:
 *  - a held Meta (Cmd) key: never a gorge hotkey — the OS/browser owns those;
 *  - an open modal picker (the radial card-action picker, portaled to body):
 *    its own keys govern, and a hotkey firing underneath it would act on a
 *    decision the player cannot currently see;
 *  - Escape is deliberately NOT guarded by the focus check: it is the panic
 *    key, and it must cancel a run wherever focus happens to be;
 *  - focus in an interactive element (button, link, input, textarea, select,
 *    contenteditable) before every other key: the element's own activation
 *    owns Space and Enter, firing the hotkey underneath would act twice, and
 *    Ctrl+Shift+F while TYPING is the find bar's grammar, never ours. This is
 *    one guard wider than the brief's input list on purpose: Space with focus
 *    on the PASS button would otherwise both activate the button and pass,
 *    posting the same intent twice;
 *  - Ctrl+Shift+F before the plain-Ctrl check: Ctrl held ALONE is the
 *    hold-priority modifier for clicks (it is never a hotkey by itself —
 *    Ctrl+Shift alone is too easy to hit, hence the F).
 */

export type HotkeyAction = 'pass' | 'end-turn' | 'hard-skip' | 'cancel-run' | 'toggle-full-control';

/** HotkeyEvent is the slice of KeyboardEvent the grammar reads, so tests can build it by hand. */
export interface HotkeyEvent {
  key: string;
  ctrlKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
  /** target is the event's original target; checked for interactive elements. */
  target?: EventTarget | null;
}

export function hotkeyAction(
  e: HotkeyEvent,
  pickerOpen: () => boolean = () => false,
): HotkeyAction | null {
  if (e.metaKey) return null;
  if (pickerOpen()) return null;
  if (e.key === 'Escape') return 'cancel-run';
  const t = e.target as { closest?: (sel: string) => unknown } | null | undefined;
  if (t && typeof t.closest === 'function' && t.closest('button, a, input, textarea, select, [contenteditable]')) return null;
  if (e.ctrlKey && e.shiftKey && (e.key === 'f' || e.key === 'F')) return 'toggle-full-control';
  if (e.ctrlKey) return null;
  if (e.key === ' ') return 'pass';
  if (e.key === 'Enter') return e.shiftKey ? 'hard-skip' : 'end-turn';
  return null;
}
