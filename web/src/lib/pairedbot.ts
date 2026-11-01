import type { Decision } from '../protocol';
import type { ExplainRow } from './review';
import { safeStorage } from './storage';

/**
 * The paired bot's client side: the player's on/off preference and the labels
 * of its search rows. A player's preference, like the play settings, so one
 * global key rather than one per table. Off until the player turns it on.
 */
const KEY = 'gorge.pairedbot.v1';

export function loadPairedBotOn(storage: Storage | null = safeStorage()): boolean {
  try {
    return storage?.getItem(KEY) === 'on';
  } catch {
    return false;
  }
}

export function savePairedBotOn(on: boolean, storage: Storage | null = safeStorage()): void {
  try {
    storage?.setItem(KEY, on ? 'on' : 'off');
  } catch {
    // A refused write leaves the toggle working for this page.
  }
}

/**
 * pairedRowLabel names a search row from the decision the client is showing:
 * a planned cast by its payment action, an answer by its options' labels.
 * Copies of one card share a label, so a single option also shows its index
 * when another option has the same label.
 */
export function pairedRowLabel(d: Decision, r: ExplainRow): string {
  if (r.payment) return d.payment_actions?.find((a) => a.id === r.payment)?.label ?? r.payment;
  if (!r.choices?.length) return 'Nothing';
  const label = (c: number) => d.options.find((o) => o.index === c)?.label ?? `option ${c}`;
  if (r.choices.length === 1) {
    const l = label(r.choices[0]);
    return d.options.filter((o) => o.label === l).length > 1 ? `${l} (${r.choices[0]})` : l;
  }
  return r.choices.map(label).join('; ');
}
