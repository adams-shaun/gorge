package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 205.1a, 613.1d: a God below its devotion isn't a creature, and a sweep's
// filter reads that with no other type-changing card in the match.
func TestSweepSkipsAGodBelowItsDevotion(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Pyroclasm")}, []*cards.Card{lookup(t, reg, "Nylea, Keen-Eyed")})
	if e.G.Active != 0 {
		passUntil(t, e, func() bool { return e.G.Active == 0 && e.G.Step == state.StepMain1 })
	}
	moveByName(t, e, 0, "Pyroclasm", state.ZHand)
	god := moveByName(t, e, 1, "Nylea, Keen-Eyed", state.ZBattlefield)
	addMana(t, e, 0, "CR")
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, castByName(t, e, 0, "Pyroclasm").Index)
	passUntilStackEmpty(t, e, 10)
	if n := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.Damage && ev.Obj == god }); n != 0 {
		t.Fatalf("the sweep dealt %d damage events to a God that isn't a creature", n)
	}
}

// Every static the layer walk folds into a type effect arms the pool's derived
// type table: a card the probe misses is read with its printed types.
func TestEveryTypeChangingStaticArmsThePoolGate(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	keys := []cards.ParamKey{cards.PKAddType, cards.PKAddTypes, cards.PKAddAllCreatureTypes, cards.PKRemoveType, cards.PKRemoveLandTypes}
	checked := 0
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for i := range f.Statics {
				st := &f.Statics[i]
				if st.Mode != "Continuous" {
					continue
				}
				for _, k := range keys {
					if st.HasParam(k) {
						checked++
						if !c.ChangesTypes() {
							t.Errorf("%s: a type-changing static does not arm the pool gate", f.Name)
						}
						break
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no type-changing static found")
	}
}
