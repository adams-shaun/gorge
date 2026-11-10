package dirac

// DebugLowLifeMax (TestLowlifeLockstep mutation): lowestLife reads the
// HIGHEST life total instead of the lowest.
var DebugLowLifeMax bool

// lowestLife is ExLowestLife (kop/values.go): the lowest CURRENT life total
// among the living players (gorge lifeExtreme over AliveFrom,
// engine/effects/count_player.go; in a two-player game min of both, read as
// the ETB-tap replacement applies). CR 614.1c, 119.3.
//
//go:noinline
func (s *State) lowestLife() int {
	a := 0
	first := true
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		b := int(s.pl[p].life)
		switch {
		case first:
			a, first = b, false
		case DebugLowLifeMax && b > a, !DebugLowLifeMax && b < a:
			a = b
		}
	}
	return a
}
