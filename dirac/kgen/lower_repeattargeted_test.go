package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// repeatEachTargetedCard lowers one real card and returns its abilities.
func repeatEachTargetedCard(t *testing.T, name string) *lCard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	return lowerCard(p, card.Faces[0].Name, card, false)
}

// TestRepeatEachTargetedLowers: gorge's RepeatEach RepeatPlayers$ Targeted
// repeats over the ability's target slot's player targets, in
// AliveFrom(controller) order (Riverchurn Monument, Singularity Rupture).
// Parker Luck's body reads the OTHER player's revealed card
// (LifeAmount$ over a library count with an IsRemembered clause), which
// stays refused, its next blocker named.
func TestRepeatEachTargetedLowers(t *testing.T) {
	c := repeatEachTargetedCard(t, "Riverchurn Monument")
	for _, rf := range c.Refusals {
		t.Errorf("Riverchurn Monument refused: %s", rf.Reason)
	}
	saw := false
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op != "RepeatEach" {
				continue
			}
			saw = true
			if in.Obj.Kind != rTarget || in.Obj.N != 0 {
				t.Errorf("Riverchurn RepeatEach Obj %v, want Target0", in.Obj)
			}
			if in.C&1 == 0 {
				t.Errorf("Riverchurn RepeatEach C %d, want the controller-first bit", in.C)
			}
			mill := a.Prog[len(a.Prog)-in.Skip]
			if in.Skip == 0 || mill.Op != "Mill" || mill.Obj.Kind != rEach {
				t.Errorf("Riverchurn RepeatEach body %v, want a Mill over RefEach", mill)
				break
			}
			if mill.A.Kind != vCountGraveyard || mill.A.F == nil || len(mill.A.F.Clauses) != 1 ||
				mill.A.F.Clauses[0].Ctrl != whoEachPl {
				t.Errorf("Riverchurn mill amount %v, want the repeated player's graveyard count", mill.A)
			}
		}
	}
	if !saw {
		t.Fatal("Riverchurn Monument lowered no RepeatEach")
	}

	c = repeatEachTargetedCard(t, "Singularity Rupture")
	for _, rf := range c.Refusals {
		t.Errorf("Singularity Rupture refused: %s", rf.Reason)
	}
	saw = false
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op != "RepeatEach" {
				continue
			}
			saw = true
			if in.Obj.Kind != rTarget || in.Obj.N != 0 || in.C&1 == 0 {
				t.Errorf("Singularity RepeatEach Obj %v C %d, want Target0 controller-first", in.Obj, in.C)
			}
			mill := a.Prog[len(a.Prog)-in.Skip]
			if in.Skip == 0 || mill.Op != "Mill" || mill.Obj.Kind != rEach {
				t.Errorf("Singularity RepeatEach body %v, want a Mill over RefEach", mill)
				break
			}
			if mill.A.Kind != vCountHand || mill.A.N != 1 || mill.A.F == nil || len(mill.A.F.Clauses) != 1 ||
				mill.A.F.Clauses[0].Owner != whoEachPl {
				t.Errorf("Singularity mill amount %v, want the repeated player's library count by owner", mill.A)
			}
		}
	}
	if !saw {
		t.Fatal("Singularity Rupture lowered no RepeatEach")
	}

	c = repeatEachTargetedCard(t, "Parker Luck")
	found := false
	for _, rf := range c.Refusals {
		if strings.Contains(rf.Reason, "RepeatEach body LifeAmount$") {
			found = true
		}
	}
	if len(c.Refusals) == 0 || !found {
		t.Errorf("Parker Luck refusals %v, want the body's LifeAmount$ named", c.Refusals)
	}
}
