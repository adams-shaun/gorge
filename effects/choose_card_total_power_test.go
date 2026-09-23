package effects

// The WithTotalPower$ ChooseCard unit pin for budgetGreedyTake -- the
// deterministic forced take the no-host fallback applies under a cumulative
// power budget (rules/choose_card_total_power_test.go pins the real corpus
// card end-to-end): walk the affordable pool in its filter order, take each
// card only while the running sum fits the cap, up to max picks.

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestChooseCardBudgetGreedyTake(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	var ids []state.ObjID
	for i, pt := range []string{"5/5", "3/3", "2/2", "2/2", "1/1"} {
		src := "Name:Cre " + string(rune('A'+i)) + "\nTypes:Creature\nPT:" + pt + "\nOracle:x\n"
		ids = append(ids, h.g.AddObject(mkCard(t, src), 0).ID)
	}
	h.g.SetZone(state.ZBattlefield, 0, ids)
	pool := make([]state.Target, 0, len(ids))
	for _, id := range ids {
		pool = append(pool, state.Target{Obj: id})
	}

	got := budgetGreedyTake(h, pool, 2, 4)
	if len(got) != 2 || got[0].Obj != ids[1] || got[1].Obj != ids[4] {
		t.Fatalf("greedy(5,3,2,2,1 cap 4 max 2) = %v, want [3-power, 1-power] (sum 4; either 2 would overrun to 5)", got)
	}
	got = budgetGreedyTake(h, pool, 2, 2)
	if len(got) != 1 || got[0].Obj != ids[2] {
		t.Fatalf("greedy cap 2 max 2 = %v, want [first 2-power] alone", got)
	}
	got = budgetGreedyTake(h, pool, 4, 1)
	if len(got) != 1 || got[0].Obj != ids[4] {
		t.Fatalf("greedy cap 1 = %v, want [1-power] alone", got)
	}
	got = budgetGreedyTake(h, pool, 0, 4)
	if len(got) != 0 {
		t.Fatalf("greedy max 0 = %v, want an empty take", got)
	}
}
