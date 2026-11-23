package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// A keyword-granted activation paid from lands (no floating mana) is a
// potential play the planner prices: TestGrantedEmbalmPotentialPlanFromLands.
func TestGrantedEmbalmPotentialPlanFromLands(t *testing.T) {
	t.Parallel()
	const vanguard = "Name:Granted Vanguard\nManaCost:W\nTypes:Creature Human Soldier\nPT:1/1\nOracle:x\n"
	const plains = "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"
	e := handEngine(t)
	id := graveyardCard(t, e, vanguard)
	o := e.G.Obj(id)
	o.IntrinsicKeywords = append(o.IntrinsicKeywords, "Embalm:CardManaCost")
	pl := onBoardCard(t, e, 0, card(t, plains))
	e.G.Obj(pl).SummonSick = false
	toMain1(t, e)
	e.priorityRound()
	if got := e.grantedKeywordLines(id); len(got) != 1 {
		t.Fatalf("precondition: granted keyword lines = %q, want one", got)
	}
	found := 0
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "ability" || pp.Action.Obj != id {
			continue
		}
		found++
		if pp.Reason != "" || pp.Plan == nil {
			t.Errorf("granted embalm plan reason %q detail %q plan %v, want a witness", pp.Reason, pp.Detail, pp.Plan != nil)
		}
	}
	if found != 1 {
		t.Fatalf("granted embalm potential plays = %d, want 1", found)
	}

	// Tapping the land floats {W}: the granted embalm is then an offered
	// option of the re-posed priority decision, not only a potential play.
	d := e.Pending()
	tap := -1
	for _, op := range d.Options {
		if op.Kind == "activate" && op.Obj == pl {
			tap = op.Index
		}
	}
	if tap < 0 {
		t.Fatal("precondition: the Plains' mana ability is not offered")
	}
	submitChoices(t, e, tap)
	if got := e.G.Players[0].Pool.Total(); got != 1 {
		t.Fatalf("precondition: pool = %d after tapping the Plains, want 1", got)
	}
	var offered []decision.Option
	for _, op := range e.Pending().Options {
		if op.Kind == "ability" && op.Obj == id {
			offered = append(offered, op)
		}
	}
	if len(offered) != 1 {
		t.Fatalf("granted embalm options after floating {W} = %d, want 1", len(offered))
	}
}
