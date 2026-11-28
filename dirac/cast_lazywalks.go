package dirac

// nestedWalkHook, when set, answers scriptOffers' calls from the script walk
// in its place (offerScripts). Only the nested-walk shadow tests set it
// (cast_lazywalks_test.go): it runs the eager body scriptOffers replaced
// beside the lazy one and panics on any difference.
var nestedWalkHook func(s *State, k *prep, d, capSpent *Decision, p uint8) int

// offerScripts is the script walk's call of scriptOffers, through the
// shadow tests' hook when one is set.
func (s *State) offerScripts(k *prep, d, capSpent *Decision, p uint8) int {
	if nestedWalkHook != nil {
		return nestedWalkHook(s, k, d, capSpent, p)
	}
	return s.scriptOffers(k, d, capSpent, p)
}
