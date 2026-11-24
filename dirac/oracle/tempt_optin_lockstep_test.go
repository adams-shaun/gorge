package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The tempting-offer sorceries in lockstep (Tempt with Bunnies and Tempt with
// Vengeance, the tempt fixture pool): each opponent is offered its own yes /
// no (RepeatOptionalForEachPlayer$, ChGenOptInSeat, canonical yes_seat /
// no_seat), and the controller takes the amount the accepted offers counted
// (StoreSVar's CountSVar accumulator, reset when the chain ends). The decks
// are the two spells and their lands on every seat, played at random, so
// every seat is offered and offers in turn.

// temptPool is the tempt fixture pool; the test fails when it does not play
// every card it holds.
func temptPool(t *testing.T) cardset.Set {
	t.Helper()
	corpusDirOrSkip(t)
	set, err := cardset.RawByName("tempt")
	if err != nil {
		t.Fatal(err)
	}
	sup := set.Supported()
	for _, name := range []string{"Mountain", "Plains", "Tempt with Bunnies", "Tempt with Vengeance"} {
		if i, ok := set.Lookup(name); !ok || i >= len(sup) || !sup[i] {
			t.Fatalf("the tempt pool does not play %s: %s", name, set.Reason(i))
		}
	}
	return set
}

// temptDeck is one seat's deck: the spells and their lands.
func temptDeck() []string {
	return deckOf(9, "Tempt with Bunnies", deckOf(9, "Tempt with Vengeance", deckOf(11, "Plains", deckOf(11, "Mountain")...)...)...)
}

// temptTally is the offers seen: the answers of each run of yes_seat /
// no_seat actions (one run per cast: every living opponent in seat order),
// by the run's length and how many accepted.
type temptTally struct {
	runs    map[[2]int]int // [offers, accepted] -> casts
	yes, no int
}

// read adds a game's canonical actions to the tally. A cast's offers are
// consecutive, so a run ends at any other action (an eliminated opponent is
// not asked, which makes a shorter run) or after seats-1 offers.
func (y *temptTally) read(acts []Canon, seats int) {
	if y.runs == nil {
		y.runs = map[[2]int]int{}
	}
	yes, _ := chooseCode("yes_seat")
	no, _ := chooseCode("no_seat")
	n, acc := 0, 0
	flush := func() {
		if n > 0 {
			y.runs[[2]int{n, acc}]++
		}
		n, acc = 0, 0
	}
	for _, a := range acts {
		switch a.Kind() {
		case ActChoose + CanonKind(yes):
			n, acc = n+1, acc+1
			y.yes++
		case ActChoose + CanonKind(no):
			n++
			y.no++
		default:
			flush()
			continue
		}
		if n == seats-1 {
			flush()
		}
	}
	flush()
}

// playTempt plays the tempt pool's matchup of seats seats over seeds 1..seeds
// in lockstep, failing on a divergence or a kernel stop. With report false the
// first divergence is returned instead of failing (the mutation test).
func playTempt(t *testing.T, seats int, seeds uint64, report bool) (*temptTally, int) {
	t.Helper()
	y, div, _ := playTemptStats(t, seats, seeds, report)
	return y, div
}

// playTemptStats is playTempt that also returns the number store's counters
// over the games.
func playTemptStats(t *testing.T, seats int, seeds uint64, report bool) (*temptTally, int, dirac.NumStoreStats) {
	t.Helper()
	var st dirac.NumStoreStats
	dirac.DebugNumStoreStats = &st
	defer func() { dirac.DebugNumStoreStats = nil }()
	reg := synthReg(t)
	set := temptPool(t)
	var lists [][]cardset.Entry
	var decks [][]string
	for i := 0; i < seats; i++ {
		d := temptDeck()
		decks = append(decks, d)
		var es []cardset.Entry
		for _, name := range []string{"Mountain", "Plains", "Tempt with Bunnies", "Tempt with Vengeance"} {
			c := 0
			for _, x := range d {
				if x == name {
					c++
				}
			}
			es = append(es, cardset.Entry{Name: name, Count: c})
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	y := &temptTally{}
	div := 0
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
		rc.MaxDecisions, rc.MaxTurns = 4000, 40
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				return y, div, st
			}
			t.Fatalf("seed %d: divergence at %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
		// the tokens the offers make outgrow a combat declaration's options
		// (StallOptions) in a long game: the game ends there, every decision
		// before it matched
		if stop := strings.HasPrefix(res.Stall, KernelStallPrefix); res.StallUnmodelled || stop && res.Stall != KernelStallPrefix+"options" {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		y.read(res.Actions, seats)
	}
	return y, div, st
}

// checkTempt fails unless the pool made offers and both answers were given.
func checkTempt(t *testing.T, y *temptTally, seats int) {
	t.Helper()
	if y.yes == 0 || y.no == 0 {
		t.Fatalf("offers answered yes %d, no %d: the pool did not reach both", y.yes, y.no)
	}
	t.Logf("%d seats: yes %d, no %d, casts by [offers accepted]: %v", seats, y.yes, y.no, y.runs)
}

// TestTemptOptInLockstep2: one opponent offered, either answer.
func TestTemptOptInLockstep2(t *testing.T) {
	y, _, st := playTemptStats(t, 2, 24, true)
	checkTempt(t, y, 2)
	if y.runs[[2]int{1, 0}] == 0 || y.runs[[2]int{1, 1}] == 0 {
		t.Fatalf("casts by [offers accepted] %v: want both a declined and an accepted offer", y.runs)
	}
	// every accepted offer counted once, and every cast that counted reset the
	// accumulator before it ended: the next cast starts at 0, and no number is
	// left on a card (a leak would show as fewer deletions than writes)
	if st.Writes != y.yes || st.Deletes != y.yes {
		t.Fatalf("%d accepted offers, but %d writes and %d resets that deleted a record (%+v)", y.yes, st.Writes, st.Deletes, st)
	}
}

// TestTemptOptInLockstep4: three opponents offered in seat order; none, some
// and all accept over the seeds, declines recorded.
func TestTemptOptInLockstep4(t *testing.T) {
	y, _, st := playTemptStats(t, 4, 24, true)
	checkTempt(t, y, 4)
	if st.Writes != y.yes || st.Deletes == 0 || st.Deletes > st.Writes {
		t.Fatalf("%d accepted offers, %d writes, %d resets that deleted a record (%+v)", y.yes, st.Writes, st.Deletes, st)
	}
	some := y.runs[[2]int{3, 1}] + y.runs[[2]int{3, 2}]
	if y.runs[[2]int{3, 0}] == 0 || some == 0 || y.runs[[2]int{3, 3}] == 0 {
		t.Fatalf("casts by [offers accepted] %v: want none, some and all of three accepting", y.runs)
	}
}

// TestTemptOptInNoAccumMutation: with the accumulator's write dropped inside
// the loop, some seed diverges (the controller's draw and tokens read it), so
// the lockstep does observe the count. Test-only global: never parallel.
func TestTemptOptInNoAccumMutation(t *testing.T) {
	defer func() { dirac.DebugOptInNoAccum = false }()
	dirac.DebugOptInNoAccum = true
	if _, div := playTempt(t, 2, 24, false); div == 0 {
		t.Fatal("no seed diverged with the accumulator dropped: the pool does not read the count")
	}
}
