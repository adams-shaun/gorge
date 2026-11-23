package dirac

import "testing"

// TestRelaxedWalkListsKeepsOuterWalkState: actScriptPays runs while an outer
// priority walk is live (canActivate), and the nested plainOptions ends with
// availX = 0 and the walk clears the filter skip. The outer walk's census mode
// and filter skip must come back, or the rest of its options are priced
// against the wrong census.
func TestRelaxedWalkListsKeepsOuterWalkState(t *testing.T) {
	sc, id, _ := unlessHandGuideBoard(t)
	land := sc.perm(id["Plains"], 0)
	sc.hand(id["Simian Spirit Guide"], 0)
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("Step kind %d, want priority", d.Kind)
	}
	outer := availPlain | availGaps | availOwn
	sc.s.g.availX = outer
	sc.s.setFiltSkip(3)
	sc.s.relaxedWalkLists(sc.k, MakeAction(AkActivate, TCode(land), 0, 0))
	if sc.s.g.availX != outer {
		t.Errorf("availX = %d after the nested walk, want the outer walk's %d", sc.s.g.availX, outer)
	}
	if got := sc.s.filtSkip(); got != 3 {
		t.Errorf("filtSkip = %d after the nested walk, want 3", got)
	}
}
