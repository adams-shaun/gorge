package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Firebending (CR 702.189a; gorge cards/kw_firebending.go, effects/mana_effect.go,
// events/apply_turn.go foldStepChange): "whenever this creature attacks, add N
// {R}. Until end of combat, you don't lose this mana as steps and phases end."
// The AddMana with B = 1 tallies its red units in State.fbR[p]. Gorge spends
// ordinary mana first, so the persistent units still floating at a boundary are
// min(fbR[p], pool[R]) and no spend path needs to know about them.

// fbR can sit above pool[R] after a spend that skips artSync (the restricted
// mana riders and filter lands in mana_filter.go, mana_window.go, mana_reflected.go); the next
// artSync or the boundary's min() clamps it. The FDN firebending
// path reaches none of those sites. Same latent staleness as poolArt.

// DebugFirebendEmpty is the mutation switch of TestFirebendMutation: the step
// boundary keeps nothing, as if the mana were ordinary.
var DebugFirebendEmpty bool

// StallFirebend: more than 255 firebending red at once (State.fbR).
const StallFirebend = 98

func init() { stallNames[StallFirebend] = "firebend" }

// firebendAdd tallies n firebending red units for player p, saturating (a
// further unit would be dropped silently, so the game stalls instead).
func (s *State) firebendAdd(p uint8, n int) {
	if n <= 0 {
		return
	}
	t := int(s.fbR[p]) + n
	if t > 255 {
		t = 255
		s.capFull(StallFirebend)
	}
	s.fbR[p] = uint8(t)
}

// firebendEnd is endStep with firebending red or persistent mana floating: leaving a step other
// than end of combat keeps min(fbR, pool[R]) in each pool (and fbR with it);
// leaving the end-of-combat step drops all of it, as gorge's foldStepChange
// subtracts each seat's CombatMana from PersistentMana before ManaClear.
// s.step is still the step being left.
func (s *State) firebendEnd(next kop.Step) {
	s.persistEmpty(s.step == kop.StepEndCombat) // also keeps PersistentMana$ units (mana_persistent.go)
	s.step = next
	s.ctl = ctlStepBegin
}

// FirebendRed is each player's firebending red (State.fbR) for the oracle's
// probes.
func (s *State) FirebendRed() [2]uint8 { return s.fbR }
