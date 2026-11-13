import type { DeckCheck, DeckRequest, DraftCard, EventPool, EventState } from '../protocol';
import type { EventApi } from './eventapi';
import { eventApi } from './eventapi';
import { message } from './draftevent.svelte';
import { COLOURS, type ColourFilter } from './draftcards';
import { isBasicLand, unionIdentity, withinIdentity } from './commanderdraft';

export type Basics = [number, number, number, number, number];

/** STEPPER_MAX mirrors limited/build.MaxBasicsPerType: a stepper stops there, and the server still validates whatever arrives. */
export const STEPPER_MAX = 40;

export type Zone = 'deck' | 'side';

/**
 * DeckBuilder is the BUILD screen's working copy of one seat's deck: which
 * pool card ids are maindeck, and how many of each basic.
 *
 * The client never decides legality. Every change is followed by a
 * POST deck/check, and the screen shows that DeckCheck — total, validity,
 * the server's error text — verbatim. "Auto" lands are the server's
 * auto_lands for the current main; "Reset" is the server's bot suggestion.
 * Ready saves the working copy (a new revision, which the server validates
 * again) and readies exactly that revision.
 *
 * In a Commander draft (`commanderMode`) the working copy also names one or
 * two commanders, which are always maindecked. The commanders' colour
 * identity (read off the wire's DraftCard.identity) greys out pool cards
 * outside it, and the builder refuses to add them: the server would reject
 * the deck anyway, and its DeckCheck still rules.
 */
export class DeckBuilder {
  pool = $state<DraftCard[]>([]);
  main = $state<string[]>([]);
  basics = $state<Basics>([0, 0, 0, 0, 0]);
  filter = $state<ColourFilter>('all');
  check = $state<DeckCheck | null>(null);
  /** rev is the latest saved revision; ready whether the seat has readied it. */
  rev = $state(0);
  ready = $state(false);
  loaded = $state(false);
  busy = $state(false);
  error = $state<string | null>(null);
  suggestion = $state<DeckRequest | null>(null);
  /** commanderMode is set for a Commander-draft event (EventState.format "commander"), before load. */
  commanderMode = $state(false);
  /** commanders is the working copy's one or two commander ids (each also in main). */
  commanders = $state<string[]>([]);
  /** notice is a refused client-side move, said once (a card outside the identity). */
  notice = $state<string | null>(null);
  /** onstate receives the EventState a Ready answers with (the event page adopts it: the last Ready opens round 1). */
  onstate: ((st: EventState) => void) | null = null;
  #saved = $state<string>('');
  #checkSeq = 0;

  readonly id: string;
  readonly #token: string;
  readonly #api: EventApi;

  constructor(id: string, token: string, api: EventApi = eventApi) {
    this.id = id;
    this.#token = token;
    this.#api = api;
  }

  /** load reads the pool, then starts from the saved deck, else the bot suggestion. */
  async load(): Promise<void> {
    try {
      this.adopt(await this.#api.pool(this.id, this.#token));
    } catch (e) {
      this.error = message(e);
      return;
    }
    await this.recheck();
  }

  adopt(p: EventPool): void {
    this.pool = p.pool ?? [];
    this.rev = p.rev;
    this.ready = p.ready;
    this.suggestion = p.suggestion
      ? { main: p.suggestion.main ?? [], basics: p.suggestion.basics, ...(p.suggestion.commanders ? { commanders: p.suggestion.commanders } : {}) }
      : null;
    const start = p.deck ?? this.suggestion;
    this.setDeck(start ?? { main: [], basics: [0, 0, 0, 0, 0] });
    this.#saved = p.deck ? key(p.deck) : '';
    this.loaded = true;
  }

  /**
   * adoptPlaying starts the working copy from the deck the seat played last
   * game (EventPool.playing, the sideboarding start), falling back to the
   * saved deck, and treats that start as "saved" so `dirty` reads as
   * "changed since last game".
   */
  adoptPlaying(p: EventPool): void {
    this.adopt(p);
    const start = p.playing ?? p.deck ?? null;
    if (start) this.setDeck(start);
    this.#saved = key(this.deck);
  }

  get deck(): DeckRequest {
    const d: DeckRequest = { main: [...this.main], basics: [...this.basics] as Basics };
    if (this.commanderMode) d.commanders = [...this.commanders];
    return d;
  }

  /** commanderCards are the chosen commanders' pool cards, in slot order. */
  get commanderCards(): DraftCard[] {
    return this.commanders.map((id) => this.pool.find((c) => c.id === id)).filter((c): c is DraftCard => c !== undefined);
  }

  /**
   * identity is the chosen commanders' combined colour identity, or null
   * with no commander chosen (nothing is greyed out then). The server's
   * DeckCheck.identity is the same union, computed server-side.
   */
  get identity(): string | null {
    if (!this.commanderMode || this.commanders.length === 0) return null;
    return unionIdentity(this.commanderCards);
  }

  /** offIdentity reports a card the chosen commanders' identity excludes (basics never are). */
  offIdentity(c: DraftCard): boolean {
    const id = this.identity;
    return id !== null && !isBasicLand(c) && !withinIdentity(c, id);
  }

  /** offIdentityMain lists the maindeck cards outside the identity (left behind by a commander change). */
  get offIdentityMain(): DraftCard[] {
    return this.deckCards.filter((c) => this.offIdentity(c));
  }

  /**
   * setCommander fills slot 0 or 1 with a pool card (null empties it). The
   * card joins the main deck; a replaced commander stays maindecked as an
   * ordinary card. Emptying slot 0 empties slot 1 too (a partner needs a
   * first commander), and so does a first commander that cannot partner.
   * Basics outside the new identity are cleared.
   */
  setCommander(slot: 0 | 1, id: string | null): void {
    const next = [...this.commanders];
    if (id !== null && !this.pool.some((c) => c.id === id)) return;
    if (slot === 0) {
      if (id === null) next.length = 0;
      else next[0] = id;
      const first = this.pool.find((c) => c.id === next[0]);
      if (next.length > 1 && (first?.partner !== true || next[1] === next[0])) next.length = 1;
    } else {
      if (next.length === 0) return;
      if (id === null || id === next[0]) next.length = 1;
      else next[1] = id;
    }
    this.commanders = next;
    for (const c of next) if (!this.main.includes(c)) this.main = [...this.main, c];
    const id2 = this.identity;
    if (id2 !== null) {
      const b = this.basics.map((n, i) => (id2.includes(COLOURS[i]) ? n : 0)) as Basics;
      if (b.some((n, i) => n !== this.basics[i])) this.basics = b;
    }
    this.notice = null;
    void this.recheck();
  }

  /** dropOffIdentity sends every maindeck card outside the identity back to the pool. */
  dropOffIdentity(): void {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway lookup inside one write
    const off = new Set(this.offIdentityMain.map((c) => c.id));
    if (off.size === 0) return;
    this.main = this.main.filter((id) => !off.has(id));
    void this.recheck();
  }

  /** dirty is true when the working copy differs from the latest saved revision (or nothing is saved). */
  get dirty(): boolean {
    return key(this.deck) !== this.#saved;
  }

  inDeck(id: string): boolean {
    return this.main.includes(id);
  }

  get deckCards(): DraftCard[] {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway index per read; main is the reactive source
    const order = new Map(this.main.map((id, i) => [id, i]));
    return this.pool.filter((c) => order.has(c.id)).sort((a, b) => order.get(a.id)! - order.get(b.id)!);
  }

  get sideCards(): DraftCard[] {
    return this.pool.filter((c) => !this.main.includes(c.id));
  }

  /** move puts one pool card into a zone (toggles when no zone is named): the click/tap path and the drop path both land here. */
  move(id: string, to?: Zone): void {
    if (!this.pool.some((c) => c.id === id)) return;
    const there = this.main.includes(id);
    const want = to === undefined ? !there : to === 'deck';
    if (want === there) return;
    const card = this.pool.find((c) => c.id === id)!;
    if (want && this.offIdentity(card)) {
      this.notice = `${card.name} is outside your commanders' colour identity.`;
      return;
    }
    this.notice = null;
    this.main = want ? [...this.main, id] : this.main.filter((x) => x !== id);
    // A commander sent to the sideboard is no longer a commander.
    if (!want && this.commanders.includes(id)) this.setCommander(this.commanders[0] === id ? 0 : 1, this.commanders[0] === id ? (this.commanders[1] ?? null) : null);
    void this.recheck();
  }

  /** step nudges one basic's count by delta, clamped to the stepper's range. */
  step(colour: number, delta: number): void {
    if (colour < 0 || colour > 4) return;
    const next = [...this.basics] as Basics;
    next[colour] = Math.min(STEPPER_MAX, Math.max(0, next[colour] + delta));
    if (next[colour] === this.basics[colour]) return;
    this.basics = next;
    void this.recheck();
  }

  setFilter(f: ColourFilter): void {
    this.filter = f;
  }

  /** autoLands replaces the basics with the server's auto-land split for the current main. */
  async autoLands(): Promise<void> {
    const c = await this.recheck();
    if (!c) return;
    this.basics = [...c.auto_lands] as Basics;
    await this.recheck();
  }

  /** resetToSuggestion restores the server's bot-built deck for this pool. */
  async resetToSuggestion(): Promise<void> {
    if (!this.suggestion) return;
    this.setDeck(this.suggestion);
    await this.recheck();
  }

  /** recheck asks the server about the working copy; an answer for an older copy is dropped. */
  async recheck(): Promise<DeckCheck | null> {
    const seq = ++this.#checkSeq;
    try {
      const c = await this.#api.check(this.id, this.#token, this.deck);
      if (seq !== this.#checkSeq) return null;
      this.check = c;
      this.error = null;
      return c;
    } catch (e) {
      if (seq === this.#checkSeq) this.error = message(e);
      return null;
    }
  }

  /** readyUp saves the working copy when it changed, then readies the saved revision. */
  async readyUp(): Promise<boolean> {
    if (this.busy) return false;
    this.busy = true;
    this.error = null;
    try {
      if (this.dirty || this.rev === 0) {
        const deck = this.deck;
        const saved = await this.#api.save(this.id, this.#token, deck);
        this.check = saved;
        if (saved.rev) this.rev = saved.rev;
        this.#saved = key(deck);
        this.ready = false;
      }
      const st = await this.#api.ready(this.id, this.#token, this.rev);
      this.ready = true;
      this.onstate?.(st);
      return true;
    } catch (e) {
      this.error = message(e);
      return false;
    } finally {
      this.busy = false;
    }
  }

  private setDeck(d: DeckRequest): void {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway lookup inside one write
    const ids = new Set(this.pool.map((c) => c.id));
    this.main = (d.main ?? []).filter((id) => ids.has(id));
    this.basics = [...(d.basics ?? [0, 0, 0, 0, 0])] as Basics;
    this.commanders = (d.commanders ?? []).filter((id) => ids.has(id)).slice(0, 2);
    for (const c of this.commanders) if (!this.main.includes(c)) this.main = [...this.main, c];
  }
}

function key(d: DeckRequest): string {
  return JSON.stringify({ m: [...(d.main ?? [])].sort(), b: d.basics, c: d.commanders ?? [] });
}

/** basicsTotal sums a basics array. */
export const basicsTotal = (b: readonly number[]): number => b.reduce((a, n) => a + n, 0);
