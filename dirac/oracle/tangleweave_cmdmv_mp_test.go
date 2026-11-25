package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// Tangleweave Armor: "Equipped creature gets +X/+X, where X is the greatest
// mana value among your commanders" (Count$ValidAll Card.IsCommander+YouOwn
// $GreatestCardManaCost: gorge's every-zone fold; kernel ExCmdGreatestMV
// N CmdMVAllZones). Four-seat Commander pods (Ghalta, Primal Hunger, mana
// value 12, in every command zone) play in lockstep, and some Germ the Armor
// makes is at least 12/12.
func TestFourSeatTangleweaveCommanderMVLockstep(t *testing.T) {
	t.Parallel()
	base, _ := testCorpus(t)
	c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join("..", "..", "testkit", "testutil", "decks"), decks: map[string][]*cards.Card{}}
	f := deck.File{Name: "tangleweave", Format: "commander", Commander: "Ghalta, Primal Hunger", Cards: []deck.Entry{
		{Name: "Ghalta, Primal Hunger", Count: 1}, {Name: "Tangleweave Armor", Count: 12}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Forest", Count: 21},
	}}
	c.AddDecks(map[string]deck.File{"tangleweave": f})
	pd, err := kgen.SynthUnionTable([]string{"Ghalta, Primal Hunger", "Tangleweave Armor", "Llanowar Elves"}, corpusDirOrSkip(t), "tangleweave-oracle")
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.Raw(cardset.NewDataSet(pd)))
	names := []string{"tangleweave", "tangleweave", "tangleweave", "tangleweave"}
	big := 0
	for seed := uint64(1); seed <= 8; seed++ {
		m, tbl, err := SetMatchupN(set, c, names, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !m.Commander {
			t.Fatal("not a Commander matchup")
		}
		rc := RunConfig{MaxDecisions: 3000, MaxTurns: 30}
		rc.Finish = func(g *GorgeSide) {
			for i := range g.E.G.Objs {
				o := &g.E.G.Objs[i]
				if o.Face() != nil && strings.Contains(o.Face().Name, "Germ") && g.E.IsCreature(o.ID) && g.E.Toughness(o.ID) >= 12 {
					big++
					return
				}
			}
		}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v\nref %q\nker %q\n%s", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions, d.Detail)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	if big == 0 {
		t.Fatal("no game made a Germ of at least 12/12")
	}
}
