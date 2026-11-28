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

// One "Sacrifice a creature: Add one mana of any color" altar beside one
// creature that is the spell's only target, and MORE untapped identical lands
// than the {2}{U} spell's generic part needs (CR 601.2g-h): the altar pays the
// {U} by sacrificing the target and the lands pay the {2}. Which identical land
// is tapped is the planner's choice, and both engines must make the same one.
// No synthetic pool holds the altar, so the mini corpus is inline.

var prismSpareCards = map[string]string{
	"mountain":   "Name:Mountain\nManaCost:no cost\nTypes:Basic Land Mountain\nOracle:({T}: Add {R}.)\n",
	"spare_prey": "Name:Spare Prey\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:\n",
	"spare_prism_altar": "Name:Spare Prism Altar\nManaCost:3\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ Any | SpellDescription$ x\n" +
		"Oracle:x\n",
	"spare_recall": "Name:Spare Recall\nManaCost:2 U\nTypes:Instant\n" +
		"A:SP$ ChangeZone | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Hand | SpellDescription$ x\n" +
		"Oracle:x\n",
}

var prismSpareDecks = [2][]cardset.Entry{
	{
		{Name: "Mountain", Count: 18}, {Name: "Spare Prism Altar", Count: 12},
		{Name: "Spare Prey", Count: 5}, {Name: "Spare Recall", Count: 5},
	},
	{{Name: "Mountain", Count: 40}},
}

// prismSpareTally counts the casts taken by how many of the caster's lands
// were untapped: [0] exactly two (no choice of land), [1] three or more; split
// counts the casts whose two lowest-id untapped lands are not the first two
// untapped in battlefield order (the planner's ObjID order and the CR 601.2g
// window's battlefield order would tap different lands).
type prismSpareTally struct {
	taken [2]int
	split int
}

// prismSparePolicy develops the board (lands, the altar paid from them, then
// one creature) and casts the spell whenever it is offered beside the altar,
// one creature and two or more untapped lands.
func prismSparePolicy(tally *prismSpareTally) Policy {
	first := FirstPolicy()
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority {
			return first.Pick(c)
		}
		g := c.G.E.G
		seat := state.PlayerID(c.G.Pending().Player)
		var altar state.ObjID
		var nCreatures, untapped int
		var open []state.ObjID // the caster's untapped lands, battlefield order
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				switch o := g.Obj(id); o.Face().Name {
				case "Spare Prism Altar":
					altar = id
				case "Spare Prey":
					nCreatures++
				case "Mountain":
					if state.PlayerID(p) == seat && !o.Tapped {
						untapped++
						open = append(open, id)
					}
				}
			}
		}
		cast := map[string]Canon{}
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActPlayLand:
				return a
			case ActCast:
				cast[c.G.M.InstanceCard(int(a.Src()))] = a
			}
		}
		if a, ok := cast["Spare Recall"]; ok && altar != 0 && nCreatures == 1 && untapped >= 2 {
			if untapped == 2 {
				tally.taken[0]++
			} else {
				tally.taken[1]++
				if !firstTwoLowest(open) {
					tally.split++
				}
			}
			return a
		}
		if a, ok := cast["Spare Prism Altar"]; ok && altar == 0 && untapped >= 3 {
			return a
		}
		if a, ok := cast["Spare Prey"]; ok && altar != 0 && nCreatures == 0 {
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

// firstTwoLowest reports whether ids' first two entries are its two lowest.
func firstTwoLowest(ids []state.ObjID) bool {
	for _, id := range ids[2:] {
		if id < ids[0] || id < ids[1] {
			return false
		}
	}
	return true
}

// prismSparePool writes the mini corpus, builds its kernel table with kgen and
// returns the gorge registry and the set.
func prismSparePool(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, "cardsfolder", "x")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, src := range prismSpareCards {
		if err := os.WriteFile(filepath.Join(folder, name+".txt"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	list := filepath.Join(dir, "cards.txt")
	names := "Mountain\nSpare Prey\nSpare Prism Altar\nSpare Recall\n"
	if err := os.WriteFile(list, []byte(names), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", dir, "-card-list", list, "-pool", "prismspare", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "prismspare.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := cards.OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	return reg, cardset.AllowUnverified(cardset.NewDataSet(pd))
}

func TestAltarPrismSpareLandLockstep(t *testing.T) {
	t.Parallel()
	reg, set := prismSparePool(t)
	tbl, _, _, err := set.Matchup(prismSpareDecks[0], prismSpareDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range prismSpareDecks {
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
	tally := &prismSpareTally{}
	for seed := uint64(1); seed <= 12; seed++ {
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{"prismspare-a", "prismspare-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, prismSparePolicy(tally), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%+v", *tally)
	if tally.taken[1] == 0 || tally.split == 0 {
		t.Fatalf("precondition: no cast with a spare untapped land out of ObjID order: %+v", *tally)
	}
}
