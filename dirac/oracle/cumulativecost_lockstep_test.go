package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cumulative upkeep cost lockstep. A cumulative upkeep (CR 702.24a) is
// paid once per age counter with the printed cost: coloured mana (Phobian
// Phantasm {B}, Yavimaya Ants {G}{G}), a hybrid pip (Arctic Nishoba {G/W}),
// life (Dystopia, Morinfen), sacrificed lands or creatures (Polar Kraken,
// Phyrexian Soulgorger) or discarded cards (Vexing Sphinx). kgen lowers each
// to the age counter and one UnlessPaid the kernel scales by the counters;
// both engines decide every pay, decline, pip face, pick and cast together.
var kcumulativeCostDecks = [2][]cardset.Entry{
	{
		{Name: "Phobian Phantasm", Count: 3}, {Name: "Yavimaya Ants", Count: 3}, {Name: "Arctic Nishoba", Count: 3},
		{Name: "Dystopia", Count: 2}, {Name: "Morinfen", Count: 2}, {Name: "Polar Kraken", Count: 3},
		{Name: "Phyrexian Soulgorger", Count: 3}, {Name: "Vexing Sphinx", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 3}, {Name: "Forest", Count: 4},
		{Name: "Plains", Count: 2},
	},
	{
		{Name: "Phobian Phantasm", Count: 3}, {Name: "Yavimaya Ants", Count: 3}, {Name: "Arctic Nishoba", Count: 3},
		{Name: "Dystopia", Count: 2}, {Name: "Morinfen", Count: 2}, {Name: "Polar Kraken", Count: 3},
		{Name: "Phyrexian Soulgorger", Count: 3}, {Name: "Vexing Sphinx", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 3}, {Name: "Forest", Count: 4},
		{Name: "Plains", Count: 2},
	},
}

type cumulativeCostStats struct {
	games, divs, stalls int
	aged                map[string]int // a card observed on the battlefield with two or more age counters
	gone                map[string]int // a card that had age counters, in a graveyard
}

func cumulativeCostCheck(ref *GorgeSide, maxAge map[state.ObjID]int32, st *cumulativeCostStats) {
	for _, pl := range []state.PlayerID{0, 1} {
		for _, id := range ref.E.G.Zone(state.ZBattlefield, pl) {
			o := ref.E.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			if n := o.Counter("AGE"); n > maxAge[id] {
				maxAge[id] = n
				if n == 2 {
					st.aged[o.Face().Name]++
				}
			}
		}
		for _, id := range ref.E.G.Zone(state.ZGraveyard, pl) {
			if o := ref.E.G.Obj(id); o != nil && o.Face() != nil && maxAge[id] > 0 {
				st.gone[o.Face().Name]++
				delete(maxAge, id)
			}
		}
	}
}

func runCumulativeCost(t *testing.T, seeds int) (st cumulativeCostStats) {
	t.Helper()
	st = cumulativeCostStats{aged: map[string]int{}, gone: map[string]int{}}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cumulativecost")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kcumulativeCostDecks {
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
	tbl, _, _, err := set.Matchup(kcumulativeCostDecks[0], kcumulativeCostDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcumulativecost-a", "kcumulativecost-b"}, decks, 1)
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
		maxAge := map[state.ObjID]int32{}
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				cumulativeCostCheck(ref, maxAge, &st)
				return nil
			}})
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return st
}

// TestCumulativeUpkeepCostLockstep plays the cumulative upkeep cost shapes
// against gorge. It asserts its own precondition: a mana, a hybrid, a life and a
// discard card each reach two age counters, so the scaled ask is posed.
func TestCumulativeUpkeepCostLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCumulativeCost(t, 48)
	t.Logf("games=%d divs=%d stalls=%d aged2=%v gone=%v", st.games, st.divs, st.stalls, st.aged, st.gone)
	if st.games != 48 || st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d games, %d divergences, %d stalls", st.games, st.divs, st.stalls)
	}
	for _, name := range []string{"Phobian Phantasm", "Arctic Nishoba", "Dystopia", "Vexing Sphinx"} {
		if st.aged[name] == 0 {
			t.Fatalf("%s never reached two age counters: its scaled ask was never posed", name)
		}
	}
}
