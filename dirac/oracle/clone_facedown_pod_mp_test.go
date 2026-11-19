package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Four-seat pods where Malleable Impostor ("enter as a copy of a creature an
// opponent controls, except it's a Faerie Shapeshifter and has flying")
// copies a manifested card: the copy takes the face-down copiable values, a
// nameless 2/2 colourless Creature (CR 707.2, 707.3), plus its exceptions,
// and is face up (gorge state.Object.SetCopyFace; dirac copy_facedown.go).
// The class the four-seat pod sweep surfaced as "Tegwyll: the P/T differs".

var cloneFaceDownPodDeck = []cardset.Entry{
	{Name: "Malleable Impostor", Count: 10}, {Name: "Soul Summons", Count: 8},
	{Name: "Write into Being", Count: 6},
	{Name: "Island", Count: 10}, {Name: "Plains", Count: 8},
}

// cloneFaceDownPodPolicy casts and plays lands whenever offered, else random,
// counting the Impostors gorge shows as blank copies of a face-down card.
func cloneFaceDownPodPolicy(seed uint64, blank map[state.ObjID]bool) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					o := g.Obj(id)
					if o != nil && !o.FaceDown && cardName(o.Card) == "Malleable Impostor" && o.CopyFace != nil && o.CopyFace.Name == "" {
						blank[id] = true
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestFourSeatCloneFaceDownLockstep: four-seat games of the deck in every
// seat play in lockstep, with Impostors entering as copies of manifested
// cards.
func TestFourSeatCloneFaceDownLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var pool []*cards.Card
	for _, e := range cloneFaceDownPodDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		pool = append(pool, c)
	}
	pd, err := kgen.TableFromCards(reg, pool, "clonefacedown-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	deck := cloneFaceDownPodDeck
	tbl, ids, err := set.MatchupN([][]cardset.Entry{deck, deck, deck, deck})
	if err != nil {
		t.Fatal(err)
	}
	if dirac.TwoSeatOnly(tbl) {
		t.Fatalf("the table is still two-seat only: %v", dirac.TwoSeatCauses(tbl))
	}
	const seeds = 24
	total := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		blank := map[state.ObjID]bool{} // this game's (object ids restart each game)
		m, err := KernelMatchupN(reg, tbl, []string{"pod-a", "pod-b", "pod-c", "pod-d"}, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 40}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, cloneFaceDownPodPolicy(seed, blank), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d (%d blank Impostors this game): %s %s %v %s", seed, d.At, len(blank), d.What, d.Field, d.Diff, d.Detail)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		total += len(blank)
	}
	t.Logf("%d games, %d Impostors entered as blank copies of a face-down card", seeds, total)
	if total < 5 {
		t.Errorf("only %d Impostors entered as blank copies, want >= 5", total)
	}
}
