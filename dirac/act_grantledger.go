package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The granted-activation ledger: ActivationLimit$ N (ExActivations N 1) is
// per ability in gorge (activationUsedCount reads one ability's AbilityPush
// records; a GRANTED activation carries Amount -1 and its own SVar), while the
// per-object census object.acts counts every non-mana activation of the
// object, which gorge's view ActivatedThisTurn also does. A Pit Imp wearing
// Lightning Prowess that taps for the Aura's damage has used none of its own
// {B} pump's two activations (CR 602.5b), so the offer gate reads the census
// less the object's granted activations this turn.
//
// The kernel lists the host of each granted activation pushed this turn in a
// cold region (coldGrantAct, source Obj+1, 0 = empty), present only for a
// table with an ExActivations N 1 expression (prepGrantAct), so State itself
// does not grow and no fixed field is added (TestNoNewFixedStateFields). A fifth granted activation in one turn stalls with
// StallActRing rather than truncating.

// grantActN is the granted activations per turn the ledger records.
const grantActN = 4

// coldGrantAct is the ledger, grantActN hosts, laid out by prepGrantAct (it
// has no need of its own: a table without the expression has no region).
var coldGrantAct = newCold[Obj]("grantAct", nil)

// prepGrantAct sizes the ledger for a table with an ExActivations N 1
// expression (an ActivationLimit$ N offer gate).
func prepGrantAct(t *kop.Table, p *prep) {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExActivations && t.Exprs[i].N == 1 {
			coldGrantAct.set(&p.cold, grantActN)
			return
		}
	}
}

// grantActOn reports whether s's table keeps the ledger.
func (s *State) grantActOn() bool { return coldGrantAct.count(s.cold.layout) > 0 }

// noteGrantAct records the push of a granted ability hosted by o (pushBuild).
//
//go:noinline
func (s *State) noteGrantAct(o Obj) {
	r := coldGrantAct.of(s)
	for i := range r {
		if r[i] == 0 {
			r[i] = o + 1
			return
		}
	}
	s.capFull(StallActRing)
}

// dropGrantAct takes back the newest record of host o: a reversed activation
// (unpushActivation) is not one (CR 733.1).
//
//go:noinline
func (s *State) dropGrantAct(o Obj) {
	r := coldGrantAct.of(s)
	for i := len(r) - 1; i >= 0; i-- {
		if r[i] == o+1 {
			r[i] = 0
			return
		}
	}
}

// grantActsOf counts the granted activations of host o this turn.
func (s *State) grantActsOf(o Obj) int {
	n := 0
	for _, h := range coldGrantAct.of(s) {
		if h == o+1 {
			n++
		}
	}
	return n
}

// dropGrantActs ends every record of host o with its stint (CR 400.7), as the
// census does.
func (s *State) dropGrantActs(o Obj) {
	r := coldGrantAct.of(s)
	for i := range r {
		if r[i] == o+1 {
			r[i] = 0
		}
	}
}

// clearGrantActs ends the turn's records.
func (s *State) clearGrantActs() { clear(coldGrantAct.of(s)) }
