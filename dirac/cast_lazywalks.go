package dirac

// The nested-walk shadow (cast_lazywalks_test.go). nestedWalkShadow makes
// every script-walk call of scriptOffers also run its eager form on a fork of
// the same State and panic unless the two leave the same options in d in the
// same order, the same spent cap, the same searched count and the same game
// State (shadowSameState). nestedWalkEager plays the eager form in the lazy
// one's place, so a whole game can be played both ways and its decision
// stream compared. Both are off outside those tests.
var nestedWalkShadow, nestedWalkEager bool

// shadowCalls counts the calls the shadow compared, and shadowPlays those
// that searched a play.
var shadowCalls, shadowPlays int

// shadowMismatch is the last shadow difference, for the test's report: the
// lazy State and the eager fork's, copied when the shadow is about to panic.
var shadowMismatch *struct{ lazy, eager State }

// offerScripts is the script walk's call of scriptOffers.
func (s *State) offerScripts(k *prep, d, capSpent *Decision, p uint8) int {
	if nestedWalkShadow {
		return s.shadowScriptOffers(k, d, capSpent, p)
	}
	return s.scriptOffers(k, d, capSpent, p)
}

func (s *State) shadowScriptOffers(k *prep, d, capSpent *Decision, p uint8) int {
	f := s.fork()
	dE := *d
	var cE Decision
	cp := (*Decision)(nil)
	if capSpent != nil {
		cE, cp = *capSpent, &cE
	}
	shadowCalls++
	was := nestedWalkEager
	nestedWalkEager = true
	nE := f.scriptOffers(k, &dE, cp, p)
	nestedWalkEager = false
	n := s.scriptOffers(k, d, capSpent, p) // its forks sit above f in the pool
	nestedWalkEager = was
	why := ""
	switch {
	case n != nE:
		why = "searched"
	case !sameOptionList(d, &dE):
		why = "options"
	case capSpent != nil && !sameOptionList(capSpent, &cE):
		why = "capSpent"
	case !shadowSameState(s, f):
		why = "state"
	}
	if nE > 0 {
		shadowPlays++
	}
	if why != "" {
		shadowMismatch = new(struct{ lazy, eager State })
		shadowMismatch.lazy.CopyFrom(s)
		shadowMismatch.eager.CopyFrom(f)
	}
	s.unfork()
	if why != "" {
		panic("dirac: scriptOffers lazy walks differ from the eager path: " + why)
	}
	return n
}

func sameOptionList(a, b *Decision) bool {
	if a.N != b.N {
		return false
	}
	for i := 0; i < int(a.N); i++ {
		if a.Options[i] != b.Options[i] {
			return false
		}
	}
	return true
}

// shadowSameState compares the lazy State with the eager fork's, the
// filtBoundPays memo aside: a walk the lazy path skipped may have left a
// different entry there, and the memo answers only for its full key
// (mana_filter.go fbpMemo).
func shadowSameState(s, f *State) bool {
	m := s.g.fbp
	s.g.fbp = f.g.fbp
	same := s.SameGame(f)
	s.g.fbp = m
	return same
}
