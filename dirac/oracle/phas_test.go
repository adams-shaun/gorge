package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The PlayerCount HasCardsIn lockstep: Wojek
// Investigator's "more cards in hand than you" Investigate Num$ (a 0/1
// Compare), Teysa's lost-life Investigate Num$, Master's Councillors' and
// The Master of Lake-town's graveyard-of-seven counts, and Aclazotz's
// Temple of the Dead transform (PlayerCount hand <= 1), over hand-shrinking
// and milling cards so the thresholds are crossed both ways.

var kphasDecks = [][2][]cardset.Entry{{
	{
		{Name: "Wojek Investigator", Count: 4}, {Name: "Teysa, Opulent Oligarch", Count: 4}, {Name: "Master's Councillors", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Aclazotz, Deepest Betrayal", Count: 4}, {Name: "The Master of Lake-town", Count: 4}, {Name: "Mind Rot", Count: 4},
		{Name: "Divination", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 4},
	},
}}

type kphasStats struct {
	games, divs, stalls int
	clues               int // distinct Clue tokens seen on gorge's battlefield
	temples             int // Temple of the Dead -> Aclazotz transforms
	lakeTown            int // Master of Lake-town deaths with a 7-card graveyard on the table
}

// kphasSeen is the per-game observation memory (ids of what was counted).
type kphasSeen struct {
	clue, lake map[state.ObjID]bool
	name       map[state.ObjID]string
}

// kphasPolicy casts, activates and attacks whenever offered, else random; it
// observes gorge's battlefield and graveyards at every pick.
func kphasPolicy(seed uint64, st *kphasStats) Policy {
	rnd := RandomPolicy(seed)
	seen := kphasSeen{clue: map[state.ObjID]bool{}, lake: map[state.ObjID]bool{}, name: map[state.ObjID]string{}}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			big := false
			for p := range g.Players {
				if len(g.Zone(state.ZGraveyard, state.PlayerID(p))) >= 7 {
					big = true
				}
			}
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					ob := g.Obj(id)
					if ob == nil || ob.Face() == nil {
						continue
					}
					n := ob.Face().Name
					if strings.HasPrefix(n, "Clue") && !seen.clue[id] {
						seen.clue[id] = true
						st.clues++
					}
					if prev := seen.name[id]; prev == "Temple of the Dead" && n == "Aclazotz, Deepest Betrayal" {
						st.temples++
					}
					seen.name[id] = n
				}
				for _, id := range g.Zone(state.ZGraveyard, state.PlayerID(p)) {
					if ob := g.Obj(id); big && ob != nil && ob.Face() != nil && ob.Face().Name == "The Master of Lake-town" && !seen.lake[id] {
						seen.lake[id] = true
						st.lakeTown++
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActAttack:
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

// runPhas plays the constructed matchup over seeds 1..seeds; mut sets the
// DebugHasCardsGE mutation.
func runPhas(t *testing.T, seeds int, mut bool) kphasStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("phas")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugHasCardsGE
	dirac.DebugHasCardsGE = mut
	defer func() { dirac.DebugHasCardsGE = old }()
	var st kphasStats
	for mi, md := range kphasDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kphas-a", "kphas-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kphasPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			st.games++
			if d := res.Div; d != nil {
				st.divs++
				if !mut {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				st.stalls++
				if !mut {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return st
}

func TestPhasLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runPhas(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d clues, %d Temple transforms, %d Lake-town deaths with a 7-card graveyard", st.games, st.divs, st.stalls, st.clues, st.temples, st.lakeTown)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.clues < 30 {
		t.Errorf("only %d clues, want >= 30", st.clues)
	}
	if st.lakeTown < 5 {
		t.Errorf("only %d Lake-town deaths with a 7-card graveyard, want >= 5", st.lakeTown)
	}
}

// TestPhasMutation reads CmpGT as >= in the kernel: Wojek's "more cards
// than you" then fires on equal hands, so the games must diverge.
func TestPhasMutation(t *testing.T) {
	st := runPhas(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
