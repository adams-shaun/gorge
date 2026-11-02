package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Madness (CR 702.35a-b; gorge rules/altcast.go, replacement.go,
// trigger_queue.go, stack.go): a discard of a madness card (every discard
// path reaches discardObj) puts it into exile instead, and queues a
// kernel-native keyword trigger (ab abMadness, source and object the
// card; gorge's "__kwMadnessCast", stack ability 255). As that trigger
// resolves, its owner is asked (gorge askMadnessCast, KTriggerOptional:
// ChOptional) to cast the card for its madness cost -- "yes" offered only
// when the card is castable -- or not: "no" puts it into the graveyard.
// The cast is begun once the trigger has left the stack, ignoring timing;
// it carries trigPlMadness (stackEntry.trigPl, which no spell otherwise
// uses beside trigPlSneak), which prices it at CardDef.MadnessCost and pays
// it through gorge's mana window (a cast begun mid-resolution has no
// payment witness).
const abMadness uint8 = 251

// trigPlMadness is in trigpl.go.

// debugMadnessMutate is the test-only mutation switch of
// TestPauperkwMadnessLockstepMutation: 1 lets the planner pay a madness
// cast at the mana step (gorge pays it through its mana window).
var debugMadnessMutate uint8

// SetDebugMadnessMutate sets the mutation switch; tests only.
func SetDebugMadnessMutate(m uint8) { debugMadnessMutate = m }

const raGenMadness = raGenManifestDread + 50 // the madness cast-or-graveyard ask (ChOptional)

// madnessExiled reports whether discarding hand card o exiles it (printed
// madness).
func (s *State) madnessExiled(k *prep, o Obj) bool {
	return k.gen != nil && k.gen.anyMadness && k.t.Cards[s.objs[o].card].HasMadness
}

// queueMadness queues the madness trigger of exiled card o.
//
//go:noinline
func (s *State) queueMadness(o Obj) {
	ob := &s.objs[o]
	p := ob.owner()
	s.enqueue(o, ob.ver, ob.card, abMadness, p, o, p, 0)
}

// resolveMadness poses the cast-or-graveyard ask, or finishes when the card
// is no longer in exile (gorge: the ability simply finishes).
//
//go:noinline
func (s *State) resolveMadness(k *prep) {
	e := &s.res.e
	if o := e.obj; s.objs[o].zone != kop.ZoneExile || s.objs[o].ver != e.srcVer {
		s.finishResolve(k, false)
		return
	}
	s.res.ask, s.res.askP = raGenMadness, e.ctrl
	s.ctl = ctlResolve
}

// madnessCastable: the owner can cast exiled card o for its madness cost
// (gorge offerCastable): the cost payable and every target slot feasible.
func (s *State) madnessCastable(k *prep, o Obj, p uint8) bool {
	c := s.objs[o].card
	cost := s.adjustCost(k, p, c, o, k.t.Cards[c].MadnessCost)
	return (s.affordsFor(k, p, &cost) || s.windowAffords(k, p, &cost)) && s.spellTargetsFeasible(k, c, p)
}
