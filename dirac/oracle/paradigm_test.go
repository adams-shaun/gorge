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

// The Paradigm lockstep (gorge rules/paradigm.go): the three
// sole SOS Paradigm carriers (a resolved one goes to exile and its name's
// free cast from exile opens in its owner's first main phases) against
// Counterspell (a countered one reaches the graveyard and opens nothing),
// with cheap creatures for Restoration Seminar's and Germination
// Practicum's targets. Gorge's answer at every decision is the expected one.
var kparadigmDecks = [2][]cardset.Entry{
	{ // A: W/B/G, the carriers
		{Name: "Decorum Dissertation", Count: 4}, {Name: "Germination Practicum", Count: 4}, {Name: "Restoration Seminar", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Walking Corpse", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5},
	},
	{ // B: U, the counterspell
		{Name: "Counterspell", Count: 4}, {Name: "Coral Merfolk", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Island", Count: 24},
	},
}

var kparadigmNames = map[string]bool{"Decorum Dissertation": true, "Germination Practicum": true, "Restoration Seminar": true}

type kparadigmStats struct {
	games, freeCasts, resolved, countered int
}

func kparadigmSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("paradigm")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kparadigmDecks {
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

// kparadigmProbe counts the arms from gorge's event log: a Paradigm card
// put on the stack from exile (the free cast), moved stack -> exile (a
// resolution) and stack -> graveyard (countered). The cursor rewinds when a
// side is rebuilt.
type kparadigmProbe struct {
	stats *kparadigmStats
	evAt  int
}

func (p *kparadigmProbe) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		o := g.Obj(e.Obj)
		if o == nil || o.Face() == nil || !kparadigmNames[o.Face().Name] {
			continue
		}
		switch {
		case e.Kind == events.PutOnStack && e.From == state.ZExile:
			p.stats.freeCasts++
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZExile:
			p.stats.resolved++
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZGraveyard:
			p.stats.countered++
		}
	}
	p.evAt = len(evs)
	return nil
}

// kparadigmRun plays seeds 1..seeds with the probe, reporting divergences
// as test errors when report is set (the mutation run expects one and stays
// quiet).
func kparadigmRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *kparadigmStats) {
	t.Helper()
	reg, set, decks := kparadigmSetup(t)
	tbl, _, _, err := set.Matchup(kparadigmDecks[0], kparadigmDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &kparadigmStats{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kparadigm-a", "kparadigm-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pr := &kparadigmProbe{stats: st}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Check: pr.check})
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

func TestParadigmLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := kparadigmRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; %d free Paradigm casts from exile, %d resolved to exile, %d countered to the graveyard",
		st.games, divs, stalls, st.freeCasts, st.resolved, st.countered)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.freeCasts < 40 {
		t.Errorf("%d free Paradigm casts from exile, want >= 40", st.freeCasts)
	}
	if st.countered < 10 {
		t.Errorf("%d countered Paradigm spells, want >= 10", st.countered)
	}
}

// TestParadigmMutation: the free cast offered in main phase 2 too must make
// the lockstep diverge.
func TestParadigmMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugParadigmAnyMain = true
	defer func() { dirac.DebugParadigmAnyMain = false }()
	if divs, _, _ := kparadigmRun(t, 64, false); divs == 0 {
		t.Fatal("DebugParadigmAnyMain: no divergence in 64 seeds")
	}
}
