package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCStatics5Lowers plays Thassa, Deep-Dwelling (a static RemoveType$
// Creature gated by CheckSVar$ X < 5 devotion to blue: "isn't a creature")
// in lockstep against the kernel. A kgen that refuses the static fails the
// kernel table build; a kernel that ignores RemoveTypes diverges on Thassa's
// type line (gorge's Chars drop Creature while devotion is below five).
// The game must reach Thassa both as a noncreature (devotion below five) and
// as a creature (five or more), the two sides of the gate.
func TestMBCStatics5Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	decks := [2][]string{
		append(deckOf(8, "Thassa, Deep-Dwelling"), append(deckOf(16, "Merfolk of the Pearl Trident"), deckOf(16, "Island")...)...),
		append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
	}
	set := charmRepeatSet(t, decks)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	prefer := []string{"Merfolk of the Pearl Trident", "Thassa, Deep-Dwelling"}
	var plain, creature bool
	reached := func(g *GorgeSide) bool {
		gm := g.E.G
		for i := range gm.Objs {
			o := &gm.Objs[i]
			if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "Thassa, Deep-Dwelling" {
				continue
			}
			ch := g.E.Chars(o.ID)
			if ch == nil {
				continue
			}
			if slices.Contains(ch.Types, "Creature") {
				creature = true
			} else {
				plain = true
			}
		}
		return plain && creature
	}
	for seed := uint64(1); seed <= 60 && !(plain && creature); seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"mbc_statics5_0", "mbc_statics5_1"}, Cards: decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions = 3000
		pol := animateRenameEquipPolicy(m, animateRenameEquipCase{prefer: prefer}, seed)
		hit := false
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, observed(pol, reached, &hit), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
	}
	if !plain || !creature {
		t.Fatalf("Thassa reached as a noncreature: %v, as a creature: %v", plain, creature)
	}
}
