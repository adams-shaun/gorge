package dirac

// "whenever a player casts a spell, if it's not their
// turn" (Scytheclaw Raptor, Adrenaline Jockey; Mode$ SpellCast |
// ValidActivatingPlayer$ Player.NonActive). gorge playerSpecQualifierNonActive
// is "the matched player is not the active player" and the matched player of
// a SpellCast trigger is the caster, so the trigger (kop.WhoseNonActive,
// which whoseOK treats as any player) fires only when the caster is not
// s.active. Opponent.NonActive needs no value: it is WhoseOpp plus
// Tf2YourTurn.

// DebugNonActiveIgnore is a mutation hook: nonActiveCast never rejects, so
// the trigger fires on every cast.
var DebugNonActiveIgnore bool

// nonActiveCast: the caster of e is the active player, so a
// Player.NonActive SpellCast trigger does not match.
func (s *State) nonActiveCast(e *stackEntry) bool {
	return !DebugNonActiveIgnore && e.ctrl == s.active
}
