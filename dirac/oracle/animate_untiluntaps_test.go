package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Hedge Whisperer lockstep: "{3}{G}, {T}, collect evidence 4: target land
// you control becomes a 5/5 green Plant Boar creature with haste for as long
// as Hedge Whisperer remains tapped" (Animate Duration$ UntilUntaps, CR
// 611.2b) beside "you may choose not to untap CARDNAME during your untap
// step". The kernel's WhileTapped override must animate the land exactly
// while gorge's UntilUntapOf registration lives: through cleanup and a kept
// untap step, ending at the host's untap or departure.

var khedgeDecks = [2][]cardset.Entry{
	{
		{Name: "Hedge Whisperer", Count: 8}, {Name: "Crow of Dark Tidings", Count: 6}, {Name: "Burglar Rat", Count: 4},
		{Name: "Hill Giant", Count: 2}, {Name: "Forest", Count: 10}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Burglar Rat", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}

// hedgeStats are what a run of games reached, read off gorge's board at
// every agreed decision.
type hedgeStats struct {
	games, divs, stalls int
	animated            int // decisions with an animated land on the battlefield
	carried             int // ... on a later turn than the one it was first seen animated (it survived cleanup)
	attacked            int // ... attacking
	reverted            int // animated lands seen a plain land again while still on the battlefield
	keeps, untaps       int // keep-tapped elections answered each way
}

func runHedge(t *testing.T, seeds int) (st hedgeStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("hedgewhisperer")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range khedgeDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(khedgeDecks[0], khedgeDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	keepCode, ok1 := canon.ChooseCode("keep_tapped")
	untapCode, ok2 := canon.ChooseCode("untap")
	if !ok1 || !ok2 {
		t.Fatal("keep_tapped / untap are not in the choose vocabulary")
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"khedge-a", "khedge-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		first := map[state.ObjID]int32{} // an animated land's first turn seen
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				for _, id := range ref.E.G.Zone(state.ZBattlefield, 0) {
					o := ref.E.G.Obj(id)
					if o == nil || o.Face() == nil || !o.Face().IsLand() {
						continue
					}
					if ref.E.IsCreature(id) {
						st.animated++
						if f, ok := first[id]; !ok {
							first[id] = ref.E.G.Turn
						} else if ref.E.G.Turn > f {
							st.carried++
						}
						if o.IsAttacking {
							st.attacked++
						}
					} else if _, ok := first[id]; ok {
						st.reverted++
						delete(first, id)
					}
				}
				return nil
			}})
		CloseSide(k)
		st.games++
		for _, a := range res.Actions {
			switch a.Kind() {
			case canon.ActChoose + canon.CanonKind(keepCode):
				st.keeps++
			case canon.ActChoose + canon.CanonKind(untapCode):
				st.untaps++
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

func TestAnimateUntilUntapsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runHedge(t, 128)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	// the animation is reached, attacks, outlives its turn while Hedge
	// Whisperer stays tapped, and ends when it untaps
	if st.animated < 50 || st.attacked < 5 || st.carried < 20 || st.reverted < 5 {
		t.Errorf("animated %d, attacked %d, carried %d, reverted %d (want at least 50, 5, 20, 5)", st.animated, st.attacked, st.carried, st.reverted)
	}
	if st.keeps < 10 || st.untaps < 10 {
		t.Errorf("keep-tapped elections: %d kept, %d untapped (want at least 10 each)", st.keeps, st.untaps)
	}
}
