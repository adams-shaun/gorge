package dirac

// DebugFaceDownPredUp (a mutation hook): PredFaceDown
// answers face UP, so the lockstep must diverge on the faceDown filter word.
var DebugFaceDownPredUp bool

// faceDownPred is PredFaceDown on a battlefield permanent: gorge wordFaceDown
// is a face-down battlefield object only (a face-down spell or exile card
// never matches; predCardCold answers false for every card).
//
//go:noinline
func (s *State) faceDownPred(k *prep, sl Slot) bool {
	return (s.faceDown(k, sl) && !s.blankCopy(sl)) != DebugFaceDownPredUp
}
