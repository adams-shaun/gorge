package dirac

// spellKicked is F2Kicked read off the battlefield: object o is a spell on the
// stack whose entry carries the kick flag (sfKicked; the optional-cost paid
// flag rides it too). A card in any other zone, or a spell that wasn't cast
// kicked, reads false, as gorge's ValidCard$ Card.kicked reads the cast's
// provenance. A SpellCast trigger that watches other spells for a kicked one
// (Merfolk Falconer's "whenever you cast a kicked spell") matches through it.
// The flag is also the buyback, offspring and entwine paid mark (kw_buyback.go,
// kw_offspring.go, kw_entwine.go), which gorge flags FlagBuyback /
// FlagOffspringPaid / nothing, never FlagKicked: such a spell is no kicked one
// (CR 702.33a: only kicker makes a spell kicked).
func (s *State) spellKicked(o Obj) bool {
	if o == NoObj {
		return false
	}
	e := s.spellEntryOf(o)
	if e == nil || e.flags&sfKicked == 0 {
		return false
	}
	c := e.card
	return int(c) >= len(s.k().t.Cards) || !offspringOffer(s.k(), c)
}
