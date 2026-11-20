package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// witnessGrantCol is the colour a witness-plan unit at sl makes when the
// source itself gains a basic land type (Urborg, Tomb of Yawgmoth makes
// Luxury Suite a Swamp beside its own "Add {B} or {R}"). Gorge's witness
// search carries the granted type's activation as its own unit — a plan
// through the intrinsic basic-land ability — beside the printed one, and the
// final tie-break reads a step's ability kind before what it produces
// (pay.CompareSteps), where "intrinsic" sorts before "printed".
//
// A source whose own mana abilities gorge calls printed (a nonbasic's own
// Production, blgPrintedMask) therefore plans through the granted ability
// whenever it can pay: Luxury Suite floats the granted Swamp's {B}, not the
// planner's preference {R}. Its own abilities' colours answer only when the
// granted colour strands the plan, in the planner's preference order, which
// is the same Produces order gorge's tie-break reads.
//
// A basic land's own tap ability is itself an intrinsic basic-land ability
// (blgPrintedMask 0 for a basic), so granted and own plans share a kind and
// the tie is decided by the step's Produces vector: the earlier colour in
// the planner's preference order (C, then G, R, B, U, W) wins — a Mountain
// under Urborg keeps its own {R} over the granted {B}, a Plains under one
// takes the granted {B} over its {W}. The grant view merges the two masks,
// so the answer is the planner's preference over it with no first colour
// (the view's blgFirst is the own colour, which the Produces order outranks
// whenever it is not the most preferred).
//
// A source without a grant keeps the planner's answer (ok false): Foul
// Orchard's own "Add {B} or {G}" keeps the planner's {G} there, and its
// staged repros record that colour (tail-32 pair 17).
func (s *State) witnessGrantCol(k *prep, p uint8, pk *planner, i int, ty *utype, rem *[kop.NumManaKinds]int, remAny int) (int, bool) {
	if k.gen == nil || ty.n != 1 || ty.rb || ty.inert || ty.pain != 0 || ty.sac || ty.sacOnly {
		// k.gen nil: the table has no land-type grants (the vanilla lab's
		// hand-written table), and blgMask/blgPrintedMask read k.gen. An
		// allocation is manual, a restricted unit floats by its own rule, and
		// a pain or sacrifice alternative is not the granted tap.
		return 0, false
	}
	sl := pk.units[pk.bestSeq[i]]
	if s.blgMask(k, sl) == 0 {
		return 0, false // the source gains no basic land type: the planner's answer stands
	}
	if s.blgPrintedMask(k, sl) != 0 {
		// the granted ability's plan outranks the printed ones (kind first)
		m := uint8(s.blgMask(k, sl)) & 0x1f
		for cc := kop.G; cc >= kop.W; cc-- {
			if m&(1<<uint(cc)) != 0 && ty.mask&(1<<uint(cc)) != 0 &&
				pk.colourKeepsPayable(int(cc), int(ty.n), rem, remAny, &pk.bestTy, i+1, pk.bestN) {
				return int(cc), true // the granted ability's plan wins the tie
			}
		}
		return 0, false // the granted colour strands the plan: the printed ones answer
	}
	// granted and own abilities share the intrinsic kind: the Produces order
	// decides (the planner's preference order)
	return pk.pickColour(ty.mask, 0, int(ty.n), rem, remAny, &pk.bestTy, i+1, pk.bestN), true
}
