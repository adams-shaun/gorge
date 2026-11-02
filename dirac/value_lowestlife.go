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
	a, b := int(s.pl[0].life), int(s.pl[1].life)
	if DebugLowLifeMax {
		if b > a {
			return b
		}
		return a
	}
	if b < a {
		return b
	}
	return a
}
