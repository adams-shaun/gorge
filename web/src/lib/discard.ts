import type { CardView, Decision, Option } from '../protocol';
import { cardFromPick } from './arrange';

/**
 * discard.ts is the discard-pick ask's pure half (fb-20260914T120705Z). Four
 * engine shapes pose a decision whose options are CARD PICKS — Option.kind
 * "discard", Obj the card's id, Label the card's name (three of the four
 * prefix it with "Discard ", the cast-cost one does not):
 *
 *  - `Mode$ RevealYouChoose` (Thoughtseize, Duress): a KModes ask, the CASTER
 *    choosing from the TARGET player's redacted hand (effects/cardflow.go);
 *  - `Mode$ TgtChoose` (Mind Rot family): the same KModes ask over the
 *    discarding player's OWN hand;
 *  - the CR 514.1 cleanup-step discard (rules/combat.go cleanupStep): a
 *    KChoose over the active player's hand;
 *  - the cast-cost discard (rules/cast.go): a KChoose whose labels are the
 *    BARE card name.
 *
 * The decision is the payload that carries these cards — the target's hand is
 * redacted in the view, so the face must be synthesized from the option
 * (cardFromPick, shared with the arrange strip), exactly as the arrange
 * surface already does. The detection predicate and the label/name split live
 * here so the component stays a renderer and the next consumer of the same
 * vocabulary cannot fork the shape.
 */

/**
 * discardName extracts the card's name from a discard option's label: the
 * engine prefixes three of the four shapes with "Discard " (effects/cardflow.go
 * and rules/combat.go) and leaves the cast-cost shape bare (rules/cast.go), so
 * the strip must tolerate both — and must be exact, because the art proxy
 * (lib/images) and the oracle resolver (lib/oracle) both look the card up by
 * this exact string. "Discard two cards"-style labels are not card names and
 * never reach here: isDiscardPick has already gated the decision on every
 * option being a card pick with a set Obj.
 */
export function discardName(label: string): string {
  return label.startsWith('Discard ') ? label.slice('Discard '.length) : label;
}

/**
 * isDiscardPick reports whether a decision is a discard-pick ask: it has
 * options, and EVERY non-concede option is a card pick — kind "discard" with a
 * set Obj. A decision that mixes any other option kind (a modal `mode` list, a
 * pass option, an option with no object to show) stays on the generic text
 * list: the card-face row is only for asks where every pickable thing is a
 * card. `decision` comes in as the panel's pending-or-null decision, so null
 * is false by construction.
 */
export function isDiscardPick(d: Decision | null): boolean {
  if (d === null || d.options.length === 0) return false;
  const picks = d.options.filter((o) => o.kind !== 'concede');
  return picks.length > 0 && picks.every((o) => o.kind === 'discard' && o.obj !== undefined);
}

/**
 * discardCard synthesizes the CardView a discard option renders as a face —
 * the shared cardFromPick under the option label's card name (the label
 * stripped of its "Discard " prefix, where the engine put one). The name is
 * what both the art proxy and the oracle resolver look the card up by, so the
 * synthesized face gets the real printed face and the hover inspector gets
 * the real printed oracle text for free.
 */
export function discardCard(d: Decision, o: Option): CardView {
  return cardFromPick(d, o, discardName(o.label));
}
