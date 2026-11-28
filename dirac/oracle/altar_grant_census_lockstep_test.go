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

// Two "Sacrifice a creature: Add" altars and one creature are one altar's worth
// of mana to a payment plan, which names a distinct victim for each such step
// (CR 601.2g-h). A cast that the census of both altars would pay, but that only
// a granted mana ability really funds beside one altar, is therefore no planned
// cast: the two engines must list the same options at every such decision.
var altarGrantCensusCards = map[string]string{
	"Mountain":         "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n",
	"Census Prey":      "Name:Census Prey\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n",
	"Census Altar":     "Name:Census Altar\nManaCost:0\nTypes:Artifact\nA:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ C | Amount$ 2 | SpellDescription$ x\nOracle:x\n",
	"Census Recall":    "Name:Census Recall\nManaCost:6\nTypes:Instant\nA:SP$ Draw | NumCards$ 1 | SpellDescription$ x\nOracle:x\n",
	"Census Reservoir": "Name:Census Reservoir\nManaCost:1\nTypes:Artifact\nS:Mode$ Continuous | Affected$ Land.YouCtrl | AddAbility$ CensusTwo | Description$ x\nSVar:CensusTwo:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | SpellDescription$ x\nOracle:x\n",
}

// altarGrantCensusPool writes the mini corpus, builds its kernel table with
// kgen and returns the gorge registry and the set.
func altarGrantCensusPool(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, "cardsfolder", "x")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	names := ""
	for _, name := range []string{"Mountain", "Census Prey", "Census Altar", "Census Recall", "Census Reservoir"} {
		if err := os.WriteFile(filepath.Join(folder, name+".txt"), []byte(altarGrantCensusCards[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		names += name + "\n"
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(names), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", dir, "-card-list", list, "-pool", "altarcensus", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "altarcensus.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := cards.OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	return reg, cardset.AllowUnverified(cardset.NewDataSet(pd))
}

// altarGrantCensusPolicy develops three lands, the grant, both altars and one
// creature, never casts the spell, and counts in ready the priority decisions
// where that board stands with all three lands untapped and the spell in hand,
// and in offered those of them whose options cast it. The granted lands alone
// make the spell's {6}; the printed lands and one altar make only 5.
func altarGrantCensusPolicy(ready, offered *int) Policy {
	first := FirstPolicy()
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority {
			return first.Pick(c)
		}
		g := c.G.E.G
		seat := state.PlayerID(c.G.Pending().Player)
		count := map[string]int{}
		untapped := 0
		for _, id := range g.Zone(state.ZBattlefield, seat) {
			o := g.Obj(id)
			count[o.Face().Name]++
			if o.Face().Name == "Mountain" && !o.Tapped {
				untapped++
			}
		}
		inHand := false
		for _, id := range g.Zone(state.ZHand, seat) {
			inHand = inHand || g.Obj(id).Face().Name == "Census Recall"
		}
		if inHand && untapped == 3 && count["Census Reservoir"] > 0 && count["Census Altar"] == 2 && count["Census Prey"] == 1 {
			*ready++
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && c.G.M.InstanceCard(int(a.Src())) == "Census Recall" {
					*offered++
					break
				}
			}
		}
		want := map[string]bool{
			"Census Reservoir": count["Census Reservoir"] == 0,
			"Census Altar":     count["Census Altar"] < 2,
			"Census Prey":      count["Census Altar"] == 2 && count["Census Prey"] == 0,
		}
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActPlayLand:
				if count["Mountain"] < 3 {
					return a
				}
			case ActCast:
				if want[c.G.M.InstanceCard(int(a.Src()))] {
					return a
				}
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return first.Pick(c)
	})
}

func TestAltarGrantCensusOneVictimLockstep(t *testing.T) {
	t.Parallel()
	reg, set := altarGrantCensusPool(t)
	deck := []cardset.Entry{
		{Name: "Mountain", Count: 14}, {Name: "Census Reservoir", Count: 6}, {Name: "Census Altar", Count: 8},
		{Name: "Census Prey", Count: 6}, {Name: "Census Recall", Count: 6},
	}
	decks := [2][]cardset.Entry{deck, {{Name: "Mountain", Count: 40}}}
	ready, offered := 0, 0
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	gd := altarMixCardsOf(t, reg, decks)
	for seed := uint64(1); seed <= 6; seed++ {
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{"altarcensus-a", "altarcensus-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, altarGrantCensusPolicy(&ready, &offered), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if ready == 0 {
		t.Fatal("precondition: no game reached two altars, one creature, the grant and the spell in hand")
	}
	if offered != 0 {
		t.Errorf("the cast was offered at %d of %d ready decisions: no payment plan names two victims", offered, ready)
	}
}
