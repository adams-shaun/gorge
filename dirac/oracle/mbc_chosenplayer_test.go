package oracle

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC chosen-player-card-1 family: the as-enters chosen opponent of The
// Rack and Black Vise, and of Entropic Specter and Nyxathid whose statics
// read it (kgen lower_chooseplayerenter.go, kernel repl_chooseplayer.go),
// played from the chosenopp fixture pool.

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
	for _, name := range append([]string{"Island", "Plains", "Swamp"}, chosenOppCards...) {
		if i, ok := set.Lookup(name); !ok || i >= len(sup) || !sup[i] {
			t.Fatalf("the chosenopp pool does not play %s: %s", name, set.Reason(i))
		}
	}
	return set
}

// chosenOppTally counts, on gorge's side, the upkeep triggers of The Rack
// and Black Vise, the player damage that followed one before the next
// trigger, and the entries of each card of the pool that chooses an opponent.
type chosenOppTally struct {
	trig, hits int
	entered    map[string]int
}

var chosenOppCards = []string{"The Rack", "Black Vise", "Entropic Specter", "Nyxathid"}

func (y *chosenOppTally) read(g *GorgeSide) {
	if y.entered == nil {
		y.entered = map[string]int{}
	}
	evs := g.E.L.Events
	for i := range evs {
		name := firstFaceName(g.E.G, evs[i].Obj)
		if evs[i].Kind == events.MoveZone && evs[i].To == state.ZBattlefield && slices.Contains(chosenOppCards, name) {
			y.entered[name]++
		}
		if evs[i].Kind != events.TriggerPush || name != "The Rack" && name != "Black Vise" {
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

// chosenOppDecks are the matchups: the upkeep artifacts, then the creatures
// whose statics read the chosen hand, each against a deck of lands only (a
// hand that never empties). seats is 2 or 4; in four seats the two decks
// alternate.
func chosenOppDecks(seats int) [][]string {
	vise := deckOf(10, "Black Vise", deckOf(10, "The Rack", deckOf(20, "Island")...)...)
	black := deckOf(8, "Entropic Specter", deckOf(8, "Nyxathid", deckOf(24, "Swamp")...)...)
	lands := deckOf(40, "Plains")
	var out [][]string
	for _, d := range [][]string{vise, black} {
		for i := 0; i < seats; i++ {
			if i%2 == 0 {
				out = append(out, d)
			} else {
				out = append(out, lands)
			}
		}
	}
	return out
}

// playChosenOpp plays every seats-sized matchup of chosenOppDecks over seeds
// 1..seeds in lockstep, failing on a divergence or a kernel stop, and returns
// gorge's tally and the canonical actions taken.
func playChosenOpp(t *testing.T, seats int, seeds uint64) (chosenOppTally, []Canon) {
	t.Helper()
	reg := synthReg(t)
	set := chosenOppPool(t)
	all := chosenOppDecks(seats)
	var y chosenOppTally
	var acts []Canon
	for m0 := 0; m0 < len(all); m0 += seats {
		decks := all[m0 : m0+seats]
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
		for seed := uint64(1); seed <= seeds; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]}, Seed: seed, PassElision: true, Surface: "full"}
			for i := 2; i < seats; i++ {
				r.MoreDecks = append(r.MoreDecks, "x"+string(rune('0'+i)))
				r.MoreCards = append(r.MoreCards, decks[i])
			}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions, rc.MaxTurns = 3000, 30
			rc.Finish = y.read
			k, err := SetSide(set, tbl, rc.Config)(m, seed)
			if err != nil {
				t.Fatalf("matchup %d seed %d: %v", m0/seats, seed, err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("matchup %d seed %d: divergence at %d: %s %s %v", m0/seats, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("matchup %d seed %d: kernel stopped: %s", m0/seats, seed, res.Stall)
			}
			acts = append(acts, res.Actions...)
			t.Logf("matchup %d seed %d: %d decisions, over %v %s", m0/seats, seed, res.Decisions, res.Over, res.Stall)
		}
	}
	return y, acts
}

// checkChosenOpp fails unless the upkeep triggers fired and hit and every
// card of the pool that chooses an opponent entered the battlefield.
func checkChosenOpp(t *testing.T, y chosenOppTally) {
	t.Helper()
	if y.trig == 0 || y.hits == 0 {
		t.Errorf("the chosen player's upkeep triggers did not fire and hit: %+v", y)
	}
	for _, n := range chosenOppCards {
		if y.entered[n] == 0 {
			t.Errorf("%s never entered the battlefield", n)
		}
	}
	t.Logf("%+v", y)
}

// TestMBCChosenPlayerCard1Lowers plays two-seat games of the cards that choose
// an opponent as they enter, against a hand that never empties. The choice is
// asked of nobody (gorge's single-option ChoosePlayer is answered in place);
// The Rack and Black Vise trigger on that player's upkeep only (ValidPlayer$
// Player.Chosen) and deal damage from that player's hand size
// (Count$ValidHand Card.ChosenCtrl), and Entropic Specter's and Nyxathid's
// statics read the same hand. No divergence, the triggers fired and hit, and
// every card entered.
func TestMBCChosenPlayerCard1Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	y, _ := playChosenOpp(t, 2, 6)
	checkChosenOpp(t, y)
}

// TestChoosePlayerOrderPairRefused: a card that chooses an opponent as it
// enters beside Orb of Dreams (permanents enter tapped, an enters replacement
// over other objects) is refused as a pair before its game, since gorge asks
// the CR 616.1 order of the two Updated replacements and the kernel does not
// pose it (StallChoosePlayerOrder). Either half plays without the other.
func TestChoosePlayerOrderPairRefused(t *testing.T) {
	set := chosenOppPool(t)
	entries := func(d []string) []cardset.Entry {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	vise := entries(deckOf(10, "Black Vise", deckOf(30, "Island")...))
	orb := entries(deckOf(10, "Orb of Dreams", deckOf(30, "Plains")...))
	lands := entries(deckOf(40, "Plains"))
	for _, ok := range [][2][]cardset.Entry{{vise, lands}, {orb, lands}} {
		if _, _, err := set.MatchupN([][]cardset.Entry{ok[0], ok[1]}); err != nil {
			t.Fatalf("precondition: one half alone must build: %v", err)
		}
	}
	_, _, err := set.MatchupN([][]cardset.Entry{vise, orb})
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) || ir.Reason != "chooseplayerorder" {
		t.Fatalf("Black Vise beside Orb of Dreams: got %v, want the chooseplayerorder interaction refusal", err)
	}
	if !slices.Contains(ir.Cards, "Black Vise") || !slices.Contains(ir.Cards, "Orb of Dreams") {
		t.Fatalf("refusal names %v, want both halves", ir.Cards)
	}
}
