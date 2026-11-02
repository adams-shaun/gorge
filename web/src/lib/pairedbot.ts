import type { Decision } from '../protocol';
import { isManualManaOption } from './manualmana';
import { promptWords } from './prompts/words';
import type { ExplainRow, ExplainSearch } from './review';
import { safeStorage } from './storage';

/**
 * The paired bot's client side: the player's on/off preference, the words of
 * its search rows, and the note for a player who answers differently. The
 * preference is a player's, like the play settings, so one global key rather
 * than one per table. Off until the player turns it on.
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
 * a planned cast by its payment action, an answer by its options' labels. An
 * option a payment action stands for reads as that action, so the two ways of
 * casting one spell share a name, as do two copies of a card in hand. Other
 * same-named options, such as two Mountains to target, also show an index. No
 * options is declining: no attack, no blocks.
 */
export function pairedRowLabel(d: Decision, r: Pick<ExplainRow, 'choices' | 'payment'>): string {
  if (r.payment) return promptWords(d.payment_actions?.find((a) => a.id === r.payment)?.label ?? 'A planned cast');
  if (!r.choices?.length) return d.kind === 'attackers' ? 'No attack' : d.kind === 'blockers' ? 'No blocks' : 'Nothing';
  const label = (c: number) => {
    const action = d.payment_actions?.find((a) => a.base_option_index === c);
    return action?.label ?? d.options.find((o) => o.index === c)?.label ?? `option ${c}`;
  };
  if (r.choices.length === 1) {
    const c = r.choices[0];
    const twin = d.options.some((o) => o.index !== c && o.label === label(c));
    const kind = d.options.find((o) => o.index === c)?.kind;
    const fromHand = kind === 'cast' || kind === 'play_land';
    return promptWords(twin && !fromHand ? `${label(c)} (${c})` : label(c));
  }
  return promptWords([...r.choices].sort((x, y) => x - y).map(label).join('; '));
}

/**
 * winChance reads a row's Q, the deciding seat's value in [-1, 1], as a
 * percentage. An unvisited row has no estimate, only the note's neutral 0.
 */
export function winChance(r: Pick<ExplainRow, 'visits' | 'q'>): number | null {
  return r.visits > 0 ? Math.round((r.q + 1) * 50) : null;
}

/** One line of the panel: every candidate search weighed under one name. */
export interface AdviceRow {
  label: string;
  visits: number;
  /** The visit-weighted estimate as a percentage; null when search never visited it. */
  chance: number | null;
  recommended: boolean;
}

/** adviceRows gathers a search's rows by what a player would call them, most visited first. */
export function adviceRows(d: Decision, search: ExplainSearch): AdviceRow[] {
  const named = new Map<string, { visits: number; value: number; recommended: boolean }>();
  for (const r of search.rows ?? []) {
    const label = pairedRowLabel(d, r);
    const row = named.get(label) ?? { visits: 0, value: 0, recommended: false };
    row.visits += r.visits;
    row.value += r.q * r.visits;
    row.recommended ||= r.chosen === true;
    named.set(label, row);
  }
  return [...named]
    .map(([label, r]) => ({ label, visits: r.visits, chance: winChance({ visits: r.visits, q: r.value / Math.max(1, r.visits) }), recommended: r.recommended }))
    .sort((a, b) => b.visits - a.visits);
}

/** shownRows is the first n rows, the recommendation always among them, and how many that leaves out. */
export function shownRows(rows: readonly AdviceRow[], n: number, dropped = 0): { rows: AdviceRow[]; more: number } {
  const shown = rows.slice(0, n);
  const best = rows.find((r) => r.recommended);
  if (best && !shown.includes(best)) shown[shown.length - 1] = best;
  return { rows: shown, more: rows.length - shown.length + dropped };
}

/** What the player answered a decision with: option indices, or the payment action of a planned cast. */
export interface PlayerAnswer {
  seq: number;
  choices: number[];
  action?: string;
}

const estimate = (r: AdviceRow | undefined) => (r?.chance == null ? 'which search did not weigh' : `estimated ${r.chance}%`);

/**
 * differedNote words an answer that was not the search's recommendation, null
 * for one that was. Tapping a land for mana is a step towards an answer, not
 * one.
 */
export function differedNote(d: Decision, search: ExplainSearch, a: PlayerAnswer): string | null {
  const rows = adviceRows(d, search);
  const best = rows.find((r) => r.recommended);
  const label = pairedRowLabel(d, { choices: a.choices, payment: a.action });
  if (!best || best.label === label) return null;
  if (!a.action && a.choices.length === 1) {
    const o = d.options.find((x) => x.index === a.choices[0]);
    if (o && isManualManaOption(o)) return null;
  }
  return `You differed: ${label}, ${estimate(rows.find((r) => r.label === label))}. Search preferred ${best.label}, ${estimate(best)}.`;
}

/** The player's notes of one game. One key, overwritten by the next game, so storage stays bounded. */
const NOTES_KEY = 'gorge.pairedbot.notes.v1';

export interface SavedNote {
  turn: number;
  text: string;
}

export function loadDifferedNotes(table: string, match: number, seat: number, storage: Storage | null = safeStorage()): SavedNote[] {
  try {
    const saved = JSON.parse(storage?.getItem(NOTES_KEY) ?? 'null') as { table: string; match: number; seat: number; notes: SavedNote[] } | null;
    return saved && saved.table === table && saved.match === match && saved.seat === seat ? saved.notes : [];
  } catch {
    return [];
  }
}

export function saveDifferedNotes(table: string, match: number, seat: number, notes: readonly SavedNote[], storage: Storage | null = safeStorage()): void {
  try {
    storage?.setItem(NOTES_KEY, JSON.stringify({ table, match, seat, notes: notes.map(({ turn, text }) => ({ turn, text })) }));
  } catch {
    // A refused write leaves the notes for this page.
  }
}
