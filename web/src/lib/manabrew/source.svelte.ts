import type { CardView, Decision, Intent, MatchInfo, View } from '../../protocol';
import { ApiError, fetchMatches, postNativeUndo } from '../api';
import { dvrReducer } from '../dvr';
import type { SeatCtx } from '../seat';
import { transitionLine } from '../clientmodel/log';
import { diffViews } from '../clientmodel/snapshotdiff';
import { WireError } from '../wire/http';
import { SnapshotMatch, type StartResult } from '../wire/snapshot.svelte';
import { applyPatch, PatchError } from './patch';
import { compatPass, loadCompat, nothingToDo } from './compat';
import { AnswerError, bindPrompt, undoMessage, type PromptBinding } from './prompt';
import { projectView, rememberCards } from './project';
import { httpTransport, streamURL, type Transport } from './transport';
import type { AgentPrompt, ClientMessage, EngineMessage, GameViewDto } from './wire';

/**
 * ManaBrewMatch is the ManaBrew adapter: the table
 * route's game source when the protocol setting is ManaBrew. The protocol has
 * no event log, so transitions come from diffing successive projected
 * snapshots (diffViews) and the transcript is built from those transitions
 * (transitionLine). The seat's answers go over ManaBrew's /send.
 */

interface Deps {
  transport: Transport;
  fetchMatches: (t: string, ctx?: SeatCtx) => Promise<MatchInfo[]>;
  /** The compat auto-pass (compat.ts); read from the setting when not given. */
  compat: boolean;
}

export class ManaBrewMatch extends SnapshotMatch {
  readonly protocol = 'manabrew' as const;
  private deps: Deps;
  private gv: GameViewDto | null = null;
  private binding: PromptBinding | null = null;
  private lastPromptId = -1;
  /** The promptId this seat last answered: the server poses it again only after an undo. */
  private answeredPromptId = -1;
  // eslint-disable-next-line svelte/prefer-svelte-reactivity -- private projection cache (last-known card faces), never rendered
  private known = new Map<number, CardView>();
  /** How many prompts the compat rule answered with a pass (diagnostics). */
  autoPassed = 0;

  constructor(table: string, seat: SeatCtx, deps: Partial<Deps> = {}) {
    super(table, seat);
    this.deps = { transport: deps.transport ?? httpTransport, fetchMatches: deps.fetchMatches ?? fetchMatches, compat: deps.compat ?? loadCompat() };
  }

  /**
   * start finds the table's live match, probes the ManaBrew state route and
   * opens the stream. A server without -manabrew (or a refused token)
   * resolves {ok:false} with a reason and leaves nothing open, so the route
   * can fall back to the native wire without waiting on a stream that will
   * never speak.
   */
  async start(): Promise<StartResult> {
    let info: MatchInfo | undefined;
    try {
      const ms = await this.deps.fetchMatches(this.table, this.seat);
      info = ms.find((m) => m.state === 'live') ?? [...ms].sort((a, b) => b.match - a.match)[0];
    } catch (e) {
      return { ok: false, reason: `could not list this table's matches (${e instanceof Error ? e.message : String(e)})` };
    }
    if (!info) return { ok: false, reason: 'the table has no match yet' };
    let first: EngineMessage[];
    try {
      first = await this.deps.transport.fetchState(this.table, info.match, this.seat.token);
    } catch (e) {
      const why = e instanceof WireError ? e : null;
      if (why && why.status === 404) return { ok: false, reason: `ManaBrew is not enabled on this server (${why.message})` };
      if (why && (why.status === 401 || why.status === 403)) return { ok: false, reason: `the ManaBrew route refused this seat (${why.message})` };
      return { ok: false, reason: `the ManaBrew route did not answer (${e instanceof Error ? e.message : String(e)})` };
    }
    if (!first.some((m) => m.kind === 'state')) return { ok: false, reason: 'the ManaBrew route answered without a game state' };
    if (this.closed) return { ok: true };
    this.seats = info.seats;
    this.match = info.match;
    this.dvr = dvrReducer(this.dvr, { type: 'snapshot', match: `${this.table}/${info.match}`, head: 0, turnStarts: [] });
    for (const m of first) this.handle(m);
    this.stream = this.deps.transport.openStream(streamURL(this.table, info.match, this.seat.token), (m) => this.handle(m), () => this.onStreamError());
    return { ok: true };
  }

  private onStreamError() {
    // EventSource reconnects by itself and the server re-sends the full
    // state and open prompt on connect; say so only if nothing arrives.
    if (!this.closed && this.view === null) this.notice = 'ManaBrew stream interrupted; reconnecting…';
  }

  /** handle consumes one engine message (exported for tests through the class). */
  handle(m: EngineMessage) {
    switch (m.kind) {
      case 'state':
        this.acceptState(m.gameView);
        break;
      case 'stateDelta': {
        if (this.gv === null) {
          void this.resync('a patch arrived before any state');
          break;
        }
        try {
          this.acceptState(applyPatch(this.gv, m.patch));
        } catch (e) {
          void this.resync(e instanceof PatchError ? e.message : String(e));
        }
        break;
      }
      case 'prompt':
        this.acceptPrompt(m);
        break;
      case 'error':
        this.noteRefusal(m.error.code, m.error.message);
        break;
      case 'display':
        break; // WIP upstream, not authoritative
    }
  }

  private async resync(why: string) {
    if (this.match === null) return;
    try {
      const msgs = await this.deps.transport.fetchState(this.table, this.match, this.seat.token);
      for (const m of msgs) if (m.kind !== 'stateDelta') this.handle(m);
    } catch (e) {
      this.notice = `ManaBrew resync failed after ${why}: ${e instanceof Error ? e.message : String(e)}`;
    }
  }

  private projected(gv: GameViewDto): View {
    return this.decorate(projectView(gv, { viewer: this.seat.seat, known: this.known }));
  }

  /**
   * decorate lays the open prompt over a projected board: the decision, and
   * the seat's potential plays. ManaBrew carries no potential_actions, but
   * an announce-able cast (`pay-<id>`) is exactly one — a spell the engine
   * would cast once mana is paid — so the autopilot's smart stops and the
   * float-then-cast affordances see the same "you have a play" the native
   * projection gives them.
   */
  private decorate(v: View): View {
    const d = this.binding?.decision ?? null;
    const plays = (d?.payment_actions ?? []).map((a) => ({ kind: 'cast', obj: a.cast.object, label: a.label }));
    return {
      ...v,
      decision: d,
      players: v.players.map((p) => (p.seat === this.seat.seat ? { ...p, potential_actions: plays.length > 0 ? plays : undefined } : p)),
    };
  }

  private acceptState(gv: GameViewDto) {
    this.gv = gv;
    const prev = this.live;
    const next = this.projected(gv);
    rememberCards(next, this.known);
    this.live = next;
    if (prev === null) {
      this.pushRing(next);
      this.show(next, 'reset');
      return;
    }
    const transitions = diffViews(prev, next);
    const lines: string[] = [];
    if (next.turn !== prev.turn) lines.push(`Turn ${next.turn} — ${this.seatName(next.active)}`);
    for (const t of transitions) {
      const l = transitionLine(t, (s) => this.seatName(s));
      if (l !== null) lines.push(l);
    }
    this.advance(lines, prev, next, transitions);
  }

  private acceptPrompt(p: AgentPrompt) {
    // A restored ask is an undo landing: a lower seq, or the very ask this
    // seat just answered (an undo of its last answer, which the server sends
    // again under the same promptId).
    const rewound = p.promptId < this.lastPromptId || p.promptId === this.answeredPromptId;
    if (rewound) {
      // The board's history stays in the ring; the panel re-bases its seq fences.
      this.answeredPromptId = -1;
      this.onRewind?.();
      this.model.emit({ type: 'reset' });
    }
    this.lastPromptId = p.promptId;
    // Compat: a priority ask with nothing to do but make mana is passed
    // unseen, as ManaBrew's own engine never poses one. A rewound ask (an
    // undo) is always shown, so the undo lands where the player can see it.
    if (this.deps.compat && !rewound && nothingToDo(p)) {
      this.autoPassed += 1;
      if (this.binding !== null) this.withdraw();
      this.sendMsg(compatPass(p)).catch(() => {
        // sendMsg already put the failure on the notice banner.
      });
      return;
    }
    this.binding = bindPrompt(p, { view: this.live });
    if (this.live === null) return;
    this.live = this.decorate(this.live);
    this.replaceRingHead(this.live);
    if (this.dvr.live) this.assign(this.live);
  }

  // ---- SeatTransport -------------------------------------------------

  async postIntent(_t: string, _k: number, intent: Intent): Promise<void> {
    const b = this.binding;
    if (b === null || b.decision === null) throw new ApiError(409, 'stalePrompt', 'no ManaBrew prompt is open');
    let msg: ClientMessage;
    try {
      msg = b.respond(intent);
    } catch (e) {
      const text = e instanceof AnswerError ? e.message : String(e);
      this.notice = `ManaBrew: ${text}`;
      throw new ApiError(400, 'unsupported', text);
    }
    await this.sendMsg(msg);
    // Answered: withdraw that prompt, unless a newer one already arrived.
    if (this.binding?.prompt.promptId === b.prompt.promptId) this.withdraw();
  }

  async fetchPending(): Promise<Decision> {
    const d = this.binding?.decision ?? null;
    if (d === null) throw new ApiError(409, 'conflict', 'nothing is pending for this seat');
    return d;
  }

  async postUndo(): Promise<void> {
    const prompt = this.binding?.prompt ?? null;
    if (prompt?.input.type !== 'chooseAction') {
      if (this.match === null) throw new ApiError(409, 'conflict', 'no match');
      return postNativeUndo(this.table, this.match, this.seat);
    }
    await this.sendMsg(undoMessage(prompt));
  }

  private async sendMsg(msg: ClientMessage) {
    if (this.match === null) throw new ApiError(409, 'conflict', 'no match');
    // An undo request names the open ask without answering it.
    if (msg.kind === 'response' && msg.action.output.type !== 'restoreSnapshot') this.answeredPromptId = msg.promptId;
    try {
      await this.deps.transport.send(this.table, this.match, this.seat.token, msg);
    } catch (e) {
      // A refused answer leaves its ask open, so it never comes back as an undo.
      if (msg.kind === 'response' && this.answeredPromptId === msg.promptId) this.answeredPromptId = -1;
      if (e instanceof WireError) {
        this.noteRefusal(e.code, e.message);
        throw new ApiError(e.status, e.code, e.message);
      }
      throw e;
    }
  }

  /** A stale prompt is a race, not a fault: the prompt closed before the answer arrived, and the panel re-reads the open one. */
  private noteRefusal(code: string, message: string) {
    if (code !== 'stalePrompt') this.notice = `ManaBrew ${code}: ${message}`;
  }

  private withdraw() {
    this.binding = null;
    if (this.live) {
      this.live = this.decorate(this.live);
      if (this.dvr.live) this.assign(this.live);
    }
  }

  /** The currently open prompt (tests and diagnostics). */
  get openPrompt(): AgentPrompt | null {
    return this.binding?.prompt ?? null;
  }
}
