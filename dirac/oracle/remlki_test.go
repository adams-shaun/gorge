package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The RememberedLKI$ lockstep: Tribute to Hunger's "you gain
// life equal to that creature's toughness" reads the creature as it last
// was (CR 608.2h), not the graveyard card's printed face. Deck A is the
// Tribute deck; deck B carries anthems, pumps and counters so the last-known
// toughness differs from the printed one. Twisted Justice (target player
// sacrifices, you draw cards equal to its power) and Xathrid Demon (an
// upkeep trigger sacrifices another creature, each opponent loses its
// power) carry the target-player and trigger paths of the same read.
var kremlkiDecks = [2][]cardset.Entry{
	{ // A: B
		{Name: "Tribute to Hunger", Count: 4}, {Name: "Doom Blade", Count: 2}, {Name: "Terror", Count: 2}, {Name: "Twisted Justice", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Vicious Clown", Count: 3}, {Name: "Xathrid Demon", Count: 2},
		{Name: "Swamp", Count: 14}, {Name: "Island", Count: 4},
	},
	{ // B: G/W P/T changers
		{Name: "Glorious Anthem", Count: 3}, {Name: "Crusade", Count: 2}, {Name: "Giant Growth", Count: 3}, {Name: "Might of the Meek", Count: 2},
		{Name: "Ajani's Pridemate", Count: 3}, {Name: "Rancor", Count: 2}, {Name: "Bonesplitter", Count: 2}, {Name: "Savannah Lions", Count: 4},
		{Name: "Elvish Mystic", Count: 3}, {Name: "Kor Skyfisher", Count: 1}, {Name: "Heroic Reinforcements", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Forest", Count: 6},
	},
}

type kremlkiStats struct {
	games, sacs, tributes, differ, tokens int
}

func kremlkiSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("remlki")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kremlkiDecks {
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
	return reg, set, decks
}

// kremlkiProbe counts, from gorge's own Entered list, the creatures the pool
// sacrifices (Tribute to Hunger, Twisted Justice, Xathrid Demon), the
// Tributes among them (the Active player gained life: nothing else in the
// pool gains) and those Tributes whose gain differed from the printed toughness.
type kremlkiProbe struct {
	stats *kremlkiStats
	turn  int
	seen  int
	life  [2]int
}

func (p *kremlkiProbe) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	gain := int(g.Players[g.Active&1].Life) - p.life[g.Active&1]
	if t := int(g.Turn); t != p.turn {
		p.turn, p.seen = t, 0
	}
	for ; p.seen < len(g.Entered); p.seen++ {
		en := g.Entered[p.seen]
		if !en.Sacrificed || !en.WasCreature {
			continue
		}
		p.stats.sacs++
		if o := g.Obj(en.Obj); o != nil && o.IsToken {
			p.stats.tokens++
		}
		if gain > 0 {
			p.stats.tributes++
		}
		if o := g.Obj(en.Obj); o != nil && o.Face() != nil && gain > 0 && gain != o.Face().Toughness() {
			p.stats.differ++
		}
	}
	p.life = [2]int{int(g.Players[0].Life), int(g.Players[1].Life)}
	return nil
}

func kremlkiRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *kremlkiStats) {
	t.Helper()
	reg, set, decks := kremlkiSetup(t)
	tbl, _, _, err := set.Matchup(kremlkiDecks[0], kremlkiDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &kremlkiStats{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kremlki-a", "kremlki-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pr := &kremlkiProbe{stats: st}
		pr.life = [2]int{20, 20}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: pr.check})
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Logf("seed %d stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !report {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

func TestRemlkiLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := kremlkiRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; %d creature sacrifices (%d tokens), %d of them Tributes, %d whose life gain differed from the printed toughness",
		st.games, divs, stalls, st.sacs, st.tokens, st.tributes, st.differ)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.tributes < 30 {
		t.Errorf("only %d Tribute sacrifices of a creature (want >= 30)", st.tributes)
	}
	if st.differ < 8 {
		t.Errorf("only %d sacrifices with a last-known toughness other than the printed one (want >= 8)", st.differ)
	}
}

// TestRemlkiMutation: summing the printed toughness instead of the last-known
// one must make the lockstep diverge.
func TestRemlkiMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugRemLKIPrinted = true
	defer func() { dirac.DebugRemLKIPrinted = false }()
	if divs, _, _ := kremlkiRun(t, 64, false); divs == 0 {
		t.Fatal("DebugRemLKIPrinted: no divergence in 64 seeds")
	}
}
