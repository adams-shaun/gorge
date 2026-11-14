package dirac

// ExOppsLostLife N 3: the controller's opponents dealt damage this turn
// (gorge PlayerCountOpponents$HasPropertywasDealtDamageThisTurn and its
// PlayerCountRegisteredOpponents twin, effects/count_player.go
// hasPropertyStateBacked over opponentGroup: the living opponents whose
// Host.DamageTakenThisTurn is above 0, every player-recipient Damage event
// since the turn began, combat or not, after replacement and prevention,
// infect's included). Gnoll War Band's "{1} less for each opponent who was
// dealt damage this turn".
//
// genState.plDmg bit p is set wherever the kernel lands damage on player p:
// a combat pass's hits (noteCombatHits) and each noncombat landing (damageTo,
// dmgSrcPlayers, damageBatch, painDamage, dealNC, the combat redirect
// tails). It clears at the turn start with the other combat-hit records
// (turnTalliesNewTurn). Kept only when a table reads it (genPrep.plDmgOn).

// DebugOppsDamagedNoncombatOff is the mutation switch: a noncombat landing
// is not recorded, so an opponent dealt only noncombat damage is not
// counted.
var DebugOppsDamagedNoncombatOff bool

// notePlDamaged records that player p was dealt damage this turn.
func (s *State) notePlDamaged(k *prep, p uint8) {
	if k.gen != nil && k.gen.plDmgOn && p < MaxSeats {
		s.g.plDmg |= 1 << p
	}
}

// notePlDamagedNC is notePlDamaged for a noncombat landing on target code
// (a player code; an object code records nothing).
func (s *State) notePlDamagedNC(k *prep, code TCode) {
	if DebugOppsDamagedNoncombatOff || !isPlayerOrNone(code) {
		return
	}
	s.notePlDamaged(k, CodeSeat(code))
}

// oppDamaged reports whether living player q was dealt damage this turn.
func (s *State) oppDamaged(q uint8) bool {
	return !s.pl[q].lost && s.g.plDmg>>q&1 != 0
}
