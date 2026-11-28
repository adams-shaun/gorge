package dirac

// emblemSrc reports whether c, a damage source with no battlefield slot, is
// the source of the resolving emblem ability: an emblem's triggered ability
// runs as the creating card's latent copy (triggers_gen.go fireEmblemsAmt),
// but its source is the emblem, which has no characteristics (CR 114.3; gorge
// state.Object.EmblemSource, engine/effects/damage.go newDamageRider). So the
// emblem's damage reads no colour and no keyword of the creating card. A timed
// Effect stored as an emblem is not marked: its damage is the card's.
func (s *State) emblemSrc(k *prep, c CardID) bool {
	if !s.res.active {
		return false
	}
	e := &s.res.e
	if e.kind == seSpell || e.card != c {
		return false
	}
	abs := k.t.Cards[c].Abilities
	return int(e.ab) < len(abs) && abs[e.ab].EmblemSrc
}
