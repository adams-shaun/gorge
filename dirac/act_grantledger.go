package dirac

// The granted-activation ledger: ActivationLimit$ N (ExActivations N 1) is
// per ability in gorge (activationUsedCount reads one ability's AbilityPush
// records; a GRANTED activation carries Amount -1 and its own SVar), while the
// per-object census object.acts counts every non-mana activation of the
// object, which gorge's view ActivatedThisTurn also does. A Pit Imp wearing
// Lightning Prowess that taps for the Aura's damage has used none of its own
// {B} pump's two activations (CR 602.5b), so the offer gate reads the census
// less the object's granted activations this turn.
//
// The kernel lists the host of each granted activation pushed this turn
// (genState.grantAct, source Obj+1), recorded only for a table with an
// ExActivations N 1 expression (genPrep.grantActOn). A fifth granted
// activation in one turn stalls with StallActRing rather than truncating.

// grantActN is the granted activations per turn the ledger records.
const grantActN = 4

// noteGrantAct records the push of a granted ability hosted by o (pushBuild).
//
//go:noinline
func (s *State) noteGrantAct(o Obj) {
	if s.g.grantN >= grantActN {
		s.capFull(StallActRing)
		return
	}
	s.g.grantAct[s.g.grantN] = o + 1
	s.g.grantN++
}

// dropGrantAct takes back the newest record of host o: a reversed activation
// (unpushActivation) is not one (CR 733.1).
//
//go:noinline
func (s *State) dropGrantAct(o Obj) {
	for i := int(s.g.grantN) - 1; i >= 0; i-- {
		if s.g.grantAct[i] == o+1 {
			copy(s.g.grantAct[i:s.g.grantN], s.g.grantAct[i+1:s.g.grantN])
			s.g.grantN--
			s.g.grantAct[s.g.grantN] = 0
			return
		}
	}
}

// grantActsOf counts the granted activations of host o this turn.
func (s *State) grantActsOf(o Obj) int {
	n := 0
	for i := uint8(0); i < s.g.grantN; i++ {
		if s.g.grantAct[i] == o+1 {
			n++
		}
	}
	return n
}

// dropGrantActs ends every record of host o with its stint (CR 400.7), as the
// census does.
func (s *State) dropGrantActs(o Obj) {
	for s.grantActsOf(o) > 0 {
		s.dropGrantAct(o)
	}
}
