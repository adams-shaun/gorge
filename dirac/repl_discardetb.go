package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplDiscardOppToBattlefield (Wilt-Leaf Liege): a card discarded by a
// resolving spell or ability an opponent of its owner controls enters the
// battlefield under its owner's control instead of the graveyard. Gorge's
// ReplMoved arm (replacement_match.go) excludes cost discards and cleanup
// discards (no resolving object), and ValidCause$ SpellAbility.OppCtrl reads
// the resolving object's controller. No State bytes: the resolving entry is
// s.res.e, live only while a resolution runs.

// discardCost discards o as a cost: no replacement applies.
func (s *State) discardCost(k *prep, o Obj) { s.discardObjAs(k, o, false) }

// discardUnlessPay discards o to pay an "unless" cost during a resolution. CR
// 118.12: the resolving object makes that payment, so it is its discard, not
// a cost discard: gorge's pay/unlessflow.go emits events.Discard (Forge's
// CostDiscard: "if this is caused by 118.12 it's also an effect"), and an
// opponent's unless puts Wilt-Leaf Liege onto the battlefield. A payer who
// controls the resolving object discards to the graveyard as before.
func (s *State) discardUnlessPay(k *prep, o Obj) { s.discardObjAs(k, o, true) }

// discardToBattlefield puts o, just detached from hand, onto the battlefield
// when it carries the replacement and an opponent's resolution discards it.
func (s *State) discardToBattlefield(k *prep, o Obj, from kop.Zone) bool {
	if k.gen.replBits[s.objs[o].card]&rbDiscETB == 0 || !s.res.active {
		return false
	}
	owner := s.objs[o].owner()
	if s.res.e.ctrl == owner {
		return false
	}
	s.enter(k, o, owner, false, from)
	return true
}
