package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Melira, the Living Cure.
//
// "Exile Melira: Choose another target creature or artifact. When it's put
// into a graveyard this turn, return that card to the battlefield under its
// owner's control." gorge registers a ChangesZone delayed trigger over the
// remembered target (ValidCard$ Card.IsTriggerRemembered, Origin$
// Battlefield, Destination$ Graveyard, ThisTurn$ True); kgen lowers it to
// OpDelayedTrigger with C = opLeaveReturn, a leave-keyed record (delayed.when
// = dlyLeaveGrave) beside the earthbend ones: matched by object id as gorge
// remembers it, fired once when the object is put into a graveyard from the
// battlefield (fireLeave, after the dies/leaves triggers), kept across the
// end step (splitLeaveDelayed) and dropped as the next turn begins.
//
// "If you would get one or more poison counters, instead you get one poison
// counter and you can't get additional poison counters this turn"
// (ReplOnePoison): addPoison's one choke point; genState.noPoison bit p is
// the turn's CantPutCounter POISON effect on player p (gorge's
// ImmediateTrigger -> Effect), cleared by newTurnGen.

// DebugNoOnePoison drops Melira's poison replacement; the lockstep mutation
// test flips it and requires a divergence.
var DebugNoOnePoison bool

// onePoison is ReplOnePoison and the turn's CantPutCounter POISON on player
// p for n poison counters: none once p's turn effect exists; else one while
// p controls a Melira (and the effect starts).
//
//go:noinline
func (s *State) onePoison(k *prep, p uint8, n int) int {
	if DebugNoOnePoison {
		return n
	}
	if s.g.noPoison&(1<<p) != 0 {
		return 0
	}
	if !k.gen.anyOnePoison {
		return n
	}
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].ctrl != p || s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplOnePoison {
				s.g.noPoison |= 1 << p
				return 1
			}
		}
	}
	return n
}
