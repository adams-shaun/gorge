package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a hand-to-battlefield move of one card whose filter may
// admit an Aura (CR 303.4f/g). gorge reaches the one gate every non-cast Aura
// entry shares (rules.auraEntryGate) from the hand walk's settleHandMove:
// with no legal object or player the Aura stays in the hand, with one it
// enters attached to it, with two or more its controller chooses as it
// enters (the "enchant" election, answered before the move folds).

// DebugHandAuraNoAttach enters such an Aura unattached (TestHandauraMutation):
// the state-based actions put it into the graveyard, where gorge has it
// attached.
var DebugHandAuraNoAttach bool

// handAuraEnter decides the picked card o's entry under ctrl. A card that is
// no Aura is none of this (host NoSlot, neither parked nor stay). stay: the
// Aura has no legal host and remains in the hand. parked: two or more hosts,
// the card waits in the hand and the resolution suspends at the raGenEnchant
// ask; its answer re-enters moveFoundAns with host answered. Otherwise host
// is what the Aura enters attached to (NoSlot for a non-Aura). The answer's
// host applies to the parked card only; the cards still to move ask again.
//
//go:noinline
func (s *State) handAuraEnter(k *prep, o Obj, ctrl uint8, answered Slot) (host Slot, parked, stay bool) {
	if answered == NoSlot {
		var ask bool
		if host, ask, stay = s.auraEntry(k, o, ctrl); ask || stay {
			return NoSlot, ask, stay
		}
	} else {
		host = answered
	}
	if DebugHandAuraNoAttach {
		host = NoSlot
	}
	return host, false, false
}

// handAuraArmed reports whether the running instruction is the one-card hand
// move marked SrAuraEntry, or a library search onto the battlefield, which kgen
// keeps to one card when its filter may admit an Aura (Guardian Sunmare; the card
// waits in the library, unshuffled, as gorge's search does); false when moveFound
// is driven outside a program (kernel tests).
//
//go:noinline
func (s *State) handAuraArmed(k *prep) bool {
	pr := s.prog(k)
	if !s.res.active || int(s.res.pc) >= len(pr) {
		return false
	}
	in := &pr[s.res.pc]
	return in.Op == kop.OpSearchEx || in.Op == kop.OpChooseMove && in.B.N&kop.SrAuraEntry != 0
}

// enterOnto arms the next enter to attach its Aura to host before the enters
// check: an Aura that enters attached (CR 303.4f) is attached when its
// "whenever an enchantment enters" watchers look (CR 603.6a), so a host it
// strips of abilities, or whose controller it changes, is seen as it then is
// (gorge folds the move and the attach before the trigger scan). A host that
// is gone or NoSlot arms nothing.
func (s *State) enterOnto(host Slot) {
	if host != NoSlot && s.occ.has(host) {
		s.g.entHost = uint8(host) + 1
	}
}

// attachEntering attaches the entering Aura at sl to the host enterOnto armed.
//
//go:noinline
func (s *State) attachEntering(k *prep, sl Slot) {
	host := Slot(s.g.entHost - 1)
	s.g.entHost = 0
	if !s.occ.has(host) {
		return
	}
	s.attachTo(k, sl, host)
	s.touchSl(sl)
	if k.gen != nil {
		s.refreshTy(k)
	}
}

// attachedTo attaches the Aura that entered at sl to host unless enter already
// did (enterOnto): the move sites' attach after the enter. It disarms enterOnto
// too, for an entry that took the held path and never reached enter.
func (s *State) attachedTo(k *prep, sl, host Slot) {
	s.g.entHost = 0
	if host != NoSlot && s.occ.has(host) && s.perms[sl].attached != host {
		s.attachTo(k, sl, host)
	}
}
