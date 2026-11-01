package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The granted-wither lockstep: Barbed Bloodletter's Pump
// KW$ Wither (until end of turn, on the creature it attaches to) must make
// that creature's combat damage to creatures -1/-1 counters (CR 702.80), as
// printed wither (Boggart Ram-Gang, the regression) does. Deck B has
// creatures with toughness 3-5 and +1/+1 counter sources, so the counters
// annihilate (CR 704.5q) and survive damage-less turns.
var kwitherDecks = [2][]cardset.Entry{
	{ // A: B/G, the Bloodletter
		{Name: "Barbed Bloodletter", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Dwynen's Elite", Count: 4},
		{Name: "Gnarlback Rhino", Count: 3}, {Name: "Elfsworn Giant", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 13},
	},
	{ // B: G/W, toughness 3-5 bodies, counter makers and printed wither
		{Name: "Gnarlback Rhino", Count: 3}, {Name: "Fecund Greenshell", Count: 2}, {Name: "Armasaur Guide", Count: 3},
		{Name: "Pridemalkin", Count: 3}, {Name: "Ajani's Pridemate", Count: 3}, {Name: "Cat Collector", Count: 3},
		{Name: "Boggart Ram-Gang", Count: 2}, {Name: "Pacifism", Count: 2},
		{Name: "Forest", Count: 11}, {Name: "Plains", Count: 8},
	},
}

type kwitherStats struct {
	games, hits int // hits: -1/-1 counters put on a defending creature on a turn its attacker had a granted wither
}

func kwitherSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("wither")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kwitherDecks {
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

// kwitherProbe counts, from gorge's own state and event stream, the -1/-1
// counters that landed on a creature of the non-active player on a turn the
// active player had a creature whose derived wither was granted (not printed
// on its face): only that creature's damage can have put them there, since
// the non-active player's own wither damage lands on the active player's
// creatures.
type kwitherProbe struct {
	stats   *kwitherStats
	turn    int
	granted bool
	evAt    int
}

func (p *kwitherProbe) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if t := int(g.Turn); t != p.turn {
		p.turn, p.granted = t, false
	}
	for _, id := range g.Zone(state.ZBattlefield, g.Active) {
		o := g.Obj(id)
		if o != nil && o.Face() != nil && o.Face().IsCreature() && !o.Face().HasKeyword("Wither") && ref.E.HasKeyword(id, "Wither") {
			p.granted = true
		}
	}
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.CounterChange || e.Counter != "M1M1" || e.Amount <= 0 || !p.granted {
			continue
		}
		if o := g.Obj(e.Obj); o != nil && o.Controller != g.Active {
			p.stats.hits++
		}
	}
	p.evAt = len(evs)
	return nil
}

// kwitherRun plays seeds 1..seeds with the probe, reporting divergences as
// test errors when report is set (the mutation run expects one and stays
// quiet).
func kwitherRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *kwitherStats) {
	t.Helper()
	reg, set, decks := kwitherSetup(t)
	tbl, _, _, err := set.Matchup(kwitherDecks[0], kwitherDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &kwitherStats{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwither-a", "kwither-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pr := &kwitherProbe{stats: st}
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

func TestWitherLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := kwitherRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; %d granted-wither hits (-1/-1 counters on a defending creature)", st.games, divs, stalls, st.hits)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.hits < 30 {
		t.Errorf("only %d granted-wither hits: the pool does not reach the mechanism often enough", st.hits)
	}
}

// TestWitherMutation: a witherKw that ignores the granted bit must make the
// lockstep diverge.
func TestWitherMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugWitherGrantIgnored = true
	defer func() { dirac.DebugWitherGrantIgnored = false }()
	if divs, _, _ := kwitherRun(t, 64, false); divs == 0 {
		t.Fatal("DebugWitherGrantIgnored: no divergence in 64 seeds")
	}
}
