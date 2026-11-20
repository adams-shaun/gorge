package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The swapped-line cut's free-source scan must not read the play's own {T}
// source: Simic Cluestone's "{T}: Add {G} or {U}" is the only green source on
// the board, and the same permanent's "{G}{U}, {T}, Sacrifice this artifact:
// Draw a card" activation is the play, so its tap is the play's cost (CR
// 602.2b, 601.2h) and cannot also fund the fee-only Refractor's activation.
// Counting it dropped the Refractor's G answer as dominated and withheld the
// activation (pool tail-45, decision 704).
func TestFeeOnlyFreeTapScanSkipsThePlaysOwnTapSource(t *testing.T) {
	t.Parallel()
	e := boundBoard(t, []string{"Mountain", "Mountain", "Mountain", "Mountain", "Swamp",
		"Energy Refractor", "Simic Cluestone"}, nil)
	cl := mustObj(t, e, "Simic Cluestone")
	var refractor state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Energy Refractor" {
			refractor = id
		}
	}
	if refractor == 0 {
		t.Fatal("precondition: Energy Refractor is not on the battlefield")
	}
	feeOnlySourceMakes(t, e, cl, state.MG)
	_, opts := e.potentialWalkOf(0, true)
	var play decision.Option
	found := false
	for _, o := range opts {
		if o.Kind == "ability" && o.Obj == cl {
			play, found = o, true
			break
		}
	}
	if !found {
		t.Fatal("precondition: the potential walk has no Simic Cluestone activation option")
	}
	sr := e.newScriptSearch(0, decision.PotentialAction{Kind: play.Kind, Obj: play.Obj, Ability: play.Ability, Mode: play.Mode}, play.AltCostIndex, potentialScriptNodes)
	if sr == nil {
		t.Fatal("precondition: no script search")
	}
	if sr.feeOnlyFreeTapColour(e, refractor, state.MG) {
		t.Fatal("the scan counted the play's own Simic Cluestone as a free green source")
	}
	if !sr.feeOnlyFreeTapColour(e, refractor, state.MR) {
		t.Fatal("precondition: a Mountain is the free red source the scan must see")
	}
}
