package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestKickerXMinLowers: a kicker that announces an {X} lowers (Thieving
// Skydiver's XMin1 X, Kangee's X 2, Verdeloth's X). The {X} rides CardDef.Kicker
// and the floor CardDef.KickerCost.XMin with Flags 0 (dirac/cost_sacx.go).
func TestKickerXMinLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Thieving Skydiver", "Kangee, Aerie Keeper", "Verdeloth the Ancient"} {
		t.Run(name, func(t *testing.T) {
			lc := lowerMultikickerCard(t, reg, name)
			if len(lc.Refusals) != 0 {
				t.Fatalf("%s refusals = %v, want none", name, lc.Refusals)
			}
			if lc.Kicker == nil || !lc.Kicker.X {
				t.Fatalf("%s: Kicker = %+v, want one announced {X}", name, lc.Kicker)
			}
		})
	}
	d := kickerXBuild(t, reg, "Thieving Skydiver")
	if d.Kicker.X != 1 || d.KickerCost.XMin != 1 || d.KickerCost.Flags != 0 {
		t.Errorf("Thieving Skydiver: Kicker.X %d KickerCost %+v, want X 1 and a Flags-0 XMin-1 part", d.Kicker.X, d.KickerCost)
	}
	d = kickerXBuild(t, reg, "Kangee, Aerie Keeper")
	if d.Kicker.X != 1 || d.KickerCost.XMin != 0 {
		t.Errorf("Kangee: Kicker.X %d KickerCost.XMin %d, want X 1 and floor 0", d.Kicker.X, d.KickerCost.XMin)
	}
}

// kickerXBuild lowers and builds one corpus card into its table entry.
func kickerXBuild(t *testing.T, reg *cards.Registry, name string) kop.CardDef {
	t.Helper()
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: reg}
	lw := lowerPool(p)
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatalf("%s build: %v", name, err)
	}
	return bs[0].def
}
