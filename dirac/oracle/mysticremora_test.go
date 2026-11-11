package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Mystic Remora lockstep: its cumulative upkeep {1} (CR 702.24a) puts an
// age counter on it at each of its controller's upkeeps and asks them to pay
// {1} per counter or sacrifice it (gorge's cumulative_pay / cumulative_sac),
// and its noncreature-spell trigger draws a card unless the caster pays {4}.
// kgen lowers the keyword to ExtCumulative + an UnlessPaid posed as the
// trigger's own cost; both engines decide every pay, decline, cast and
// attack together.

var kremoraDecks = [2][]cardset.Entry{
	{
		{Name: "Mystic Remora", Count: 8}, {Name: "Hill Giant", Count: 8},
		{Name: "Shock", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Mystic Remora", Count: 8}, {Name: "Hill Giant", Count: 8},
		{Name: "Shock", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

type remoraStats struct {
	games, divs, stalls int
	aged1, aged2, aged3 int // Remora observations at an agreed decision with that many age counters
	sacrificed          int // a Remora that had age counters, in a graveyard (the unpaid upkeep)
}

// remoraCheck reads gorge's board at every agreed decision: a Remora on the
// battlefield during its controller's turn carries at most one age counter
// per upkeep survived, and one that had counters and is now in a graveyard
// was sacrificed by a declined (or unpayable) upkeep.
func remoraCheck(ref *GorgeSide, maxAge map[state.ObjID]int32, st *remoraStats) {
	for _, pl := range []state.PlayerID{0, 1} {
		for _, id := range ref.E.G.Zone(state.ZBattlefield, pl) {
			o := ref.E.G.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != "Mystic Remora" {
				continue
			}
			n := o.Counter("AGE")
			if n > maxAge[id] {
				maxAge[id] = n
			}
			switch {
			case n == 1:
				st.aged1++
			case n == 2:
				st.aged2++
			case n >= 3:
				st.aged3++
			}
		}
		for _, id := range ref.E.G.Zone(state.ZGraveyard, pl) {
			if o := ref.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mystic Remora" && maxAge[id] > 0 {
				st.sacrificed++
				delete(maxAge, id)
			}
		}
	}
}

func runRemora(t *testing.T, seeds int) (st remoraStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mysticremora")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kremoraDecks {
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
	tbl, _, _, err := set.Matchup(kremoraDecks[0], kremoraDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kremora-a", "kremora-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				remoraCheck(ref, maxAge, &st)
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

func TestMysticRemoraLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runRemora(t, 64)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	// preconditions: the games reached the upkeep payments and both outcomes
	if st.aged1 == 0 || st.aged2 == 0 || st.sacrificed == 0 {
		t.Fatalf("the games never reached a paid second upkeep or an unpaid one: %+v", st)
	}
}
