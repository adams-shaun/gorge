package botpolicy

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/decision"
)

// worthAbilities lists the offered "ability" options abilityScore rates
// worth taking (A1 and A5 applied), in option order.
func (b Board) worthAbilities(d *decision.Decision) []int {
	var out []int
	for _, o := range d.Options {
		if o.Kind != "ability" {
			continue
		}
		if _, worth := b.abilityScore(o, d.Player); worth {
			out = append(out, o.Index)
		}
	}
	return out
}

// exploreAbility is X2 (ExploreDecide): a uniform pick, over the seat's rng,
// among the worth-taking abilities, or -1 when none is. The rng is consumed
// only when there is something to pick, and the pick is a pure function of
// the options and the rng state, so the policy stays deterministic.
func (b Board) exploreAbility(d *decision.Decision, r *rand.Rand) int {
	w := b.worthAbilities(d)
	if len(w) == 0 {
		return -1
	}
	return w[r.IntN(len(w))]
}

// exploreEarlyAbility is X3/X4 (ExploreDecide): with probability 1/3 in a
// main phase (before the land drop and the cast), or 1/4 at any other
// priority window, activate a uniformly chosen worth-taking ability.
// Returns -1 when the coin says no or nothing is worth taking.
func (b Board) exploreEarlyAbility(d *decision.Decision, r *rand.Rand) int {
	w := b.worthAbilities(d)
	if len(w) == 0 {
		return -1
	}
	n := 4
	switch {
	case b.IsMain:
		n = 3
	case b.Pool.Total() > 0:
		// Outside a main phase floating mana is only ever X6's own float,
		// which exists to pay for exactly this: spend it.
		n = 1
	}
	if r.IntN(n) != 0 {
		return -1
	}
	return w[r.IntN(len(w))]
}

// exploreFloat is X6 (ExploreDecide): the engine offers an ability only
// once the FLOATING pool can pay it (the float-then-act payment model,
// rules/legal.go), and the production tap gate (chooseTap) floats mana only
// toward a castable card, in a main phase. So an ability costing mana was
// offered only when a cast happened to leave enough floating -- never
// outside a main phase (an upkeep-only or combat-only ability, a response)
// and never for a cost above the leftover (Tower of Eons' {8}). X6 floats
// mana speculatively: with an empty pool, with probability 1/3 in a main
// phase or 1/6 elsewhere, it taps one plain {T} mana source; once the pool
// holds mana it keeps tapping (every tap is a new priority window, where
// X2/X4 can spend the pool on a now-affordable ability). Only bare-tap
// "activate" options are taken (Option.Cost empty), so a float never
// sacrifices, pays life or loops a free repeatable ability, and it ends when
// the untapped sources do. Returns -1 when it does not tap.
func (b Board) exploreFloat(d *decision.Decision, r *rand.Rand) int {
	var taps []int
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Cost == "" {
			taps = append(taps, o.Index)
		}
	}
	if len(taps) == 0 {
		return -1
	}
	if b.Pool.Total() == 0 {
		n := 6
		if b.IsMain {
			n = 3
		}
		if r.IntN(n) != 0 {
			return -1
		}
	}
	return taps[r.IntN(len(taps))]
}

// exploreManaPick is X5 (ExploreDecide): when every option of d is a "mana"
// option, a uniform pick over the seat's rng; ok is false for any other
// shape.
func exploreManaPick(d *decision.Decision, r *rand.Rand) (int, bool) {
	for _, o := range d.Options {
		if o.Kind != "mana" {
			return 0, false
		}
	}
	return d.Options[r.IntN(len(d.Options))].Index, true
}
