package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kickedWaivedFeasible is the kicked cast's target census for a spell whose plain cast is
// unfeasible only for its root target (CR 601.2b, 601.2c; gorge kickedWaivedRootTargetsAvailable,
// rules/target_kicked_chain.go): a root whose minimum reads the kicked bit (Tear Asunder's
// TargetMin$ X over Count$Kicked.0.1, the kicked cast takes none) announces nothing when
// kicked, so the kicked cast needs only the slots it still owes, each at its kicked minimum.
// It is false for a spell with no such root, which stays the plain census's alone.
func (s *State) kickedWaivedFeasible(k *prep, c CardID, p uint8) bool {
	ab := spellAb(k, c)
	if ab == 255 {
		return false
	}
	a := &k.t.Cards[c].Abilities[ab]
	if a.NModes > 0 {
		return false
	}
	if k.gen != nil && s.g.probe.kind == 0 {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
		defer func() { s.g.probe = probe{} }()
	}
	waived := false
	for i := 0; i < int(a.NTargets); i++ {
		t := &a.Targets[i]
		if t.Kind == kop.TgtNone || t.UpTo || t.TFlags&(kop.TgfAtResolution|kop.TgfGiftRange) != 0 {
			continue
		}
		if t.TFlags&kop.TgfKickedRange == 0 {
			if staticSlotMin(t) > 0 && !s.hasTarget(k, t, p, NoSlot, NoObj) {
				return false
			}
			continue
		}
		switch {
		case t.MinKicked == 0:
			waived = waived || staticSlotMin(t) > 0
		case !s.hasTarget(k, t, p, NoSlot, NoObj):
			return false
		}
	}
	return waived
}

// offerKickedRootless is the hand walk's kicked offer for a card the plain cast's target census
// leaves out (tgtOK, twOK: whether the plain or the teamworked cast has its targets; both false is the census gap) when the
// kicked cast needs none of that root (kickedWaivedFeasible). Every other gate the plain cast
// owes is judged as the walk does before it offers anything.
func (s *State) offerKickedRootless(k *prep, d *Decision, p uint8, c CardID, o Obj, base kop.ManaCost, tgtOK, twOK bool, preDelve, plainBase *kop.ManaCost, cav, av *avail) {
	if tgtOK || twOK || k.cards[c].kw&kop.KwKicker == 0 && !k.t.Cards[c].HasMultikicker {
		return
	}
	if s.kickedWaivedFeasible(k, c, p) && s.spellCostsFeasible(k, c, p, o) && s.altAddCastableFrom(k, c, p, o, preDelve, plainBase, false, false, av) && !s.castHeldOut(o) {
		s.offerKickedHand(k, d, p, c, o, base, cav)
	}
}
