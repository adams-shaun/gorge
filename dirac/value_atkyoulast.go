package dirac

// ExAtkYouLastTurn is PlayerCountPlayers$HasPropertyattackedYouTheirLastTurn
// (Avenge's ReduceCost CheckSVar$ X): the number of the controller's living
// opponents that declared an attack on the controller during their own most
// recent completed turn (gorge count_cov3.go
// cov3PlayerScalarHasPropertyAttackedYouTheirLastTurn over
// Engine.AttackedDuringLastTurn; the controller itself never counts). It
// reads the per-seat attack record of trig_atklastturn.go, which a table
// keeps when genPrep.anyAtkLast is set (a DamageDone line with
// Tf5DmgAttackedYouLastTurn, or an expression of this kind).

// DebugAtkYouLastYes counts the reads that found an attacking opponent (a
// test-only counter: the lockstep asserts a game reached the shape).
var DebugAtkYouLastYes int

// atkYouLastCount counts the living seats other than you that attacked you
// during their last turn.
func (s *State) atkYouLastCount(you uint8) int {
	n := 0
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != you && !s.out(q) && s.attackedYouLastTurn(q, you) {
			n++
		}
	}
	if n != 0 {
		DebugAtkYouLastYes++
	}
	return n
}
