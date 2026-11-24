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
// The kernel lists the hosts of the granted activations pushed this turn in a
// cold region (coldGrantAct, packed as the activation ledger's entries are:
// host Obj+1 and its push count, actPack, 0 = empty), present only for a
// table with an ExActivations N 1 expression (prepGrantAct), so State itself
// does not grow and no fixed field is added (TestNoNewFixedStateFields). A fifth
// distinct host of granted activations in one turn stalls with StallActRing
// rather than truncating (a fifth push did before 2026-11-19).

// grantActN is the distinct hosts per turn the ledger records.
const grantActN = 4

// coldGrantAct is the ledger, grantActN hosts, laid out by prepGrantAct (it
// has no need of its own: a table without the expression has no region).
var coldGrantAct = newCold[Obj]("grantAct", nil)

// prepGrantAct sizes the ledger for a table with an ExActivations N 1
// expression (an ActivationLimit$ N offer gate).
func prepGrantAct(t *kop.Table, p *prep) {
	prepOvMk(t, p) // act_ovmaker.go: the granting record's maker ledger, sized beside the grant ledger
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
	if !actBump(coldGrantAct.of(s), o) {
		s.capFull(StallActRing)
	}
}

// dropGrantAct takes back one push of host o: a reversed activation
// (unpushActivation) is not one (CR 733.1).
//
//go:noinline
func (s *State) dropGrantAct(o Obj) {
	r := coldGrantAct.of(s)
	for i := len(r) - 1; i >= 0; i-- {
		if r[i] != 0 && r[i]&actObjMask == o+1 {
			if r[i] -= 1 << actCntShift; r[i]>>actCntShift == 0 {
				r[i] = 0
			}
			return
		}
	}
}

// grantActsOf counts the granted activations of host o this turn.
func (s *State) grantActsOf(o Obj) int {
	n := 0
	for _, h := range coldGrantAct.of(s) {
		if h != 0 && h&actObjMask == o+1 {
			n += int(h >> actCntShift)
		}
	}
	return n
}

// dropGrantActs ends every record of host o with its stint (CR 400.7), as the
// census does.
func (s *State) dropGrantActs(o Obj) {
	r := coldGrantAct.of(s)
	for i := range r {
		if r[i] != 0 && r[i]&actObjMask == o+1 {
			r[i] = 0
		}
	}
}

// clearGrantActs ends the turn's records.
func (s *State) clearGrantActs() { clear(coldGrantAct.of(s)) }
