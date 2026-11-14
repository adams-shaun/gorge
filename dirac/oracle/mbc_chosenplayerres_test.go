package oracle

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The mid-resolution chosen player (kgen lower_chooseplayerres.go, kernel
// act_chooseplayerres.go): Gluntch, the Bestower's end step chooses a player
// to put two +1/+1 counters on a creature they control, a second to draw and,
// with three or more players, a third to create two Treasures. Played from the
// mbcchosenplayer fixture pool.

// gluntchPool is the mbcchosenplayer fixture pool; the test fails when it does
// not play every card it holds (kgen refuses one).
func gluntchPool(t *testing.T) cardset.Set {
	t.Helper()
	corpusDirOrSkip(t)
	set, err := cardset.RawByName("mbcchosenplayer")
	if err != nil {
		t.Fatal(err)
	}
	sup := set.Supported()
	for _, name := range []string{"Forest", "Plains", "Grizzly Bears", "Gluntch, the Bestower"} {
		if i, ok := set.Lookup(name); !ok || i >= len(sup) || !sup[i] {
			t.Fatalf("the mbcchosenplayer pool does not play %s: %s", name, set.Reason(i))
		}
	}
	return set
}

// gluntchTally is gorge's account of the end-step resolutions: one window
// per resolution, from its first "chosen" event to the Cleanup that clears
// the remembered list.
type gluntchTally struct {
	windows    int // resolutions seen
	counters   int // windows that put two +1/+1 counters on a creature
	draws      int // windows whose second choice drew a card
	treasures  int // Treasure tokens created by the third choice
	threeWay   int // windows that chose three players
	twoWay     int // windows that chose two
	violations []string
}

type gluntchWindow struct {
	src       state.ObjID
	chosen    []state.ObjID // the players chosen, as player refs, in order
	draws     []state.PlayerID
	tokens    []state.PlayerID
	counterTo []state.ObjID
}

func (y *gluntchTally) read(g *GorgeSide) {
	var w *gluntchWindow
	evs := g.E.L.Events
	for _, e := range evs {
		switch {
		case e.Kind == events.Choose && e.Counter == "chosen" && len(e.IDs) == 1 && e.IDs[0]&state.PlayerRef(0) == state.PlayerRef(0) && firstFaceName(g.E.G, e.Obj) == "Gluntch, the Bestower":
			if w == nil {
				w = &gluntchWindow{src: e.Obj}
			}
			w.chosen = append(w.chosen, e.IDs[0])
		case w == nil:
		case e.Kind == events.Draw:
			w.draws = append(w.draws, e.Player)
		case e.Kind == events.TokenCreate && strings.Contains(e.Text, "treasure"):
			w.tokens = append(w.tokens, e.Player)
		case e.Kind == events.CounterChange && strings.Contains(strings.ToUpper(e.Counter), "P1P1") && e.Amount == 2:
			w.counterTo = append(w.counterTo, e.Obj)
		case e.Kind == events.Choose && e.Counter == "clear-remembered" && e.Obj == w.src:
			y.close(g, w)
			w = nil
		}
	}
}

func (y *gluntchTally) close(g *GorgeSide, w *gluntchWindow) {
	y.windows++
	bad := func(f string, a ...any) { y.violations = append(y.violations, fmt.Sprintf(f, a...)) }
	n := len(w.chosen)
	switch n {
	case 2:
		y.twoWay++
	case 3:
		y.threeWay++
	default:
		bad("window %d chose %d players", y.windows, n)
		return
	}
	for i := range w.chosen {
		if slices.Contains(w.chosen[:i], w.chosen[i]) {
			bad("window %d chose a player twice: %v", y.windows, w.chosen)
		}
	}
	pl := func(i int) state.PlayerID { return state.PlayerID(w.chosen[i] &^ state.PlayerRef(0)) }
	if len(w.draws) != 1 || w.draws[0] != pl(1) {
		bad("window %d: the second choice is %d, draws %v", y.windows, pl(1), w.draws)
	} else {
		y.draws++
	}
	if len(w.counterTo) > 1 {
		bad("window %d: %d counter placements", y.windows, len(w.counterTo))
	}
	for _, o := range w.counterTo {
		y.counters++
		if ob := g.E.G.Obj(o); ob == nil || ob.Controller != pl(0) {
			bad("window %d: counters on %d, the first choice is %d", y.windows, o, pl(0))
		}
	}
	switch {
	case n == 2 && len(w.tokens) != 0:
		bad("window %d: %d treasures with two players", y.windows, len(w.tokens))
	case n == 3 && (len(w.tokens) != 2 || w.tokens[0] != pl(2) || w.tokens[1] != pl(2)):
		bad("window %d: the third choice is %d, treasures to %v", y.windows, pl(2), w.tokens)
	}
	y.treasures += len(w.tokens)
}

// gluntchDeck is every seat's deck: Gluntch, the creatures the counters go on
// and the lands to cast them.
func gluntchDeck() []string {
	return deckOf(4, "Gluntch, the Bestower", deckOf(14, "Grizzly Bears", deckOf(11, "Forest", deckOf(11, "Plains")...)...)...)
}

// playGluntch plays seeds 1..seeds of a seats-seat pod of gluntchDeck in
// lockstep, failing on a divergence or a kernel stop, and returns gorge's
// tally and the canonical actions taken.
func playGluntch(t *testing.T, seats int, seeds uint64) (gluntchTally, []Canon) {
	t.Helper()
	reg := synthReg(t)
	set := gluntchPool(t)
	deck := gluntchDeck()
	var lists [][]cardset.Entry
	for i := 0; i < seats; i++ {
		es := make([]cardset.Entry, len(deck))
		for j, n := range deck {
			es[j] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	var y gluntchTally
	var acts []Canon
	for seed := uint64(1); seed <= seeds; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{deck, deck}, Seed: seed, PassElision: true, Surface: "full"}
		for i := 2; i < seats; i++ {
			r.MoreDecks = append(r.MoreDecks, "x"+string(rune('0'+i)))
			r.MoreCards = append(r.MoreCards, deck)
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 4000, 30
		rc.Finish = y.read
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
		acts = append(acts, res.Actions...)
	}
	return y, acts
}

// checkGluntch fails on any resolution that broke the chain's rules and
// unless the first choice, the pick and the draw each happened.
func checkGluntch(t *testing.T, y gluntchTally, acts []Canon) {
	t.Helper()
	for _, v := range y.violations {
		t.Error(v)
	}
	pc, _ := chooseCode("player")
	kc, _ := chooseCode("counter_pick")
	asks, picks := 0, 0
	for _, a := range acts {
		switch a.Kind() {
		case ActChoose + CanonKind(pc):
			asks++
		case ActChoose + CanonKind(kc):
			picks++
		}
	}
	t.Logf("%+v; player asks %d, counter picks %d", y, asks, picks)
	if y.windows == 0 || y.draws == 0 || y.counters == 0 {
		t.Errorf("the end step chain did not run through its choices, draw and counters: %+v", y)
	}
	if asks == 0 || picks == 0 {
		t.Errorf("a player ask or a counter pick was never taken: %d, %d", asks, picks)
	}
}

// TestMBCChosenPlayerLowers plays two-seat games of Gluntch, the Bestower: the
// first choice is asked of the controller over both players, the second has
// one player left and is taken in place, and the third choice and the
// Treasures sit behind ConditionSVarCompare$ GE3, so none is made. The chosen
// player draws, the first one puts the counters on a creature of theirs (asked
// when they control two or more), and no Treasure is ever created. The
// four-seat pods are TestFourSeatGluntchLockstep (build tag dirac_mp).
func TestMBCChosenPlayerLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	y, acts := playGluntch(t, 2, 12)
	checkGluntch(t, y, acts)
	if y.threeWay != 0 || y.treasures != 0 {
		t.Errorf("two seats chose a third player or made Treasures: %+v", y)
	}
}
