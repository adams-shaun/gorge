package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The as-enters copy's granted Myriad lockstep (CR 707.9b, 702.116a):
// Auton Soldier enters as a copy of a creature "except it ... has myriad".
// gorge folds the keyword into the copy's copiable values, so the copy's
// Myriad trigger follows the template's own attack triggers; the kernel
// lists the GrantedMyriad trigger from the as-enters grant record. The
// templates carry attack triggers of their own (Vicious Conquistador,
// Infectious Horror, Audacious Thief), so the trigger order is compared too.
// A copy's face is the template's, so the instances are counted by the
// printed card (state.Object.Card).

var kmyriadCopyDecks = [2][]cardset.Entry{
	{
		{Name: "Auton Soldier", Count: 6}, {Name: "Vicious Conquistador", Count: 4},
		{Name: "Infectious Horror", Count: 4}, {Name: "Audacious Thief", Count: 4},
		{Name: "Island", Count: 11}, {Name: "Swamp", Count: 11},
	},
	{
		{Name: "Vicious Conquistador", Count: 4}, {Name: "Infectious Horror", Count: 4},
		{Name: "Audacious Thief", Count: 4}, {Name: "Coral Merfolk", Count: 3},
		{Name: "Wind Drake", Count: 3}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 12},
	},
}

func TestMyriadCopyLockstep(t *testing.T) {
	t.Parallel()

	fires := 0
	games, stalls := runMyriadPool(t, "myriadcopy", kmyriadCopyDecks, 100, func(g *GorgeSide) {
		if g == nil {
			return
		}
		for i := range g.E.G.Objs {
			o := &g.E.G.Objs[i]
			if o.Ability == nil || o.Ability.API != "Myriad" {
				continue
			}
			if src := g.E.G.Obj(o.Source); src != nil && src.Card != nil && src.Card.Faces[0].Name == "Auton Soldier" {
				fires++
			}
		}
	})
	if t.Failed() {
		return
	}
	t.Logf("%d games, %d stalls; Auton Soldier copy Myriad instances: %d", games, stalls, fires)
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	if fires < kmyriadMin {
		t.Errorf("Auton Soldier copies put %d Myriad instances on the stack, want at least %d", fires, kmyriadMin)
	}
}
