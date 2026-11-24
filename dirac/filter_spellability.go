package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The three SpellCast ValidSA$ words read off the cast spell's own stack
// entry (gorge trigmatch.ValidSAMatches, a single-field clause being a plain
// spec filter over the spell, except Spell.Teamwork). Each reads false for
// anything but a spell on the stack.

// castSpellEntry is the stack entry of the spell the card matcher c reads,
// nil when c is no spell on the stack.
func (s *State) castSpellEntry(c *fctx) *stackEntry {
	if !c.card() || c.o == NoObj || c.zone != kop.ZoneStack {
		return nil
	}
	e := s.spellEntryOf(c.o)
	if e == nil || int(e.card) >= len(c.k.t.Cards) {
		return nil
	}
	return e
}

// fwModal: FwModal, gorge's `Modal`: len(ChosenModes) > 0. The entry's mode
// bits 0..NModes-1 are the charm's picks (a repeat charm's counts via
// ModeSet); a face-down spell has no ability, and a spell that is no charm
// has no modes to choose.
//
//kmanifest:fword FwModal
func (s *State) fwModal(c *fctx, l *kop.FLit) bool {
	e := s.castSpellEntry(c)
	if e == nil || e.faceDown() {
		return false
	}
	ab := spellAb(c.k, e.card)
	if ab == 255 {
		return false
	}
	a := &c.k.t.Cards[e.card].Abilities[ab]
	return a.NModes > 0 && a.ModeSet(e.mode)&(1<<a.NModes-1) != 0
}

// fwSingleTarget: FwSingleTarget, gorge's `singleTarget`:
// len(Targets)+len(SubTargets) == 1. chainTargets is the cast-time chain,
// root and every SubAbility$ link alike, not de-duplicated (two slots naming
// one object are two targets here, as in gorge's count).
//
//kmanifest:fword FwSingleTarget
func (s *State) fwSingleTarget(c *fctx, l *kop.FLit) bool {
	e := s.castSpellEntry(c)
	if e == nil {
		return false
	}
	var codes [kop.MaxTargets + maxXTgt]TCode
	return s.chainTargets(e, &codes) == 1
}

// fwTeamwork: FwTeamwork, gorge's `Teamwork` (Object.TeamworkPaid). The paid
// flag is sfKicked on the entry (kw_teamwork.go), which a kicker cast sets as
// well, so the spell's card must have Teamwork for the bit to mean it.
//
//kmanifest:fword FwTeamwork
func (s *State) fwTeamwork(c *fctx, l *kop.FLit) bool {
	e := s.castSpellEntry(c)
	return e != nil && e.flags&sfKicked != 0 && c.k.t.Cards[e.card].Teamwork != 0
}
