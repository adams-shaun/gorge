package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// emptyManaAsk activates Three Tree City on a board with chosen as its chosen
// creature type and returns the engine at the ask that follows, with the
// ask's full answer list.
func emptyManaAsk(t *testing.T, chosen string, extra ...string) (*Engine, *decision.Decision, []decision.Intent) {
	t.Helper()
	e := boundBoard(t, append([]string{"Three Tree City", "Mountain", "Mountain"}, extra...), nil)
	var ttc state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == "Three Tree City" {
			ttc = id
		}
	}
	e.G.Obj(ttc).ChosenType = chosen
	// the {2} fee is paid from the floating pool: tap both Mountains first
	activate := func(match func(o decision.Option) bool) {
		t.Helper()
		d := e.Pending()
		for _, o := range d.Options {
			if o.Kind == "activate" && match(o) {
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
		t.Fatal("the mana ability is not offered")
	}
	for i := 0; i < 2; i++ {
		activate(func(o decision.Option) bool { return o.Obj != ttc })
	}
	activate(func(o decision.Option) bool { return o.Obj == ttc })
	d := e.Pending()
	return e, d, scriptAnswers(d)
}

// TestScriptDropsAnEmptyManaAbilityAnswer pins the prefix search's pruning of
// a mana ability that yields nothing: Three Tree City's {2},{T} with no
// creature of its chosen type is no answer to branch on, while with one it
// stays.
func TestScriptDropsAnEmptyManaAbilityAnswer(t *testing.T) {
	e, d, all := emptyManaAsk(t, "Mutant")
	if len(all) < 2 {
		t.Fatalf("precondition: the ask has %d answers, want both of Three Tree City's abilities", len(all))
	}
	kept := scriptDropEmptyManaAnswers(e.manaActivation, d.Player, d, all, func(ab *cards.SA) bool {
		return e.manaAbilityYieldsNothing(d.Player, d.Source, e.G.Obj(d.Source).Face(), ab)
	})
	if len(kept) != len(all)-1 {
		t.Fatalf("no creature of the chosen type: kept %d of %d answers, want the empty ability's one dropped", len(kept), len(all))
	}
	e, d, all = emptyManaAsk(t, "Bear", "Grizzly Bears")
	if kept := scriptDropEmptyManaAnswers(e.manaActivation, d.Player, d, all, func(ab *cards.SA) bool {
		return e.manaAbilityYieldsNothing(d.Player, d.Source, e.G.Obj(d.Source).Face(), ab)
	}); len(kept) != len(all) {
		t.Fatalf("a creature of the chosen type: kept %d of %d answers, want all", len(kept), len(all))
	}
}
