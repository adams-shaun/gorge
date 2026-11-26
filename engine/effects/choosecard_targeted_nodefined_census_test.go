package effects

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestChooseCardTargetedNoDefinedAsksNobody is the class census for a
// ChooseCard that carries ValidTgts$ and no Defined$ (Hunted by The Family's
// target carrier). Forge reads only the targeted players as choosers, so
// when every target is a card nobody is asked: the targeted card's
// controller must not become a chooser (CR 115.1).
func TestChooseCardTargetedNoDefinedAsksNobody(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var carriers []*cards.SA
	var names []string
	visit := func(name string, sa *cards.SA) {
		if sa == nil || sa.API != "ChooseCard" {
			return
		}
		if strings.TrimSpace(sa.ParamStr(cards.PKDefined)) == "" && TargetsOf(sa).Has(TgtValidPresent) {
			carriers = append(carriers, sa)
			names = append(names, name)
		}
	}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, sa := range f.Abilities {
				visit(f.Name, sa)
			}
			keys := make([]string, 0, len(f.SVars))
			for k := range f.SVars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if strings.Contains(f.SVars[k], "ChooseCard") {
					visit(f.Name, cards.ResolveSVar(f.SVars, k))
				}
			}
		}
	}
	if len(carriers) == 0 {
		t.Fatal("census found no targeting Defined$-less ChooseCard: the scan is not reading the corpus")
	}
	h, c := fixtureHost(t)
	victim := h.g.AddObject(mkCard(t, "Name:Victim\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	c.Targets = []state.Target{{Obj: victim.ID}}
	for i, sa := range carriers {
		if got := chooseCardChoosers(h, c, sa); len(got) != 0 {
			t.Errorf("%s: ChooseCard with only card targets asks %v, want nobody", names[i], got)
		}
	}
	// A targeted PLAYER is still the chooser.
	c.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	if got := chooseCardChoosers(h, c, carriers[0]); len(got) != 1 || got[0] != 1 {
		t.Errorf("%s: a player target asks %v, want [1]", names[0], got)
	}
	t.Logf("%d targeting Defined$-less ChooseCard carriers", len(carriers))
}
