import type { DraftAdvice, DraftAdvicePick, DraftPack } from '../protocol';
import { ApiError } from './api';
import { eventApi, type EventApi } from './eventapi';
import { safeStorage } from './storage';

/**
 * draftassist is the event screens' bot-assist switch and the draft advice
 * behind it. It is OFF unless the player turns it on; the browser remembers
 * the player's answer (a per-viewer preference, like the paired bot's
 * switch). While on, the draft screen asks the seat's own drafter
 * (GET /api/events/{id}/advice) about each new pack and marks its picks; the
 * build screen shows the bot's suggested commanders with an Apply action.
 *
 * Advice is display only: nothing here is posted, and a pick is still the
 * player's own (round, pick_no, id) intent.
 */

const KEY = 'gorge.eventassist.v1';

export function loadAssistOn(storage: Storage | null = safeStorage()): boolean {
  try {
    return storage?.getItem(KEY) === 'on';
  } catch {
    return false;
  }
}

export function saveAssistOn(on: boolean, storage: Storage | null = safeStorage()): void {
  try {
    if (on) storage?.setItem(KEY, 'on');
    else storage?.removeItem(KEY);
  } catch {
    // A refused write leaves the switch working for this page.
  }
}

/** AdviceMark is one suggested card as the pack draws it: its rank (1-based) and the drafter's reason. */
export interface AdviceMark {
  rank: number;
  pick: DraftAdvicePick;
}

/**
 * DraftAssist holds the switch and the advice for the pack on screen. An
 * answer is kept only when it names the pack still showing (same round and
 * pick number); a 409 means the event's drafter gives no advice, and the
 * switch is withdrawn for this page.
 */
export class DraftAssist {
  on = $state(loadAssistOn());
  /** unavailable is set by a 409: this event's drafter gives no advice. */
  unavailable = $state(false);
  advice = $state<DraftAdvice | null>(null);
  error = $state<string | null>(null);
  #seq = 0;
  readonly #api: EventApi;

  constructor(api: EventApi = eventApi) {
    this.#api = api;
  }

  toggle(): void {
    this.on = !this.on;
    saveAssistOn(this.on);
    if (!this.on) this.advice = null;
  }

  /** forPack is the advice for this pack, or null when none (or a stale one) is held. */
  forPack(p: DraftPack | null): DraftAdvice | null {
    const a = this.advice;
    if (!a || !p || p.waiting || a.waiting) return null;
    return a.round === p.round && a.pick_no === p.pick_no ? a : null;
  }

  /** marks maps a card id to its rank and reason, for the pack on screen. */
  marks(p: DraftPack | null): Map<string, AdviceMark> {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway index per read; advice is the reactive source
    const out = new Map<string, AdviceMark>();
    (this.forPack(p)?.picks ?? []).forEach((pick, i) => out.set(pick.id, { rank: i + 1, pick }));
    return out;
  }

  /** fetch asks about the pack on screen; an answer for an older pack (or after a newer ask) is dropped. */
  async fetch(event: string, token: string, p: DraftPack | null): Promise<void> {
    if (!this.on || this.unavailable || !p || p.waiting || !this.#api.advice) return;
    if (this.forPack(p)) return;
    const seq = ++this.#seq;
    try {
      const a = await this.#api.advice(event, token);
      if (seq !== this.#seq) return;
      this.error = null;
      if (a.round === p.round && a.pick_no === p.pick_no) this.advice = a;
    } catch (e) {
      if (seq !== this.#seq) return;
      if (e instanceof ApiError && e.status === 409) {
        this.unavailable = true;
        this.advice = null;
        return;
      }
      this.error = 'The bot did not answer for this pack.';
    }
  }
}
