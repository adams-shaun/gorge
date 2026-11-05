package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGrantedSacrificeHostCostPosesNoChoice pins CR 602.2b and 701.21a for a
// granted Sac<1/OriginalHost> cost: it names exactly the grantor, so with the
// Equipment still under its controller's control paying it has one possible
// answer. Deconstruction Hammer's granted ability, once its target is chosen,
// sacrifices the Hammer and taps the creature without posing a sacrifice
// KChoose.
func TestGrantedSacrificeHostCostPosesNoChoice(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := linkBoard(t, reg, []string{"Deconstruction Hammer", "Grizzly Bears"}, nil)
	hammer := findOnBoard(t, e, 0, "Deconstruction Hammer")
	bear := findOnBoard(t, e, 0, "Grizzly Bears")
	addMana(t, e, 0, "C")
	opt, ok := findAbilityCostOption(e, hammer, 0, 0)
	if !ok {
		t.Fatal("precondition: Equip not offered")
	}
	submitChoices(t, e, opt.Index)
	targetObject(t, e, bear)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(hammer).AttachedTo != bear {
		t.Fatalf("precondition: Hammer attached to %d, want the bear %d", e.G.Obj(hammer).AttachedTo, bear)
	}
	addMana(t, e, 0, "CCC")
	var granted decision.Option
	found := false
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == bear {
			granted, found = o, true
		}
	}
	if !found {
		t.Fatal("precondition: the granted destroy ability is not offered on the equipped creature")
	}
	e.beginActivation(0, granted)
	targetObject(t, e, hammer)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("granted Sacrifice-the-host cost posed a KChoose: %+v", d)
	}
	if got := e.G.Obj(hammer).Zone; got != state.ZGraveyard {
		t.Fatalf("Hammer in %s, want graveyard (sacrificed as the cost)", got)
	}
	if !e.G.Obj(bear).Tapped {
		t.Fatal("the equipped creature was not tapped as the cost")
	}
}
