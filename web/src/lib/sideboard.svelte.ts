import type { EventState, SideboardRequest } from '../protocol';
import { ApiError } from './api';
import { DeckBuilder } from './deckbuild.svelte';
import { message } from './draftevent.svelte';
import { eventApi, type EventApi } from './eventapi';

/**
 * Sideboard is the between-games panel's state: one seat's working
 * copy for the next game of its Bo3 series, started from the deck it played
 * last game (EventPool.playing).
 *
 * The working copy is a DeckBuilder, so every change is followed by the
 * server's deck check, and the "full builder" sheet edits the very same
 * copy. Nothing here decides legality: Keep sends `{game, keep: true}`,
 * Submit sends `{game, deck}` (pool card ids and basic counts), and the
 * server answers. A 409 (stale game, already submitted, not between games)
 * refetches — the event state through `onstale`, and the pool — and a 422
 * shows the server's own words.
 */
export class Sideboard {
  readonly builder: DeckBuilder;
  busy = $state(false);
  /** error is the server's refusal (a 422's message, or a failed request). */
  error = $state<string | null>(null);
  /** notice is the expected correction after a 409: the state moved on and was refetched. */
  notice = $state<string | null>(null);
  /** submitted is set once the server accepted this seat's deck for the game. */
  submitted = $state(false);
  /** onstate receives the EventState a successful submit answers with. */
  onstate: ((st: EventState) => void) | null = null;
  /** onstale asks the owner to refetch the event state after a 409. */
  onstale: (() => void | Promise<void>) | null = null;

  readonly id: string;
  readonly #token: string;
  readonly #api: EventApi;

  constructor(id: string, token: string, api: EventApi = eventApi) {
    this.id = id;
    this.#token = token;
    this.#api = api;
    this.builder = new DeckBuilder(id, token, api);
  }

  /** load reads the pool and starts the working copy from the deck played last game. */
  async load(): Promise<void> {
    try {
      this.builder.adoptPlaying(await this.#api.pool(this.id, this.#token));
    } catch (e) {
      this.error = message(e);
      return;
    }
    await this.builder.recheck();
  }

  /** changed reports a working copy that differs from the deck played last game. */
  get changed(): boolean {
    return this.builder.dirty;
  }

  /** keep re-locks last game's deck for `game`. */
  keep(game: number): Promise<boolean> {
    return this.#send({ game, keep: true });
  }

  /** submit locks the working copy for `game` (the server saves it as a new revision). */
  submit(game: number): Promise<boolean> {
    return this.#send({ game, deck: this.builder.deck });
  }

  async #send(req: SideboardRequest): Promise<boolean> {
    if (this.busy) return false;
    this.busy = true;
    this.error = null;
    this.notice = null;
    try {
      const st = await this.#api.sideboard(this.id, this.#token, req);
      this.submitted = true;
      this.onstate?.(st);
      return true;
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        this.notice = 'The series moved on before your deck landed — here is where it stands now.';
        await this.onstale?.();
        await this.load();
      } else {
        this.error = message(e);
      }
      return false;
    } finally {
      this.busy = false;
    }
  }
}
