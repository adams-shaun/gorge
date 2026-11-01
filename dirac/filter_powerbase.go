package dirac

// DebugPowerGTBaseGE is a mutation hook (TestPowerbaseMutation): >= instead
// of >, so an unmodified creature matches powerGTbasePower.
var DebugPowerGTBaseGE bool

// powerGTBase is PredPowerGTBase on a battlefield permanent: gorge
// powerGTbasePower, the current layered power (ptGen) greater than the base
// power through layer 7b (basePTGen; CR 613.4).
//
//go:noinline
func (s *State) powerGTBase(k *prep, sl Slot) bool {
	pw, _ := s.ptGen(k, sl)
	bp, _ := s.basePTGen(k, sl)
	if DebugPowerGTBaseGE {
		return pw >= bp
	}
	return pw > bp
}
