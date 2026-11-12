package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// H.M.S. Bounty, Unlikely Timeship's STATION 8 grants flying and a combat
// damage trigger: draw a card, then AddTurn$ You only while a Whale is
// controlled and Count$YourExtraTurns is 0. The kernel's ExYourExtraTurns
// is a per-seat bit set when an extra turn actually begins (CR 500.7), so
// the second connect in the extra turn must not grant another one.
var bountyDecks = [2][]cardset.Entry{
	{
		{Name: "H.M.S. Bounty, Unlikely Timeship", Count: 4}, {Name: "Long-Finned Skywhale", Count: 8}, {Name: "Craw Wurm", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Island", Count: 8},
	},
	{
		{Name: "Wall of Wood", Count: 20}, {Name: "Forest", Count: 20},
	},
}

type kbountyStats struct {
	stations     int // station special actions
	picks        int // the station tap picks they posed
	attacks      int // Bounty attacks declared
	extraTurns   int // extra turns actually begun (same player active twice)
	gatedAttacks int // Bounty attacks by a player whose extra turn already began
}

type kbountyProbe struct {
	base  Policy
	r     *rand.Rand
	st    *kbountyStats
	seen  bool
	turn  int32
	act   uint8
	begun [2]bool
}

func (p *kbountyProbe) pick(c *PickCtx) Canon {
	if gs := c.G; gs != nil {
		g := gs.E.G
		if !p.seen {
			p.seen, p.turn, p.act = true, g.Turn, uint8(g.Active)
		}
		if g.Turn != p.turn {
			if uint8(g.Active) == p.act {
				p.st.extraTurns++
				p.begun[g.Active&1] = true
			}
			p.turn, p.act = g.Turn, uint8(g.Active)
		}
	}
	var special, attacks []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActSpecial:
			special = append(special, a)
		case ActAttack:
			attacks = append(attacks, a)
		}
	}
	if len(special) > 0 && p.r.IntN(4) != 0 {
		return special[p.r.IntN(len(special))]
	}
	if len(attacks) > 0 && p.r.IntN(8) != 0 {
		a := attacks[p.r.IntN(len(attacks))]
		if c.G != nil && refCardName(c.G, a.Src()) == "H.M.S. Bounty, Unlikely Timeship" {
			p.st.attacks++
			if o := c.G.E.G.Obj(c.G.id.obj[a.Src()]); o != nil && p.begun[uint8(o.Controller)&1] {
				p.st.gatedAttacks++
			}
		}
		return a
	}
	return p.base.Pick(c)
}

// runBounty plays the constructed matchup in both seat orders.
func runBounty(t *testing.T, seeds int, report bool) (games, divs, stalls int, st kbountyStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("bounty")
	if err != nil {
		t.Fatal(err)
	}
	stationPick, _ := chooseCode("station")
	special, _ := specialCode("station")
	for order := 0; order < 2; order++ {
		md := [2][]cardset.Entry{bountyDecks[order], bountyDecks[1-order]}
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
				t.Fatalf("order %d deck %d: %d cards", order, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"bounty-a", "bounty-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			probe := &kbountyProbe{base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x626f756e)), st: &st}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if res.Stall != "" {
				stalls++
				if res.StallUnmodelled {
					t.Errorf("order %d seed %d: %s at turn %d", order, seed, res.Stall, res.Turns)
				} else if report {
					t.Logf("order %d seed %d: %s at turn %d", order, seed, res.Stall, res.Turns)
				}
			}
			for _, a := range res.Actions {
				switch {
				case a.Kind() == ActSpecial && a.Mode() == special:
					st.stations++
				case a.Kind() == ActChoose+CanonKind(stationPick):
					st.picks++
				}
			}
			if d := res.Div; d != nil {
				divs++
				if report {
					lo := max(d.At-12, 0)
					t.Logf("order %d seed %d last actions %v", order, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("order %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", order, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestBountyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runBounty(t, 48, true)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if st.stations == 0 || st.picks != st.stations {
		t.Errorf("stations %d, picks %d: every station must be answered by one pick", st.stations, st.picks)
	}
	if st.extraTurns == 0 {
		t.Error("no extra turn begun")
	}
	if st.attacks == 0 || st.gatedAttacks == 0 {
		t.Errorf("Bounty attacks %d, after an extra turn %d: want the gated second connect", st.attacks, st.gatedAttacks)
	}
}

// TestBountyMutation: without the per-seat begun bit, the second connect
// reads 0 and grants another extra turn, so the lockstep must diverge.
func TestBountyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoExtraTurnBegun = true
	defer func() { dirac.DebugNoExtraTurnBegun = false }()
	games, divs, _, _ := runBounty(t, 24, false)
	if divs == 0 {
		t.Fatalf("DebugNoExtraTurnBegun: no divergence in %d games", games)
	}
}
