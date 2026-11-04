import type { Advice, Decision } from '../protocol';
import { isManualManaOption } from './manualmana';
import { promptWords } from './prompts/words';
import type { ExplainNote, ExplainRow, ExplainSearch } from './review';
import { safeStorage } from './storage';

/**
 * The paired bot's client side: the player's on/off preference, the words of
 * its search rows, the pick it marks on the decision, and the note for a
 * player who answers differently. The preference is a player's, like the play
 * settings, so global keys rather than one per table.
 */
const KEY = 'gorge.pairedbot.v1';
/** The switch at a table with another human, remembered apart from the one for games alone with bots. */
const SHARED_KEY = 'gorge.pairedbot.shared.v1';
/** Whether the panel marks what the bot's rules would do. Stored only when turned off. */
const RULES_KEY = 'gorge.pairedbot.rules.v1';
/** Whether the bot's pick is marked on the decision itself. Stored only when turned off. */
const MARK_KEY = 'gorge.pairedbot.mark.v1';

/**
 * loadPairedBotOn is the player's switch. Alone with bots it starts on, so the
 * bot advises from the first decision; at a table with another human it starts
 * off. Each remembers the player's own answer.
 */
export function loadPairedBotOn(shared: boolean, storage: Storage | null = safeStorage()): boolean {
  try {
    const v = storage?.getItem(shared ? SHARED_KEY : KEY);
    return v === 'on' ? true : v === 'off' ? false : !shared;
  } catch {
    return !shared;
  }
}

export function savePairedBotOn(on: boolean, shared: boolean, storage: Storage | null = safeStorage()): void {
  try {
    storage?.setItem(shared ? SHARED_KEY : KEY, on ? 'on' : 'off');
  } catch {
    // A refused write leaves the toggle working for this page.
  }
}

export function loadMarkPick(storage: Storage | null = safeStorage()): boolean {
  try {
    return storage?.getItem(MARK_KEY) !== 'off';
  } catch {
    return true;
  }
}

export function saveMarkPick(show: boolean, storage: Storage | null = safeStorage()): void {
  try {
    if (show) storage?.removeItem(MARK_KEY);
    else storage?.setItem(MARK_KEY, 'off');
  } catch {
    // A refused write leaves the setting for this page.
  }
}

export function loadShowRules(storage: Storage | null = safeStorage()): boolean {
  try {
    return storage?.getItem(RULES_KEY) !== 'off';
  } catch {
    return true;
  }
}

export function saveShowRules(show: boolean, storage: Storage | null = safeStorage()): void {
  try {
    if (show) storage?.removeItem(RULES_KEY);
    else storage?.setItem(RULES_KEY, 'off');
  } catch {
    // A refused write leaves the setting for this page.
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
  /** The percentage of the search's lines that looked at it. */
  share: number;
  recommended: boolean;
  /** What the bot's rules would do without searching. */
  rules: boolean;
}

/** adviceRows gathers a search's rows by what a player would call them, most visited first. */
export function adviceRows(d: Decision, search: ExplainSearch): AdviceRow[] {
  const named = new Map<string, { visits: number; value: number; recommended: boolean; rules: boolean }>();
  for (const r of search.rows ?? []) {
    const label = pairedRowLabel(d, r);
    const row = named.get(label) ?? { visits: 0, value: 0, recommended: false, rules: false };
    row.visits += r.visits;
    row.value += r.q * r.visits;
    row.recommended ||= r.chosen === true;
    row.rules ||= r.bot === true;
    named.set(label, row);
  }
  const lines = search.sims || [...named.values()].reduce((sum, r) => sum + r.visits, 0) || 1;
  return [...named]
    .map(([label, r]) => ({
      label,
      visits: r.visits,
      chance: winChance({ visits: r.visits, q: r.value / Math.max(1, r.visits) }),
      share: Math.round((100 * r.visits) / lines),
      recommended: r.recommended,
      rules: r.rules,
    }))
    .sort((a, b) => b.visits - a.visits);
}

/** One step of what search expects to happen after the recommended play. */
export interface ExpectedStep {
  who: string;
  label: string;
  /** How much of the lines that reached this decision went this way, as a percentage. */
  share: number;
  /** Where the lines through it end up, as a percentage; the player's perspective. */
  chance: number;
}

/**
 * expectedLine words a search's most-visited continuation after its
 * recommendation. The server leaves out plies that depend on cards the
 * player cannot see, so the line may be short; the second value says so.
 */
export function expectedLine(search: ExplainSearch): { steps: ExpectedStep[]; cutShort: boolean } {
  const steps = (search.line ?? []).map((p) => ({
    who: p.actor === 'you' ? 'You' : 'Opponent',
    label: promptWords(p.label),
    share: Math.round(100 * p.share),
    chance: Math.round((p.q + 1) * 50),
  }));
  return { steps, cutShort: search.line_end === 'hidden' };
}

/** shownRows is the first n rows, the recommendation always among them, and how many that leaves out. */
export function shownRows(rows: readonly AdviceRow[], n: number, dropped = 0): { rows: AdviceRow[]; more: number } {
  const shown = rows.slice(0, n);
  const best = rows.find((r) => r.recommended);
  if (best && !shown.includes(best)) shown[shown.length - 1] = best;
  return { rows: shown, more: rows.length - shown.length + dropped };
}

/**
 * BotPick is the answer the paired bot would give one decision, as the
 * decision's own controls name it, so each can mark itself. It is display
 * only: nothing posts it.
 */
export interface BotPick {
  seq: number;
  /** The option indices of its answer; a planned cast's legacy option. Empty when it would decline: no attack, no blocks. */
  options: number[];
  /** The payment action of the planned cast it would make. */
  payment: string | null;
  /** The cards its answer names: what it would cast, activate, attack or block with, or target. */
  objs: number[];
  /** Its estimate as a percentage; null when search did not weigh it and the bot's rules picked. */
  chance: number | null;
}

/**
 * botPickOf is the pick advice a recommends for decision d, null when the
 * advice is for another decision or recommends nothing. Search's
 * recommendation comes first; a decision search does not weigh shows the bot's
 * rules pick, with no estimate.
 */
export function botPickOf(d: Decision, a: Advice): BotPick | null {
  if (a.state !== 'ready' || a.ask_seq !== d.seq || a.explain === null || typeof a.explain !== 'object') return null;
  const n = a.explain as ExplainNote;
  const chosen = n.search?.rows?.find((r) => r.chosen === true);
  let options: number[];
  let payment: string | null = null;
  let chance: number | null = null;
  if (n.search && chosen) {
    payment = chosen.payment ?? null;
    const base = payment === null ? undefined : d.payment_actions?.find((x) => x.id === payment)?.base_option_index;
    options = payment === null ? [...(chosen.choices ?? [])] : base === undefined || base === null ? [] : [base];
    chance = adviceRows(d, n.search).find((r) => r.recommended)?.chance ?? null;
  } else if (n.bot) {
    options = [...n.bot];
  } else {
    return null;
  }
  const objs = new Set<number>();
  for (const i of options) {
    const obj = d.options.find((o) => o.index === i)?.obj;
    if (obj) objs.add(obj);
  }
  if (payment !== null) {
    const obj = d.payment_actions?.find((x) => x.id === payment)?.cast.object;
    if (obj) objs.add(obj);
  }
  return { seq: d.seq, options, payment, objs: [...objs], chance };
}

/** declines reports whether pick p, for decision d, answers with nothing: no attack, no blocks. */
export function declines(p: BotPick | null, d: Decision): boolean {
  return p !== null && p.seq === d.seq && p.payment === null && p.options.length === 0;
}

/** pickTag is the words on a control the bot would press. */
export function pickTag(p: BotPick): string {
  return p.chance === null ? "Bot's pick" : `Bot's pick, ${p.chance}%`;
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
