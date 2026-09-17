import type { Decision, PlayerView, View } from '../protocol';

/**
 * castable.ts answers ONE question for the auto-pass logic: could this seat
 * still act if it stopped and floated every mana its untapped sources could
 * produce?
 *
 * The answer is the SERVER'S, not this module's. The wire carries
 * PlayerView.potential_actions (view/view.go): the engine's own legal-offer
 * walk (rules/legal.go's legalActionsPriced — the exact code that builds a
 * priority decision's options, including command-zone and flashback casts,
 * non-mana activated abilities and land drops) priced against the hypothetical
 * pool the seat's untapped sources could produce (rules.PotentialMana,
 * unbounded for Indeterminate sources). An entry there is a play the engine
 * WOULD offer once the mana floated — never a promise one is offered right
 * now, because the engine prices a cast against the FLOATING pool only (the
 * float-then-cast payment model that motivates the whole module).
 *
 * This module deliberately re-derives NOTHING. Its predecessor priced hand
 * cards from printed `mana_cost` on the client and drifted from the engine on
 * every cost rule the engine owns: Thalia's live RaiseCost (the Jitte report,
 * fb-20260914T145022Z — the client stopped a window the engine correctly
 * refused), a Medallion's live ReduceCost (the client passed a window the
 * engine would act in), every activation without a {T} in its printed cost
 * (Equip, manland animation), command-zone and graveyard (flashback) casts,
 * Indeterminate mana sources priced at zero (Tron + Karn), and X spells
 * refused while the engine offers them at X=0 (CR 107.3b). Each was one more
 * hand-maintained cost rule; the structural fix is that there are no client
 * rules left to drift.
 *
 * Fails closed (false) whenever the view does not carry the seat's own
 * projection: a seat the view does not name, a spectator view, or another
 * seat's PlayerView (the projection is attached to the viewer's own seat only,
 * because the walk reads that seat's hidden zones — CR 400.2). Pure functions
 * only: no Svelte, no I/O, no wall clock.
 */

/** playerOf returns the view's PlayerView for a seat, or undefined when absent. */
function playerOf(view: View, seat: number): PlayerView | undefined {
  return view.players?.find((p) => p.seat === seat);
}

/**
 * castableAfterTap reports whether ANY real play (a cast, ability or land
 * drop) is potential for this seat — i.e. whether stopping would surface a
 * window the seat could act in once its mana floated. The kinds that are
 * never a play — the mana tap ("activate", every priority window offers
 * one), pass and concede — are absent from the projection by construction,
 * and isPotentialPlay whitelists the server's documented vocabulary, so an
 * unknown kind can never make a pure mana-tap window stop-worthy. It does
 * NOT consult whose
 * turn it is — the stop fires wherever the caller's own stop rules already
 * consult the step.
 */
export function castableAfterTap(view: View, seat: number): boolean {
  const p = playerOf(view, seat);
  return !!p?.potential_actions?.some((a) => isPotentialPlay(a));
}

/**
 * isPotentialPlay is the one kind test for "this projected action is a real
 * play": the whitelist of exactly the kinds the server's PotentialAction
 * contract names (decision.PotentialAction: "cast", "ability", "play_land")
 * — the mana tap ("activate"), pass and concede are absent from the
 * projection by construction. A whitelist, not a blacklist of the three
 * never-a-play kinds: an unknown or malformed kind must not be read as a
 * play and make a window stop-worthy (rv2c review).
 */
const POTENTIAL_PLAY_KINDS: ReadonlySet<string> = new Set(['cast', 'ability', 'play_land']);
function isPotentialPlay(a: { kind: string }): boolean {
  return POTENTIAL_PLAY_KINDS.has(a.kind);
}

/**
 * respondableAfterTap is the instant-speed sibling: it reports whether the
 * seat could still interact with something on the stack if it stopped and
 * tapped out — a potential CAST or ABILITY. The server's walk carries its own
 * timing gates: a sorcery-speed play (a sorcery cast, an Equip, a land drop)
 * is only ever potential in a sorcery window, which an opponent-object window
 * never is, so what survives here is exactly the instant-speed answer the
 * response rules want. A "play_land" potential action is deliberately not
 * enough: a land drop is never a response (CR 305 — lands are sorcery-speed
 * and cannot interact with a resolving spell).
 */
export function respondableAfterTap(view: View, seat: number): boolean {
  const p = playerOf(view, seat);
  return !!p?.potential_actions?.some((a) => a.kind === 'cast' || a.kind === 'ability');
}

/**
 * castablesAfterTap is castableAfterTap's descriptive twin (fb-20260916T225211Z):
 * the SAME projection castableAfterTap tests, returned as the human labels of
 * what would become playable -- the server's own offer label for every real
 * play (isPotentialPlay) in the seat's potential_actions, suffixed "(after
 * tapping)" so a stop note can say not just WHAT made the window stop-worthy
 * but WHY it was not already offered (the float-then-cast payment model: the
 * engine prices a cast against the floating pool only). It fails closed
 * (empty list) whenever the view does not carry the seat's own projection: a
 * seat the view does not name, a spectator view, or another seat's
 * PlayerView. It does NOT consult whose turn it is -- the stop fires
 * wherever the caller's own stop rules already consult the step. The
 * `decision` parameter is accepted (unused) so this keeps the same call
 * shape as its callers (autopilot.ts's actionables).
 */
export function castablesAfterTap(view: View, seat: number, _decision?: Decision): string[] {
  const p = playerOf(view, seat);
  const out: string[] = [];
  for (const a of p?.potential_actions ?? []) {
    if (isPotentialPlay(a) && a.label) out.push(`${a.label} (after tapping)`);
  }
  return out;
}
