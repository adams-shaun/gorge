import type { DraftPack, EventSeries, EventState, Frame, PodProgress, SeriesUpdate } from '../protocol';
import { ApiError } from './api';
import { eventApi, isStale, type EventApi } from './eventapi';
import type { EventSeat } from './eventseat';

/** The event phases, in order (event.Phase on the server). */
export const PHASES = ['LOBBY', 'DRAFT', 'BUILD', 'ROUND1', 'ROUND2', 'ROUND3', 'COMPLETE'] as const;
export type Phase = (typeof PHASES)[number];

/** isRound reports a ROUNDn phase. */
export const isRound = (phase: string | undefined): boolean => !!phase && /^ROUND\d+$/.test(phase);

/**
 * EventClient is one event page's live state: the viewer's EventState, its
 * pack at the queue head, the pod's progress and the card it has selected.
 *
 * Everything it holds is a server answer. A pick sends only (round, pick_no,
 * card id); the server's reply — or the draft_pack frame the same mutation
 * pushed — replaces the pack. A stale pick (409 stale_pick / not_your_turn,
 * or a card that already left the pack) is never retried: the client
 * refetches and shows what the server now offers.
 */
export class EventClient {
  state = $state<EventState | null>(null);
  pack = $state<DraftPack | null>(null);
  progress = $state<PodProgress | null>(null);
  selected = $state<string | null>(null);
  /** packAt is the client clock (ms) when the current pack arrived: the pick countdown starts from the pack's remaining_ms there. */
  packAt = $state(0);
  busy = $state(false);
  /** error is a failure the player must see (a refused join, an unreachable server). */
  error = $state<string | null>(null);
  /** notice is a transient, expected correction (the pack moved on). */
  notice = $state<string | null>(null);
  /** missing is set when the event does not exist (404). */
  missing = $state(false);
  /** unauthorised is set when the server refused the seat token: the page falls back to spectating. */
  unauthorised = $state(false);

  readonly id: string;
  seat: EventSeat | null;
  readonly #api: EventApi;

  constructor(id: string, seat: EventSeat | null, api: EventApi = eventApi) {
    this.id = id;
    this.seat = seat;
    this.#api = api;
  }

  /** you is the viewer's seat as the SERVER resolved it (EventState.you), null for a spectator. */
  get you(): number | null {
    return this.state?.you ?? null;
  }

  get phase(): string {
    return this.state?.phase ?? '';
  }

  /** joined reports whether the viewer's human seat has a display name yet. */
  get joined(): boolean {
    const you = this.you;
    if (you === null || !this.state) return false;
    return this.state.seats[you]?.joined ?? false;
  }

  get token(): string | null {
    return this.seat?.token ?? null;
  }

  /** load fetches the state and, while drafting, the viewer's pack. It is also the reconnect refetch. */
  async load(): Promise<void> {
    try {
      this.state = await this.#api.state(this.id, this.token);
      this.missing = false;
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        this.missing = true;
        return;
      }
      if (e instanceof ApiError && e.status === 401 && this.seat) {
        // A token the server does not know: keep the page usable as a spectator.
        this.unauthorised = true;
        this.seat = null;
        return this.load();
      }
      this.error = message(e);
      return;
    }
    if (this.phase === 'DRAFT' && this.you !== null && this.token) await this.loadPack();
  }

  async loadPack(): Promise<void> {
    if (!this.token) return;
    try {
      this.adoptPack(await this.#api.pack(this.id, this.token));
    } catch (e) {
      this.error = message(e);
    }
  }

  /** join records the viewer's display name (LOBBY); the server validates it. */
  async join(name: string): Promise<boolean> {
    if (!this.token) return false;
    this.busy = true;
    this.error = null;
    try {
      this.state = await this.#api.join(this.id, this.token, name);
      if (this.phase === 'DRAFT') await this.loadPack();
      return true;
    } catch (e) {
      this.error = message(e);
      return false;
    } finally {
      this.busy = false;
    }
  }

  /** select marks one card of the current pack for inspection; a second select of the same card keeps it. */
  select(id: string | null): void {
    if (id !== null && !this.pack?.pack.some((c) => c.id === id)) return;
    this.selected = id;
  }

  /** pick takes a card (default: the selected one) from the pack at the queue head. */
  async pick(id: string | null = this.selected): Promise<void> {
    const p = this.pack;
    if (!id || !p || p.waiting || !this.token || this.busy) return;
    this.busy = true;
    this.error = null;
    this.notice = null;
    try {
      const next = await this.#api.pick(this.id, this.token, { round: p.round, pick_no: p.pick_no, id });
      this.adoptPack(next);
      // No pack queued: either a neighbour is still picking or that was the
      // last pick of the draft. The state says which (DRAFT or BUILD) even
      // when no event_state frame reaches this page.
      if (next.waiting) this.state = await this.#api.state(this.id, this.token);
    } catch (e) {
      if (isStale(e)) {
        this.notice = 'That pack moved on before your pick landed — here is the pack you hold now.';
        this.selected = null;
        await this.load();
      } else {
        this.error = message(e);
      }
    } finally {
      this.busy = false;
    }
  }

  /** adoptPack takes a server pack (REST reply or draft_pack frame), keeping the selection only while that card is still on offer. */
  adoptPack(p: DraftPack): void {
    if (this.you !== null && p.seat !== this.you) return;
    this.pack = { ...p, pack: p.pack ?? [], pool: p.pool ?? [] };
    this.packAt = Date.now();
    if (this.selected && !this.pack.pack.some((c) => c.id === this.selected)) this.selected = null;
  }

  /** handleFrame applies one SSE frame addressed to this event. It reports whether the frame was this event's. */
  handleFrame(f: Frame): boolean {
    switch (f.t) {
      case 'event_state': {
        const st = f.body as EventState;
        if (st.event !== this.id) return false;
        const wasDraft = this.phase === 'DRAFT';
        this.state = st;
        if (wasDraft && st.phase !== 'DRAFT') this.selected = null;
        return true;
      }
      case 'draft_pack': {
        const p = f.body as DraftPack;
        if (p.event !== this.id) return false;
        this.adoptPack(p);
        return true;
      }
      case 'pod_progress': {
        const p = f.body as PodProgress;
        if (p.event !== this.id) return false;
        this.progress = p;
        return true;
      }
      case 'series_update': {
        const u = f.body as SeriesUpdate;
        if (u.event !== this.id || !this.state) return false;
        this.state = { ...this.state, series: patchSeries(this.state.series ?? [], u.series) };
        return true;
      }
    }
    return false;
  }
}

/** patchSeries replaces the (round, a, b) row with the update, or appends it. */
export function patchSeries(list: EventSeries[], s: EventSeries): EventSeries[] {
  let found = false;
  const out = list.map((x) => {
    if (x.round === s.round && x.a === s.a && x.b === s.b) {
      found = true;
      // The update is the server's row; a Join URL is only ever in the
      // viewer's own state, so keep one the update does not carry.
      return { ...s, join: s.join ?? (s.state === 'live' ? x.join : undefined) };
    }
    return x;
  });
  return found ? out : [...out, s];
}

export function message(e: unknown): string {
  if (e instanceof ApiError) return e.message || `${e.status} ${e.code}`;
  return e instanceof Error ? e.message : String(e);
}
