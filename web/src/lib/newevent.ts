import type { BotPolicyInfo, CreateEventRequest, EventSetInfo } from '../protocol';
import { withBase } from './basepath';

/** POD_SEATS is the pod size: four seats, each a human (an invite link) or a bot. */
export const POD_SEATS = 4;

/** BOT_SEATS is the default bot count: seat 1 is you, seats 2–4 bots. */
export const BOT_SEATS = POD_SEATS - 1;

/** DEFAULT_PERSONA is the pickbot persona the server seats when a create body names none (eventhost: Drafter "" = "rating-colour"). */
export const DEFAULT_PERSONA = 'rating-colour';

/** RATINGS_17LANDS is EventSetInfo.ratings for a set whose pick ratings come from 17Lands' public data. */
export const RATINGS_17LANDS = '17lands';

/**
 * setNote is the line under a set's name on the create screen: the server's
 * pack size and rating source. Only a 17lands-rated set names 17Lands.
 */
export function setNote(s: EventSetInfo): string {
  const ratings = `ratings: ${s.ratings === RATINGS_17LANDS ? '17Lands' : 'card heuristic'}`;
  return s.pack_size > 0 ? `${s.pack_size}-card packs · ${ratings}` : ratings;
}

/**
 * ratedBy17Lands reports whether a set's bot ratings come from 17Lands, so
 * the screens owe its CC BY credit. Unknown — the catalogue not loaded, or
 * a set it does not list — keeps the credit: attribution the data may
 * need is never dropped on a guess.
 */
export function ratedBy17Lands(sets: EventSetInfo[] | null, code: string | undefined): boolean {
  const s = sets?.find((x) => x.code === code);
  return !s || s.ratings === RATINGS_17LANDS;
}

/**
 * eventPolicies narrows the server's offered bot policies to those an event
 * can seat. /api/bot-policies already lists only policies that cannot read
 * an opponent's decklist (bots.Info.Env == false); an event game is a
 * constructed-format table, so a policy must play constructed. The server
 * re-checks every name at create time.
 */
export function eventPolicies(list: BotPolicyInfo[], format: EventFormat = 'draft'): BotPolicyInfo[] {
  const plays = format === 'commander' ? 'commander' : 'constructed';
  return list.filter((p) => p.formats.includes(plays));
}

/** SeatKind is a create-screen seat: a human (who gets an invite link) or a bot. */
export type SeatKind = 'human' | 'bot';

/** SeatChoice is one seat row of the create screen. policy/drafter apply to a bot ("" = the server default). */
export interface SeatChoice {
  kind: SeatKind;
  policy: string;
  drafter: string;
}

/** PickTimer is the create body's pick_timer: "" leaves the choice to the server. */
export type PickTimer = '' | 'off' | 'standard' | 'relaxed';

/** PICK_TIMERS are the offered pick-timer modes, in segmented-control order. */
export const PICK_TIMERS: { mode: Exclude<PickTimer, ''>; label: string }[] = [
  { mode: 'off', label: 'Off' },
  { mode: 'standard', label: 'Standard' },
  { mode: 'relaxed', label: 'Relaxed' },
];

/** defaultSeats is the create screen's starting pod: seat 1 is you (human), seats 2–4 bots. */
export function defaultSeats(): SeatChoice[] {
  return Array.from({ length: POD_SEATS }, (_, i) => ({ kind: i === 0 ? 'human' : 'bot', policy: '', drafter: '' }));
}

/** humanCount counts the human seats of a pod. */
export const humanCount = (seats: readonly SeatChoice[]): number => seats.filter((s) => s.kind === 'human').length;

/**
 * timerOffered reports whether a pick timer can be chosen: the server
 * stores "off" for a pod with fewer than two humans whatever is sent, so
 * the control is disabled (showing Off) there.
 */
export const timerOffered = (seats: readonly SeatChoice[]): boolean => humanCount(seats) >= 2;

/**
 * EventFormat is the create body's supply format. "commander" is a
 * Commander draft (Mystery Booster Commander style): offered only for a set
 * the server reports `commander: true` for.
 */
export type EventFormat = 'draft' | 'sealed' | 'prerelease' | 'commander';

/** FORMATS lists the Mode segment's buttons, in order. */
export const FORMATS: { format: EventFormat; label: string }[] = [
  { format: 'draft', label: 'Draft' },
  { format: 'sealed', label: 'Sealed' },
  { format: 'prerelease', label: 'Prerelease' },
];

/**
 * commanderSets is the server's Commander-draft-capable sets. The flag is
 * the capability: with none, the create screen offers no Commander mode.
 */
export const commanderSets = (sets: readonly EventSetInfo[] | null): EventSetInfo[] => (sets ?? []).filter((s) => s.commander === true);

/**
 * commanderLabel names the Commander mode's button: "MBC Commander Draft"
 * when the server offers exactly one set for it, else the generic name.
 */
export function commanderLabel(sets: readonly EventSetInfo[] | null): string {
  const c = commanderSets(sets);
  return c.length === 1 ? `${c[0].code} Commander Draft` : 'Commander Draft';
}

/** commanderFormat reports a Commander-draft event (EventState.format "commander"). */
export const commanderFormat = (f: string | undefined): boolean => f === 'commander';

/** sealedFormat reports a format with no picks (sealed or prerelease). */
export const sealedFormat = (f: string | undefined): boolean => f === 'sealed' || f === 'prerelease';

/** defaultBestOf is the server's default series length per format: best of 3 for a prerelease (MTR 2.1), else 1. */
export const defaultBestOf = (f: EventFormat): 1 | 3 => (f === 'prerelease' ? 3 : 1);

/** seriesFormat reports a format whose rounds are head-to-head series (every format but a Commander draft's pod games). */
export const seriesFormat = (f: EventFormat): boolean => f !== 'commander';

/** defaultRebuild is the server's default for deck changes between rounds: on for sealed formats (MTR 7.3). */
export const defaultRebuild = (f: EventFormat): boolean => sealedFormat(f);

/** CreateOptions are the create screen's format choices. */
export interface CreateOptions {
  bestOf?: 1 | 3;
  /** pickTimer is sent only when the player chose one and the pod has two or more humans. */
  pickTimer?: PickTimer;
  format?: EventFormat;
  rebuild?: boolean;
}

/**
 * createRequest builds the POST /api/events body: each seat's kind, and for
 * a bot its chosen game policy and pickbot persona ("" = the server's
 * default for either). format is sent only when it is not a draft; best_of
 * only when it differs from the format's server default, so a draft or a
 * prerelease keeps its default unless the player changed it. A Commander
 * draft sends neither best_of nor rebuild (its rounds are free-for-all pod
 * games, not series) nor a bot drafter (the server seats its Commander-draft
 * drafter whatever is named). rebuild is sent only when the player's choice differs from the format's default, and
 * pick_timer only for a draft pod the server times. It never carries a seed:
 * the server mints it, and an unknown body field is a 400.
 */
export function createRequest(set: string, seats: readonly SeatChoice[], opts: CreateOptions = {}): CreateEventRequest {
  const format = opts.format ?? 'draft';
  const req: CreateEventRequest = {
    set,
    seats: seats.map((s) => {
      if (s.kind === 'human') return { kind: 'human' };
      const seat: CreateEventRequest['seats'][number] = { kind: 'bot' };
      if (s.policy) seat.policy = s.policy;
      if (s.drafter && format !== 'commander') seat.drafter = s.drafter;
      return seat;
    }),
  };
  if (seriesFormat(format) && opts.bestOf !== undefined && opts.bestOf !== defaultBestOf(format)) req.best_of = opts.bestOf;
  if (format !== 'draft') req.format = format;
  if (seriesFormat(format) && opts.rebuild !== undefined && opts.rebuild !== defaultRebuild(format)) req.rebuild = opts.rebuild;
  if (!sealedFormat(format) && opts.pickTimer && timerOffered(seats)) req.pick_timer = opts.pickTimer;
  return req;
}

/** timerLabel names an event's stored pick-timer mode for the lobby and topbar. */
export function timerLabel(mode: string | undefined): string {
  switch (mode) {
    case 'standard':
      return 'standard pick timer';
    case 'relaxed':
      return 'relaxed pick timer';
    default:
      return 'no pick timer';
  }
}

/**
 * inviteURL is the full link a create screen hands out for one invite: the
 * page origin plus the server's join path (under the mount base).
 */
export const inviteURL = (origin: string, join: string): string => origin + withBase(join);

/**
 * maskToken hides an invite link's seat token for display: the link is a
 * bearer credential, so the Invites panel shows it masked until the player
 * asks to see it (Copy always copies the real link).
 */
export const maskToken = (url: string): string => url.replace(/([?&]token=)[^&#]*/, '$1••••••');
