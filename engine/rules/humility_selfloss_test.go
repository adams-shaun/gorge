package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func probePT(t *testing.T, e *Engine, id state.ObjID, wp, wt int32, what string) {
	t.Helper()
	d := e.Derived(id)
	if d.Power != wp || d.Toughness != wt {
		t.Errorf("%s: P/T = %d/%d, want %d/%d", what, d.Power, d.Toughness, wp, wt)
	}
}

// Opalescence + Humility, both orders (the classic 613.6/613.7 ruling).
// Opalescence first: Humility is a 4/4 creature in layer 4, loses its own
// abilities in layer 6 but its 7b set keeps applying (CR 613.6) after
// Opalescence's 7b -> Humility 1/1, Bears 1/1.
// Humility first: Opalescence's later 7b wins on Humility -> 4/4.
func TestHumilityOwnSetSurvivesOwnLoss(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, order := range [][2]string{{"Opalescence", "Humility"}, {"Humility", "Opalescence"}} {
		t.Run(order[0]+"First", func(t *testing.T) {
			e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Opalescence"), lookup(t, reg, "Humility"), lookup(t, reg, "Grizzly Bears")}, nil)
			bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
			first := moveCorpusCard(t, e, order[0], 0, state.ZBattlefield)
			second := moveCorpusCard(t, e, order[1], 0, state.ZBattlefield)
			hum := first
			if order[1] == "Humility" {
				hum = second
			}
			if !e.IsCreature(hum) {
				t.Fatalf("Humility is not a creature under Opalescence: %v", e.Derived(hum).Types)
			}
			probePT(t, e, bears, 1, 1, "Grizzly Bears")
			if order[0] == "Opalescence" {
				probePT(t, e, hum, 1, 1, "Humility (Humility later)")
			} else {
				probePT(t, e, hum, 4, 4, "Humility (Opalescence later)")
			}
		})
	}
}
