package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two DIFFERENT sacrifice-another mana sources beside one creature that is the
// spell's only target (CR 601.2c-h, 601.2g-h): only the any-colour altar can
// pay the {U}, so both engines must offer the cast whichever altar the board
// ranks first, and pay it by sacrificing the target. No synthetic pool holds
// both Forge altars, so the mini corpus is inline.

var altarMixCards = map[string]string{
	"mountain": "Name:Mountain\nManaCost:no cost\nTypes:Basic Land Mountain\nOracle:({T}: Add {R}.)\n",
	"mix_prey": "Name:Mix Prey\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:\n",
	"mix_bone_altar": "Name:Mix Bone Altar\nManaCost:3\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ C | Amount$ 2 | SpellDescription$ Add {C}{C}.\n" +
		"Oracle:Sacrifice a creature: Add {C}{C}.\n",
	"mix_prism_altar": "Name:Mix Prism Altar\nManaCost:3\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ Any | SpellDescription$ Add one mana of any color.\n" +
		"Oracle:Sacrifice a creature: Add one mana of any color.\n",
	"mix_recall": "Name:Mix Recall\nManaCost:2 U\nTypes:Instant\n" +
		"A:SP$ ChangeZone | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Hand | SpellDescription$ Return target creature to its owner's hand.\n" +
		"Oracle:Return target creature to its owner's hand.\n",
}

// altarMixDecks lists seat 0's two altars in the given order: object ids
// follow the deck list, and the lower id is the altar a plan names first.
func altarMixDecks(first, second string) [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Mountain", Count: 18}, {Name: first, Count: 6}, {Name: second, Count: 6},
			{Name: "Mix Prey", Count: 5}, {Name: "Mix Recall", Count: 5},
		},
		{{Name: "Mountain", Count: 40}},
	}
}

// altarMixTally counts, by which altar has the lower object id ([0] the
// colourless one, [1] the any-colour one), the ready decisions (both altars,
// one creature, two or more untapped lands, the spell in hand), the ready ones
// offering the spell, and the casts taken.
type altarMixTally struct {
	ready, offered, taken [2]int
}

// altarMixPolicy develops the board (up to three lands, each altar paid from
// them, then one creature) and casts the spell when exactly two lands are
// untapped, so the payment has no choice of land: a spare identical land meets
// a separate difference in which one each engine taps for a sacrifice-funded
// cast.
func altarMixPolicy(tally *altarMixTally) Policy {
	first := FirstPolicy()
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority {
			return first.Pick(c)
		}
		g := c.G.E.G
		seat := state.PlayerID(c.G.Pending().Player)
		var bone, prism, creatures, lands state.ObjID
		var nCreatures, untapped int
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				switch o := g.Obj(id); o.Face().Name {
				case "Mix Bone Altar":
					bone = id
				case "Mix Prism Altar":
					prism = id
				case "Mix Prey":
					creatures = id
					nCreatures++
				case "Mountain":
					if state.PlayerID(p) == seat {
						lands++
						if !o.Tapped {
							untapped++
						}
					}
				}
			}
		}
		cast := map[string]Canon{}
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActPlayLand:
				if lands < 3 {
					return a
				}
			case ActCast:
				cast[c.G.M.InstanceCard(int(a.Src()))] = a
			}
		}
		inHand := false
		for _, id := range g.Zone(state.ZHand, seat) {
			inHand = inHand || g.Obj(id).Face().Name == "Mix Recall"
		}
		if bone != 0 && prism != 0 && creatures != 0 && nCreatures == 1 && untapped >= 2 && inHand {
			order := 0
			if prism < bone {
				order = 1
			}
			tally.ready[order]++
			if a, ok := cast["Mix Recall"]; ok {
				tally.offered[order]++
				if untapped == 2 {
					tally.taken[order]++
					return a
				}
			}
		}
		for _, name := range []string{"Mix Bone Altar", "Mix Prism Altar"} {
			have := bone
			if name == "Mix Prism Altar" {
				have = prism
			}
			if a, ok := cast[name]; ok && have == 0 && untapped >= 3 {
				return a
			}
		}
		if a, ok := cast["Mix Prey"]; ok && bone != 0 && prism != 0 && nCreatures == 0 {
			return a
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return first.Pick(c)
	})
}

// altarMixPool writes the mini corpus, builds its kernel table with kgen and
// returns the gorge registry and the set.
func altarMixPool(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, "cardsfolder", "x")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, src := range altarMixCards {
		if err := os.WriteFile(filepath.Join(folder, name+".txt"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	list := filepath.Join(dir, "cards.txt")
	names := "Mountain\nMix Prey\nMix Bone Altar\nMix Prism Altar\nMix Recall\n"
	if err := os.WriteFile(list, []byte(names), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", dir, "-card-list", list, "-pool", "altarmix", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "altarmix.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := cards.OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	return reg, cardset.AllowUnverified(cardset.NewDataSet(pd))
}

// altarMixCardsOf is decks as gorge deals them.
func altarMixCardsOf(t *testing.T, reg *cards.Registry, decks [2][]cardset.Entry) (gd [2][]*cards.Card) {
	t.Helper()
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the mini corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	return gd
}

func TestAltarMixedVictimLockstep(t *testing.T) {
	t.Parallel()
	reg, set := altarMixPool(t)
	tally := &altarMixTally{}
	for _, decks := range [][2][]cardset.Entry{
		altarMixDecks("Mix Bone Altar", "Mix Prism Altar"),
		altarMixDecks("Mix Prism Altar", "Mix Bone Altar"),
	} {
		tbl, _, _, err := set.Matchup(decks[0], decks[1])
		if err != nil {
			t.Fatal(err)
		}
		gd := altarMixCardsOf(t, reg, decks)
		for seed := uint64(1); seed <= 20; seed++ {
			cfg := Config{GorgeAutopaySurface: true}
			m, err := NewMatchup(reg, [2]string{"altarmix-a", "altarmix-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, altarMixPolicy(tally), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 40})
			CloseSide(k)
			if err := unmodelledStallError(t.Name(), &res); err != nil {
				t.Errorf("%s first, seed %d: %v", decks[0][1].Name, seed, err)
			}
			if d := res.Div; d != nil {
				t.Fatalf("%s first, seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", decks[0][1].Name, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%+v", *tally)
	for order, name := range []string{"colourless", "any-colour"} {
		if tally.ready[order] == 0 {
			t.Fatalf("precondition: no ready board with the %s altar first", name)
		}
		if tally.offered[order] != tally.ready[order] {
			t.Errorf("%s altar first: the cast was offered at %d of %d ready decisions", name, tally.offered[order], tally.ready[order])
		}
		if tally.taken[order] == 0 {
			t.Errorf("%s altar first: the offered cast was never taken", name)
		}
	}
}
