import { MODAL_PICKER_SELECTOR } from './hotkeys';

/**
 * modals.ts is the live-DOM half of the hotkey grammar's modal guard
 * (prio3 review r2): the pure grammar in hotkeys.ts takes a `pickerOpen`
 * callback, and this module is what that callback is in the wiring.
 *
 * MODAL_PICKER_SELECTOR (defined beside the grammar so the two cannot
 * drift) matches every modal decision-blocking surface:
 *
 *  - `[data-option-picker]` — either OptionPicker card-action shape (the
 *    radial wheel or the >6-option list), portaled to <body>, present only
 *    while it is open;
 *  - `[data-pile-modal]` — PileModal's portaled backdrop (hand, graveyard
 *    and exile), present only while it is open.
 *
 * A new surface that blocks a pending decision must join this selector or
 * hotkeys aimed underneath it (Space posting PASS, Enter arming a run) will
 * fire while the player cannot see the board. The probe reads the live
 * document, so it is DOM work the pure grammar deliberately does not do.
 */
export function modalPickerOpen(): boolean {
  return typeof document !== 'undefined' && document.querySelector(MODAL_PICKER_SELECTOR) !== null;
}
