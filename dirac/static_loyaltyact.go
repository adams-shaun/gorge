package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugNumLoyaltyActOff is a mutation switch: a StatNumLoyaltyAct static is
// ignored, so the limit stays CR 606.3's one activation. Only the mutation
// test sets it.
var DebugNumLoyaltyActOff bool

// useNumLoyaltyAct opens the generated forms for a table carrying the static
// and sizes its per-slot cold region (coldLoyaltyAct). A table without it keeps
// no region, so State itself does not grow (TestStateLayoutPinned).
var useNumLoyaltyAct = newGenUse(func(t *kop.Table) bool {
	return tableHasStatic(t, kop.StatNumLoyaltyAct)
})

// coldLoyaltyAct is each battlefield slot's loyalty abilities activated this
// turn: a perSlot region, zeroed when the slot takes a new permanent (CR 400.7)
// and at each turn's start (turn.go). A table without StatNumLoyaltyAct sizes
// it to nothing.
var coldLoyaltyAct = newCold[uint8]("loyaltyAct", func(g *genPrep) int {
	if g != nil && g.uses.has(useNumLoyaltyAct) {
		return MaxPerm
	}
	return 0
}).perSlot()

// loyaltyActUsed is sl's loyalty activations this turn.
func (s *State) loyaltyActUsed(sl Slot) int {
	c := coldLoyaltyAct.of(s)
	if int(sl) >= len(c) {
		return 0
	}
	return int(c[sl])
}

// noteLoyaltyActivation records a loyalty ability of sl as activated, beside
// the pfLoyaltyUsed bit (CR 606.3): the bit keeps the object's change hash and
// the pre-static read, the count is what a Twice$ static raises the limit for.
func (s *State) noteLoyaltyActivation(k *prep, sl Slot) {
	if sl == NoSlot {
		return
	}
	s.perms[sl].flags |= pfLoyaltyUsed
	if k == nil || k.gen == nil || !k.gen.uses.has(useNumLoyaltyAct) {
		return
	}
	if c := coldLoyaltyAct.of(s); int(sl) < len(c) && c[sl] < 255 {
		c[sl]++
	}
}

// resetLoyaltyActivations clears sl's count at the turn change; the existing
// loop clears pfLoyaltyUsed and marks the object.
func (s *State) resetLoyaltyActivations(sl Slot) {
	if c := coldLoyaltyAct.of(s); int(sl) < len(c) {
		c[sl] = 0
	}
}

// loyaltyUsedAtLimit reports whether sl may not activate another loyalty
// ability now: CR 606.3's one, raised to two by a live StatNumLoyaltyAct static
// whose filter matches sl (gorge rules/legal_loyalty.go loyaltyAbilityLimit's
// Twice$ arm; a second Twice$ static does not compound).
func (s *State) loyaltyUsedAtLimit(k *prep, sl Slot) bool {
	if sl == NoSlot {
		return false
	}
	if k == nil || k.gen == nil || !k.gen.uses.has(useNumLoyaltyAct) {
		return s.perms[sl].flags&pfLoyaltyUsed != 0
	}
	limit := 1
	if !DebugNumLoyaltyActOff && s.staticFlagGen(k, kop.StatNumLoyaltyAct, sl) {
		limit = 2
	}
	return s.loyaltyActUsed(sl) >= limit
}
