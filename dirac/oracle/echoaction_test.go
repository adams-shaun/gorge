package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The echo action-cost lockstep. Echo (CR 702.30a) is normally paid with mana,
// but Deepcavern Imp and Rakdos Headliner say "Echo—Discard a card" and
// Skizzik Surger says "Echo—Sacrifice two lands": gorge's echoElectionAsk
// offers echo_pay / echo_sac, and echo_pay poses echoActionAsk, a KChoose of
// exactly N cards in hand or N lands. kgen lowers those two cost shapes to an
// UnlessPaid Discard / Sacrifice after ExtCumulative B=EchoNoCounter; the
// kernel asks the same pay-or-decline, then the N-object pick. Both engines
// decide every pay, decline, cast and attack together.
var kechoActionDecks = [2][]cardset.Entry{
	{
		{Name: "Deepcavern Imp", Count: 4}, {Name: "Rakdos Headliner", Count: 4}, {Name: "Skizzik Surger", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Deepcavern Imp", Count: 4}, {Name: "Rakdos Headliner", Count: 4}, {Name: "Skizzik Surger", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

type echoActionStats struct {
	games, divs, stalls int
	paid, gone          map[string]int // an echo card still on the battlefield / in a graveyard two turns after it first appeared
}

func newEchoActionStats() echoActionStats {
	return echoActionStats{paid: map[string]int{}, gone: map[string]int{}}
}

func echoActionCheck(ref *GorgeSide, first map[state.ObjID]int32, st *echoActionStats) {
	turn := ref.E.G.Turn
	for _, pl := range []state.PlayerID{0, 1} {
		for _, id := range ref.E.G.Zone(state.ZBattlefield, pl) {
			o := ref.E.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Deepcavern Imp", "Rakdos Headliner", "Skizzik Surger":
				if t, ok := first[id]; !ok {
					first[id] = turn
				} else if turn == t+2 {
					st.paid[o.Face().Name]++
				}
			}
		}
		for _, id := range ref.E.G.Zone(state.ZGraveyard, pl) {
			o := ref.E.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			if t, ok := first[id]; ok && turn == t+2 {
				st.gone[o.Face().Name]++
				delete(first, id)
			}
		}
	}
}

func runEchoAction(t *testing.T, seeds int) (st echoActionStats) {
	t.Helper()
	st = newEchoActionStats()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("echoaction")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kechoActionDecks {
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
	tbl, _, _, err := set.Matchup(kechoActionDecks[0], kechoActionDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kechoaction-a", "kechoaction-b"}, decks, 1)
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
		first := map[state.ObjID]int32{}
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				echoActionCheck(ref, first, &st)
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

// TestEchoActionLockstep plays the two action echo shapes against gorge. It
// asserts its own preconditions: Deepcavern Imp and Skizzik Surger each reach
// a paid echo (still on the battlefield two turns after first appearing) and a
// declined one (in a graveyard two turns after).
func TestEchoActionLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runEchoAction(t, 64)
	t.Logf("games=%d divs=%d stalls=%d paid=%v gone=%v", st.games, st.divs, st.stalls, st.paid, st.gone)
	if st.games != 64 || st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d games, %d divergences, %d stalls", st.games, st.divs, st.stalls)
	}
	for _, name := range []string{"Deepcavern Imp", "Skizzik Surger"} {
		if st.paid[name] == 0 || st.gone[name] == 0 {
			t.Fatalf("%s never reached both echo outcomes: %d paid, %d gone", name, st.paid[name], st.gone[name])
		}
	}
}
