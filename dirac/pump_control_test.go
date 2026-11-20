package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// pumpControlScen is a vanilla table with one hand-built Pump
// Duration$ AsLongAsControl record (an Indestructible grant); idx is its
// Table.Overrides index.
func pumpControlScen(t *testing.T) (sc *scen, idx int) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Overrides = append([]kop.Override(nil), v.Table.Overrides...)
	idx = len(tbl.Overrides)
	tbl.Overrides = append(tbl.Overrides, kop.Override{WhileHost: true, AsLongAsControl: true, AddKw: kop.KwIndestructible})
	tbl.Exprs = make([]kop.Expr, 1) // a generated table: prepareGen keeps g.gen
	sc = newScenTable(t, &tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyWhileHost {
		t.Fatal("prep: the WhileHost flag did not reach genPrep")
	}
	return sc, idx
}

func (sc *scen) indestructible(o Obj) bool {
	return sc.s.kw(sc.k, sc.slot(o))&kop.KwIndestructible != 0
}

// pumpControlApply resolves the grant from host onto target for ctrl.
func pumpControlApply(t *testing.T, sc *scen, idx int, host, target Obj, ctrl uint8) {
	t.Helper()
	e := &stackEntry{obj: host, srcVer: sc.s.objs[host].ver, ctrl: ctrl}
	sc.s.applyWhileHost(sc.k, sc.slot(target), idx, e)
}

func TestPumpControlEndsOnHostControlChange(t *testing.T) {
	for _, self := range []bool{false, true} {
		sc, idx := pumpControlScen(t)
		host := sc.perm(v.GrizzlyBears, 0)
		target := host
		if !self {
			target = sc.perm(v.GrizzlyBears, 0)
		}
		pumpControlApply(t, sc, idx, host, target, 0)
		if !sc.indestructible(target) {
			t.Fatalf("self=%v: precondition: the grant did not apply", self)
		}
		sc.s.ctrlMove(sc.slot(host), 1)
		if sc.s.perms[sc.slot(host)].ctrl != 1 {
			t.Fatalf("self=%v: precondition: the host did not change controller", self)
		}
		if sc.indestructible(target) {
			t.Fatalf("self=%v: the grant outlived the host's control change", self)
		}
		sc.s.ctrlMove(sc.slot(host), 0) // back under the activator: no revival
		if sc.indestructible(target) {
			t.Fatalf("self=%v: the grant revived when the host came back", self)
		}
	}
}

func TestPumpControlEndsOnHostLeaving(t *testing.T) {
	sc, idx := pumpControlScen(t)
	host := sc.perm(v.GrizzlyBears, 0)
	target := sc.perm(v.GrizzlyBears, 0)
	pumpControlApply(t, sc, idx, host, target, 0)
	if !sc.indestructible(target) {
		t.Fatal("precondition: the grant did not apply")
	}
	sc.s.hostLeft(sc.k, host)
	if sc.indestructible(target) {
		t.Fatal("the grant outlived its host")
	}
}

func TestPumpControlNotAppliedWithoutHostControl(t *testing.T) {
	sc, idx := pumpControlScen(t)
	host := sc.perm(v.GrizzlyBears, 1)
	target := sc.perm(v.GrizzlyBears, 0)
	if c := sc.s.perms[sc.slot(host)].ctrl; c != 1 {
		t.Fatalf("precondition: the host is controlled by %d, want 1", c)
	}
	pumpControlApply(t, sc, idx, host, target, 0)
	if sc.indestructible(target) {
		t.Fatal("a grant from a host the activator does not control applied")
	}
	pumpControlApply(t, sc, idx, host, target, 1)
	if !sc.indestructible(target) {
		t.Fatal("the host's controller was refused the grant")
	}
}

func TestPumpControlEndsOnPhaseOut(t *testing.T) {
	sc, idx := pumpControlScen(t)
	host := sc.perm(v.GrizzlyBears, 0)
	target := sc.perm(v.GrizzlyBears, 0)
	pumpControlApply(t, sc, idx, host, target, 0)
	if !sc.indestructible(target) {
		t.Fatal("precondition: the grant did not apply")
	}
	sc.s.phaseOut(sc.k, sc.slot(host), false)
	if !sc.s.hiddenAt(sc.slot(host)) {
		t.Fatal("precondition: the host did not phase out")
	}
	if sc.indestructible(target) {
		t.Fatal("the grant outlived its host's phase-out")
	}
	sc.s.phaseIn(sc.k, sc.slot(host))
	if sc.s.hiddenAt(sc.slot(host)) {
		t.Fatal("precondition: the host did not phase in")
	}
	if sc.indestructible(target) {
		t.Fatal("the grant revived when the host phased in")
	}
}
