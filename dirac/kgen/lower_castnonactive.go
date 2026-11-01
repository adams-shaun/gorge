package kgen

// ValidActivatingPlayer$ Player.NonActive /
// Opponent.NonActive on a SpellCast trigger (Scytheclaw Raptor, Adrenaline
// Jockey, March of the World Ooze). gorge playerSpecQualifierNonActive
// (engine/effects/player_filter.go) is "the matched player is not the active
// player", and the matched player of a SpellCast trigger is the caster.

// whoNonActive is the trigger Whose of Player.NonActive. build_ab.go casts
// the value to kop.Whose, so it must equal kop.WhoseNonActive
// (TestWhoNonActiveMatchesKop).
const whoNonActive uint8 = 200

// castNonActive lowers the NonActive spellings of a SpellCast trigger's
// ValidActivatingPlayer$ (plain SpellCast only: not SpellCastOrCopy, whose
// copies have no caster to test). Opponent.NonActive is "an opponent casts
// on the listener's controller's turn": WhoseOpp plus OnlyYourTurn (two
// players). Player.NonActive is caster-relative and needs kop.WhoseNonActive.
func castNonActive(tr *lTrigger, mode, v string) (uint8, bool) {
	if mode != "SpellCast" || tr.OnlyYourTurn || tr.OnlyOppTurn {
		return 0, false
	}
	switch v {
	case "Opponent.NonActive":
		tr.OnlyYourTurn = true
		return whoOpp, true
	case "Player.NonActive":
		return whoNonActive, true
	}
	return 0, false
}
