package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// The MBC chosen-player-card-1 family: the as-enters chosen opponent of The
// Rack and Black Vise (kgen lower_chooseplayerenter.go, kernel
// repl_chooseplayer.go), played from the chosenopp fixture pool.

// chosenOppPool is the chosenopp fixture pool; the test fails when it does
// not play every card it holds (kgen refuses one).
func chosenOppPool(t *testing.T) cardset.Set {
	t.Helper()
	corpusDirOrSkip(t)
	set, err := cardset.RawByName("chosenopp")
	if err != nil {
		t.Fatal(err)
	}
	sup := set.Supported()
	for _, name := range []string{"The Rack", "Black Vise", "Island", "Plains"} {
		if i, ok := set.Lookup(name); !ok || i >= len(sup) || !sup[i] {
			t.Fatalf("the chosenopp pool does not play %s: %s", name, set.Reason(i))
		}
	}
	return set
}

// chosenOppTally counts, on gorge's side, the watched cards' triggers and the
// player damage that followed one before the next priority.
type chosenOppTally struct{ trig, hits int }

func (y *chosenOppTally) read(g *GorgeSide, watch ...string) {
	evs := g.E.L.Events
	for i := range evs {
		if evs[i].Kind != events.TriggerPush || !slices.Contains(watch, firstFaceName(g.E.G, evs[i].Obj)) {
			continue
		}
		y.trig++
		for j := i + 1; j < len(evs) && evs[j].Kind != events.TriggerPush && evs[j].Kind != events.TurnChange; j++ {
			if evs[j].Kind == events.Damage && evs[j].Obj == 0 && evs[j].Amount > 0 {
				y.hits++
				break
			}
		}
	}
}

// TestMBCChosenPlayerCard1Lowers plays two-seat games of The Rack and Black
// Vise against a hand that never empties: each chooses the one opponent as
// it enters (asked of nobody: gorge's single-option ChoosePlayer is answered
// in place), triggers on that player's upkeep only (ValidPlayer$
// Player.Chosen) and deals damage from that player's hand size
// (Count$ValidHand Card.ChosenCtrl). No divergence, and the triggers fired
// and dealt damage.
func TestMBCChosenPlayerCard1Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	watch := []string{"The Rack", "Black Vise"}
	set := chosenOppPool(t)
	vise := deckOf(10, "Black Vise", deckOf(10, "The Rack", deckOf(20, "Island")...)...)
	lands := deckOf(40, "Plains")
	decks := [2][]string{vise, lands}
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	var y chosenOppTally
	for seed := uint64(1); seed <= 6; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: decks, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		rc.Finish = func(g *GorgeSide) { y.read(g, watch...) }
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
	}
	if y.trig == 0 || y.hits == 0 {
		t.Fatalf("the chosen player's upkeep triggers did not fire and hit: %+v", y)
	}
	t.Logf("%+v", y)
}
