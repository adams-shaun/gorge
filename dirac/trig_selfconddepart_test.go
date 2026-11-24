package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// selfCondDepartScen is a table holding Sun Droplet's upkeep trigger: "at the
// beginning of each upkeep, if this has a charge counter, gain 1 life"
// (IsPresent$ Card.Self+counters_GE1_CHARGE, an intervening-if). lki marks the
// condition as a PresentDefined$ Self clause (Cond.Zone ZoneBattlefield), the
// one gorge presentClauseHolds answers from the departed source's last known
// counters; without it gorge counts the live battlefield. The returned source
// has one charge counter, and its trigger is on the stack in seat 1's upkeep.
func selfCondDepartScen(t *testing.T, lki bool) (*scen, Obj) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf, Pred: kop.PredCountersGE, PredN: int16(kop.CtrCharge) | 1<<8})
	f := kop.FilterID(len(tbl.Filters) - 1)
	cond := kop.Cond{Kind: kop.CondPresent, F: f, Op: kop.CmpGE, A: kop.Const(1)}
	if lki {
		cond.Zone = kop.ZoneBattlefield
	}
	tbl.Conds = []kop.Cond{{}, cond}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Droplet", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigPhase, Step: kop.StepUpkeep, Whose: kop.WhoseAny, Cond: 1},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	o := sc.perm(src, 0)
	if !sc.s.perms[sc.slot(o)].setCtrN(sc.s, int(kop.CtrCharge), 1) {
		t.Fatal("precondition: the charge counter was not placed")
	}
	sc.passUntil(t, kop.StepUpkeep)
	sc.s.Step()
	if sc.s.turn != 3 || sc.s.sp != 1 || sc.s.stack[0].obj != o || sc.s.stack[0].kind != seTrigger {
		t.Fatalf("precondition: want the source's upkeep trigger alone on the stack on turn 3, have turn %d, %d entries", sc.s.turn, sc.s.sp)
	}
	return sc, o
}

// resolveAfterDestroy destroys the source in response to its trigger and
// passes until the stack is empty, returning seat 0's life change.
func resolveAfterDestroy(t *testing.T, sc *scen, o Obj) int {
	t.Helper()
	s := sc.s
	life := s.pl[0].life
	s.destroy(sc.k, sc.slot(o))
	if sc.onBF(o) {
		t.Fatal("precondition: the source is still on the battlefield")
	}
	for i := 0; i < 8 && s.sp > 0; i++ {
		if d := s.Step(); d.Kind != DkPriority {
			t.Fatalf("decision kind %d while the trigger waits", d.Kind)
		}
		s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if s.sp != 0 {
		t.Fatalf("%d stack entries remain: the upkeep trigger never left the stack", s.sp)
	}
	return int(s.pl[0].life - life)
}

// TestSelfConditionTriggerLeavesStack: a plain IsPresent$ Card.Self+... clause
// counts the battlefield (gorge presentClauseHolds' countPresentScoped), so a
// source destroyed in response no longer satisfies it and the trigger is
// removed without effect (CR 603.4).
func TestSelfConditionTriggerLeavesStack(t *testing.T) {
	sc, o := selfCondDepartScen(t, false)
	if got := resolveAfterDestroy(t, sc, o); got != 0 {
		t.Fatalf("life changed by %d: the intervening-if read the departed source's last known counters", got)
	}
}

// TestSelfConditionPresentDefinedReadsLKI: the PresentDefined$ Self clause
// (Graft, Impending's tick) reads the departed source's last known counters
// (CR 608.2h), so the same trigger still resolves.
func TestSelfConditionPresentDefinedReadsLKI(t *testing.T) {
	sc, o := selfCondDepartScen(t, true)
	if got := resolveAfterDestroy(t, sc, o); got != 1 {
		t.Fatalf("life changed by %d, want 1: the PresentDefined$ Self clause must read the last known counters", got)
	}
}
