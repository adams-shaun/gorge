package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Event-ended Effect statics (kop.Override.UntilCtrlUntap / UntilCast):
// Orcish Farmer's "target land becomes a Swamp until its controller's next
// untap step" and Soul Sculptor's "target creature becomes an enchantment and
// loses all abilities until a player casts a creature spell" (the script reads
// ValidActivatingPlayer$ You). The fixture pool is dirac/cardset/pools/fixtures/
// endtrig; the kernel is dirac/static_ovendevent.go.
const endTrigPool = "endtrig"

var endTrigDeck = []cardset.Entry{
	{Name: "Orcish Farmer", Count: 5}, {Name: "Soul Sculptor", Count: 5}, {Name: "Prodigal Sorcerer", Count: 2},
	{Name: "Serra Angel", Count: 1}, {Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 3},
	{Name: "Hill Giant", Count: 2}, {Name: "Elite Vanguard", Count: 2},
	{Name: "Mountain", Count: 5}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 2},
	{Name: "Swamp", Count: 1},
}

// endTrigDrops runs fn and returns the drops it made of each kind.
func endTrigDrops(fn func()) (untap, cast int64) {
	u0, c0 := dirac.EventOvDropStats.CtrlUntap.Load(), dirac.EventOvDropStats.Cast.Load()
	fn()
	return dirac.EventOvDropStats.CtrlUntap.Load() - u0, dirac.EventOvDropStats.Cast.Load() - c0
}

// TestEffectEndingTriggerLockstep: 64 two-seat games in lockstep with gorge,
// and both drops must have happened (the pool reaches the ends).
func TestEffectEndingTriggerLockstep(t *testing.T) {
	decks := [2][]cardset.Entry{endTrigDeck, endTrigDeck}
	reg, set, gd := lockstepPool(t, endTrigPool, decks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatal("the endtrig deck is not playable in the endtrig pool")
	}
	var divs []lockstepDivergence
	var clean int
	untap, cast := endTrigDrops(func() {
		clean, _, divs = lockstepSeedsMode(t, reg, set, endTrigPool, decks, gd, 1, 64, lockstepFailEach, nil, nil, castFirstPolicy)
	})
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if untap == 0 || cast == 0 {
		t.Fatalf("drops: ctrl-untap %d, cast %d, want both above zero: the pool does not reach an end", untap, cast)
	}
	t.Logf("%d clean games; %d untap-step drops, %d cast drops", clean, untap, cast)
}

// TestEffectEndingTriggerMutation: with the drops switched off some game
// diverges from gorge.
func TestEffectEndingTriggerMutation(t *testing.T) {
	decks := [2][]cardset.Entry{endTrigDeck, endTrigDeck}
	reg, set, gd := lockstepPool(t, endTrigPool, decks)
	old := dirac.DebugNoEventOvDrop
	dirac.DebugNoEventOvDrop = true
	defer func() { dirac.DebugNoEventOvDrop = old }()
	_, _, divs := lockstepSeedsMode(t, reg, set, endTrigPool, decks, gd, 1, 64, lockstepQuiet, nil, nil, castFirstPolicy)
	if len(divs) == 0 {
		t.Fatal("no game diverged with the event-ended records never dropped: the pool does not reach them")
	}
	t.Logf("%d games diverged without the drops", len(divs))
}

// TestFourSeatEffectEndingTriggerLockstep: four seats, where "its controller's
// untap step" is not always the activator's and a cast by another seat does not
// end a Soul Sculptor Effect.
func TestFourSeatEffectEndingTriggerLockstep(t *testing.T) {
	var d []string
	for _, e := range endTrigDeck {
		d = append(d, deckOf(e.Count, e.Name)...)
	}
	untap, cast := endTrigDrops(func() {
		if n := pod4FixtureLockstep(t, endTrigPool, [4][]string{d, d, d, d}, 24, true, nil); n != 0 {
			t.Fatalf("%d diverged", n)
		}
	})
	t.Logf("%d untap-step drops, %d cast drops", untap, cast)
	if untap == 0 && cast == 0 {
		t.Fatal("no four-seat game reached either drop")
	}
}
