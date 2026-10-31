import type { Decision, Intent, MatchInfo, View } from '../../protocol';
import { ApiError, fetchEvents, fetchMatches, fetchView, postIntent, postNativeUndo } from '../api';
import { diffViews } from '../clientmodel/snapshotdiff';
import { isHiddenKind } from '../logfilter';
import { dvrReducer } from '../dvr';
import type { SeatCtx } from '../seat';
import { WireError } from '../wire/http';
import { SnapshotMatch, type StartResult } from '../wire/snapshot.svelte';
import { followupOf, nextAnswer, panelDecision, startWalk, substepDecision, type Answer, type Walk } from './answer';
import { httpTransport, streamURL, type Transport } from './transport';
import type { Sbv2Request, Sbv2Send } from './wire';

/** How soon, and then how often, a seat with no open request asks whether the match ended. */
const END_FIRST_CHECK = 500;
const END_CHECK = 2000;

/** kindOfLine tells the kinds the log hides by default from their spelling, all a v2 log line carries. */
function kindOfLine(line: string): string | null {
  if (/^(You have|.+ has) priority$/.test(line)) return 'priority';
  if (/ (is|are) asked: /.test(line)) return 'decision_ask';
  if (/^(You answer|.+ answers) \S+$/.test(line)) return 'decision_made';
  if (line.startsWith('Step: ')) return 'step';
  if (line === 'Combat ends') return 'end_combat_reset';
  return null;
}

/** The decision shown to the panel, and how its answer travels. */
interface Asked {
  decision: Decision;
  request: Sbv2Request;
  how: 'walk' | 'candidate' | 'native';
  /** The follow-up decision shown instead of the pose's own. */
  followup: string | null;
}

interface Deps {
  transport: Transport;
  fetchMatches: (t: string, ctx?: SeatCtx) => Promise<MatchInfo[]>;
  fetchView: (t: string, k: number, seq?: number, ctx?: SeatCtx) => Promise<View>;
}

/**
 * Sbv2Match is the table route's game source when the protocol setting is
 * SpellBench v2. Each request carries the native view in x_gorge_view_v1, so it
 * only has to follow requests: the board, the decision and the log lines all
 * arrive with them. Between requests the board stands still, and the routes
 * have no terminal message, so a match's end is read from the native routes.
 */
export class Sbv2Match extends SnapshotMatch {
  readonly protocol = 'sbv2' as const;
  readonly autoPayOnly = true;
  private deps: Deps;
  private open: Sbv2Request | null = null;
  private asked: Asked | null = null;
  private walk: Walk | null = null;
  private nativeIndex = -1;
  private answeredIndex = -1;
  private lastRequest = 0;
  private lastLine: string | null = null;
  /** The live board came from the native routes, so its object ids are native. */
  private nativeBoard = false;
  private endTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(table: string, seat: SeatCtx, deps: Partial<Deps> = {}) {
    super(table, seat);
    this.deps = { transport: deps.transport ?? httpTransport, fetchMatches: deps.fetchMatches ?? fetchMatches, fetchView: deps.fetchView ?? fetchView };
  }

  /**
   * start finds the table's live match and probes the state route. A table v2
   * doesn't cover answers 409 and resolves {ok:false, quiet:true}; a server
   * without -sbv2 or a refused token resolves {ok:false} with a reason. Either
   * leaves nothing open, so the route can fall back to Native.
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
    let first: Sbv2Request | null;
    try {
      first = await this.deps.transport.fetchState(this.table, info.match, this.seat.token);
    } catch (e) {
      const why = e instanceof WireError ? e : null;
      if (why?.status === 409) return { ok: false, quiet: true, reason: 'SpellBench v2 covers two-player constructed tables with automatic mana' };
      if (why?.status === 404) return { ok: false, reason: `SpellBench v2 is not enabled on this server (${why.message})` };
      if (why?.status === 401 || why?.status === 403) return { ok: false, reason: `the SpellBench v2 route refused this seat (${why.message})` };
      return { ok: false, reason: `the SpellBench v2 route did not answer (${e instanceof Error ? e.message : String(e)})` };
    }
    if (this.closed) return { ok: true };
    this.seats = info.seats;
    this.match = info.match;
    this.dvr = dvrReducer(this.dvr, { type: 'snapshot', match: `${this.table}/${info.match}`, head: 0, turnStarts: [] });
    if (first) this.accept(first);
    else await this.showNativeBoard();
    if (this.closed || this.live?.over) return { ok: true };
    this.stream = this.deps.transport.openStream(streamURL(this.table, info.match, this.seat.token), (r) => this.accept(r), () => this.onStreamError());
    return { ok: true };
  }

  override close() {
    super.close();
    if (this.endTimer !== null) clearTimeout(this.endTimer);
  }

  private onStreamError() {
    // EventSource reconnects by itself and the server re-serves the open
    // request; say so only if nothing has arrived.
    if (!this.closed && this.view === null) this.notice = 'SpellBench v2 stream interrupted; reconnecting…';
  }

  /** accept consumes one state document or stream frame. */
  accept(r: Sbv2Request | null) {
    if (r === null) {
      this.open = null;
      if (this.walk === null) this.withdraw();
      this.watchForEnd(END_FIRST_CHECK);
      return;
    }
    if (r.request_id === this.open?.request_id) return; // a reconnect re-serves what's open
    if (this.endTimer !== null) clearTimeout(this.endTimer);
    this.open = r;
    const human = r.decision.extensions.x_gorge_human_v1;
    const payload = r.decision.extensions.x_gorge_view_v1;
    const seq = Number(r.request_id);
    const index = human.native_only ? payload.decision.seq : payload.native_index;
    // An undo re-asks an earlier native decision, under fresh object ids.
    const rewound = seq < this.lastRequest || index < this.nativeIndex;
    const sameDecision = index === this.nativeIndex;
    if (rewound) {
      this.onRewind?.();
      this.walk = null;
      this.answeredIndex = -1;
    }
    this.lastRequest = seq;
    this.nativeIndex = index;
    const walking = this.walk !== null && this.walk.nativeIndex === index;
    if (!walking) this.walk = null;

    let answer: Answer | null = null;
    if (walking) {
      answer = nextAnswer(this.walk!, payload);
      if (answer === null) this.walk = null;
    }
    if (answer !== null) {
      this.asked = null;
      this.paint(payload.view, null, human.log, rewound || this.nativeBoard);
      this.nativeBoard = false;
      void this.sendAnswer(r, answer).catch(() => {
        // sendMsg already put the failure on the notice banner.
        this.walk = null;
      });
      return;
    }
    // A substep with no Intent behind it (a reload mid-decision, an answer the
    // last Intent didn't settle) is shown as the follow-up decision it orders,
    // or else as its candidates.
    const midDecision = !human.native_only && (walking || (r.decision.group?.substep_index ?? 0) > 0 || (sameDecision && index === this.answeredIndex));
    const followup = midDecision ? followupOf(payload) : null;
    let decision: Decision;
    if (followup !== null) decision = { ...payload.followups![followup], seq };
    else if (midDecision) decision = substepDecision(r, human, seq);
    else decision = human.native_only ? { ...payload.decision, seq } : panelDecision(payload, seq);
    this.asked = { decision, request: r, how: human.native_only ? 'native' : midDecision && followup === null ? 'candidate' : 'walk', followup };
    // A native-only request's view has native ids, so nothing carries across.
    this.paint(payload.view, decision, human.log, rewound || human.native_only || this.nativeBoard);
    this.nativeBoard = human.native_only;
  }

  private paint(board: View, decision: Decision | null, lines: string[], reset: boolean) {
    const next: View = { ...board, decision };
    const prev = this.live;
    this.live = next;
    if (lines.length > 0) this.lastLine = lines[lines.length - 1];
    if (prev === null || reset) {
      for (const line of lines) this.log(line, next);
      this.pushRing(next);
      this.show(next, 'reset');
      return;
    }
    this.advance(lines, prev, next, diffViews(prev, next));
  }

  private withdraw() {
    this.asked = null;
    if (this.live?.decision) {
      this.live = { ...this.live, decision: null };
      this.replaceRingHead(this.live);
      if (this.dvr.live) this.assign(this.live);
    }
  }

  private watchForEnd(delay: number) {
    if (this.endTimer !== null) clearTimeout(this.endTimer);
    this.endTimer = setTimeout(() => void this.checkForEnd(), delay);
  }

  private async checkForEnd() {
    if (this.closed || this.open !== null || this.match === null) return;
    const ms = await this.deps.fetchMatches(this.table, this.seat).catch(() => []);
    if (this.closed || this.open !== null) return;
    const info = ms.find((m) => m.match === this.match);
    if (info && info.state !== 'live') await this.showNativeBoard();
    else this.watchForEnd(END_CHECK);
  }

  /**
   * showNativeBoard paints the native view when no request is open to carry
   * one: a table loaded between decisions, or a finished match. Its lines up to
   * the last one this seat was shown are already in the log, with v2's spelling.
   */
  private async showNativeBoard() {
    if (this.match === null) return;
    let board: View;
    try {
      board = await this.deps.fetchView(this.table, this.match, undefined, this.seat);
    } catch (e) {
      this.notice = `SpellBench v2 could not read the board (${e instanceof Error ? e.message : String(e)})`;
      return;
    }
    if (this.closed || this.open !== null) return;
    const tail = board.over ? await this.finalLines() : [];
    this.paint(board, null, tail, true);
    this.nativeBoard = true;
    if (board.over) {
      this.stream?.close();
      this.stream = null;
    }
  }

  private async finalLines(): Promise<string[]> {
    const last = this.lastLine;
    if (this.match === null || last === null) return [];
    const events = await fetchEvents(this.table, this.match, 0, this.seat).catch(() => []);
    const plain = (line: string) => line.replace(/ #\d+\b/g, '').split(':')[0];
    const at = events.map((e) => plain(e.line)).lastIndexOf(plain(last));
    return at < 0 ? [] : events.slice(at + 1).filter((e) => !isHiddenKind(e.event.kind)).map((e) => e.line.replace(/ #\d+\b/g, ''));
  }

  protected override lineKind(line: string): string {
    return kindOfLine(line) ?? this.protocol;
  }

  private async sendAnswer(r: Sbv2Request, answer: Answer) {
    await this.sendMsg('name' in answer
      ? { request_id: r.request_id, x_gorge_human_v1: { choose_name: answer.name } }
      : { request_id: r.request_id, candidate_id: answer.candidate });
  }

  private async sendMsg(msg: Sbv2Send) {
    if (this.match === null) throw new ApiError(409, 'conflict', 'no match');
    try {
      await this.deps.transport.send(this.table, this.match, this.seat.token, msg);
    } catch (e) {
      if (e instanceof WireError) {
        // stale is the race the protocol documents: the request closed first.
        if (e.code !== 'stale') this.notice = `SpellBench v2 ${e.code}: ${e.message}`;
        throw new ApiError(e.status, e.code, e.message);
      }
      throw e;
    }
  }

  // ---- SeatTransport -------------------------------------------------

  async postIntent(t: string, k: number, intent: Intent): Promise<void> {
    const asked = this.asked;
    if (asked === null) throw new ApiError(409, 'conflict', 'no SpellBench v2 request is open');
    if (intent.seq !== asked.decision.seq) throw new ApiError(409, 'conflict', `answer for ${intent.seq}, but the open request is ${asked.decision.seq}`);
    const payload = asked.request.decision.extensions.x_gorge_view_v1;
    const requestId = asked.request.request_id;
    const kind = asked.decision.options.find((o) => o.index === intent.choices[0])?.kind;
    if (asked.how === 'native' || kind === 'activate') {
      await postIntent(t, k, { ...intent, seq: payload.decision.seq }, this.seat);
    } else if (asked.how === 'candidate') {
      this.answeredIndex = payload.native_index;
      await this.sendMsg({ request_id: requestId, candidate_id: intent.choices[0] });
    } else if (kind === 'concede') {
      await this.sendMsg({ request_id: requestId, x_gorge_human_v1: { concede: true } });
    } else {
      const walk = startWalk(intent, payload, asked.followup);
      const answer = nextAnswer(walk, payload);
      if (answer === null) throw new ApiError(400, 'unsupported', 'SpellBench v2 has no candidate for this answer');
      this.walk = walk;
      this.answeredIndex = payload.native_index;
      await this.sendAnswer(asked.request, answer);
    }
    if (this.asked?.request.request_id === requestId) this.withdraw();
  }

  async fetchPending(): Promise<Decision> {
    if (this.asked === null) throw new ApiError(409, 'conflict', 'nothing is pending for this seat');
    return this.asked.decision;
  }

  async postUndo(t: string, k: number): Promise<void> {
    const open = this.open;
    if (open === null || open.decision.extensions.x_gorge_human_v1.native_only) return postNativeUndo(t, k, this.seat);
    await this.sendMsg({ request_id: open.request_id, x_gorge_human_v1: { undo: true } });
  }

  /** The request this seat is being asked (tests and diagnostics). */
  get openRequest(): Sbv2Request | null {
    return this.open;
  }
}
