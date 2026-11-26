package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestHandXAbilityOfferedFromLands: an announced-X activated ability whose
// mana must come from untapped lands is offered from the hand too (CR 602.2b:
// mana abilities are activated while paying). Webstrike Elite with three
// untapped Forests and an empty pool is offered its {X}{G}{G} cycling; it is
// paid at X = 0 from the lands, discarding the Elite.
func TestHandXAbilityOfferedFromLands(t *testing.T) {
	t.Parallel()
	elite := corpusAlternativeCard(t, "Webstrike Elite")
	e := handEngine(t, elite)
	var forests []state.ObjID
	for i := 0; i < 3; i++ {
		forests = append(forests, onBoard(t, e, 0, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"))
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool holds %d mana, want empty", got)
	}
	for _, f := range forests {
		if o := e.G.Obj(f); o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Forest %d zone=%v tapped=%v", f, o.Zone, o.Tapped)
		}
	}
	id := e.G.Zone(state.ZHand, 0)[0]
	if o := e.G.Obj(id); o == nil || o.Face() == nil || o.Face().Name != "Webstrike Elite" {
		t.Fatalf("precondition: hand card %d is not Webstrike Elite", id)
	}
	e.priorityRound()
	var opt decision.Option
	ok := false
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == id {
			opt, ok = o, true
			break
		}
	}
	if !ok {
		t.Fatalf("X cycling not offered from three untapped Forests: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	answerX(t, e, 0)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("Webstrike Elite zone = %s, want graveyard after cycling", got)
	}
	tapped := 0
	for _, f := range forests {
		if e.G.Obj(f).Tapped {
			tapped++
		}
	}
	if tapped != 2 {
		t.Fatalf("%d Forests tapped after paying {G}{G} at X = 0, want 2", tapped)
	}
}
