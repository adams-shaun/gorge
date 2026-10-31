import type { Decision, Intent, Option, PaymentAction } from '../../protocol';
import type { HumanExtension, NativeOp, Sbv2Request, ViewPayload } from './wire';

/**
 * answer.ts joins the seat panel's world to v2's. The panel answers one native
 * decision with one Intent; v2 asks a substep at a time, one candidate each,
 * and x_gorge_view_v1's ops say what each candidate means natively. A Walk
 * holds what the Intent still has to say, and nextAnswer picks the candidate
 * for the substep at hand.
 */

export interface Walk {
  nativeIndex: number;
  actionId: string | null;
  choices: number[];
  rest: number[];
  /** The choices no candidate has spoken for yet. */
  remaining: number[];
  /** The follow-up decision the Intent answered, when it isn't the pose's own. */
  followup: string | null;
}

export type Answer = { candidate: number } | { name: string };

export function startWalk(intent: Intent, payload: ViewPayload, followup: string | null = null): Walk {
  const choices = intent.choices;
  return {
    nativeIndex: payload.native_index,
    actionId: intent.payment?.action_id ?? intent.announce?.action_id ?? null,
    choices,
    // An Intent without Rest means the unchosen options in offered order.
    rest: intent.rest ?? payload.decision.options.map((o) => o.index).filter((i) => !choices.includes(i)),
    remaining: [...choices],
    followup,
  };
}

/** A partition candidate's destination is inside the native Choices when the op says so, or by its name. */
function insideChoices(op: NativeOp): boolean {
  return op.inside ?? (op.list === 'top' || op.list === 'hand');
}

/**
 * nextAnswer is the answer to the substep payload poses, consuming what it
 * speaks for. Null means the Intent doesn't settle it.
 */
export function nextAnswer(w: Walk, payload: ViewPayload): Answer | null {
  const ops = payload.ops ?? [];
  const found = (test: (op: NativeOp) => boolean): Answer | null => {
    const i = ops.findIndex(test);
    return i >= 0 ? { candidate: i } : null;
  };

  if (w.actionId !== null) {
    const paid = found((op) => op.payment?.action_id === w.actionId);
    if (paid) return paid;
  }
  for (const [at, choice] of w.followup === null ? w.remaining.entries() : []) {
    const chosen = found((op) => op.op === 'choose' && op.option === choice);
    if (chosen) {
      w.remaining.splice(at, 1);
      return chosen;
    }
    const cast = found((op) => op.op === 'cast' && (op.covers ?? []).includes(choice));
    if (cast) return cast;
    const option = payload.decision.options.find((o) => o.index === choice);
    if (option?.kind === 'name') {
      w.remaining.splice(at, 1);
      return { name: option.label };
    }
  }
  const lists: Record<string, number[]> = { choices: w.choices, rest: w.rest };
  if (w.followup !== null) lists[`followup:${w.followup}`] = w.choices;
  const placed = found((op) => op.op === 'list' && lists[op.list ?? '']?.[op.position ?? 0] === op.option);
  if (placed) return placed;
  if (ops[0]?.op === 'dest') {
    const wanted = w.choices.includes(ops[0].option);
    const dest = found((op) => op.op === 'dest' && insideChoices(op) === wanted);
    if (dest) return dest;
  }
  const declined = found((op) => op.op === 'none');
  if (declined) return declined;
  if (w.remaining.length === 0) {
    const finished = found((op) => op.op === 'finish');
    if (finished) return finished;
  }
  return ops.length === 1 ? { candidate: 0 } : null;
}

/**
 * panelDecision is the native decision the seat panel answers. A priority
 * decision keeps what a candidate can do, plus taps, which the source posts on
 * the native route because v2 hides them: an alternate cost collapses into its
 * printed one, and a cast pays with the one plan its candidate stands for.
 */
export function panelDecision(payload: ViewPayload, seq: number): Decision {
  const d = payload.decision;
  if (d.kind !== 'priority') return { ...d, seq };
  const ops = payload.ops ?? [];
  const offered = new Set(ops.flatMap((op) => [op.option, ...(op.covers ?? [])]));
  const plans = new Map(ops.flatMap((op) => (op.payment ? [[op.payment.action_id, op.payment.plan] as const] : [])));
  const paymentActions = (d.payment_actions ?? []).flatMap((a): PaymentAction[] => {
    const plan = plans.get(a.id);
    return plan ? [{ ...a, plans: [plan] }] : [];
  });
  return {
    ...d,
    seq,
    options: d.options.filter((o: Option) => o.kind === 'concede' || o.kind === 'activate' || offered.has(o.index)),
    ...(paymentActions.length > 0 ? { payment_actions: paymentActions } : { payment_actions: undefined }),
  };
}

/**
 * followupOf names the later native decision a substep is ordering, if it
 * carries one: the order of a dig's remainder, which the native wire asks as a
 * decision of its own. v2 folds it into the dig's transaction.
 */
export function followupOf(payload: ViewPayload): string | null {
  const key = payload.ops?.[0]?.list?.match(/^followup:(.+)$/)?.[1];
  return key !== undefined && payload.followups?.[key] ? key : null;
}

function cardNameOf(ref: unknown): string | null {
  return (ref as { card_name?: string | null } | undefined)?.card_name ?? null;
}

function semanticLabel(semantic: { kind: string } & Record<string, unknown>): string {
  const item = semantic.item as { object?: unknown } | undefined;
  const card = cardNameOf(semantic.card) ?? cardNameOf(item?.object);
  const destination = typeof semantic.destination === 'string' ? ` to ${semantic.destination}` : '';
  return card !== null ? `${card}${destination}` : semantic.kind.replaceAll('_', ' ');
}

/**
 * substepDecision shows one substep's candidates as they are, for a substep no
 * Intent speaks for: its answer is the candidate id itself.
 */
export function substepDecision(r: Sbv2Request, human: HumanExtension, seq: number): Decision {
  const d = r.decision.extensions.x_gorge_view_v1.decision;
  const options = (r.decision.candidates ?? []).map((c, index) => ({
    index,
    player: d.player,
    kind: 'choice',
    label: human.labels[String(c.candidate_id)] || semanticLabel(c.semantic),
  }));
  return { seq, player: d.player, kind: 'choose', prompt: d.prompt, min: 1, max: 1, options } as Decision;
}
