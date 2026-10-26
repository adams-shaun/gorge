package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cr-g133: a twobrid ({2/X}) pip is payable with one coloured mana (CR
// 107.4e), so the cheapest payment of a twobrid hand spell counts each pip
// as 1, not 2 (CR 202.3f makes the mana value 2 per pip, but the mana value
// is not a payment floor). The quiet proof's castFloor must therefore be the
// cheapest mana payment, not the face's mana value: with the mana-value
// floor the proof called a payable Reigning Victor ({2/R}{2/W}{2/B}, mana
// value 6) quiet and skipped its priority window (or panicked in verify
// mode), while the walk offered the cast.

// twobridHandFixture seats Reigning Victor in seat 0's hand, spends seat 0's
// land drop, and funds the pool, then asserts the board the tests below read.
func twobridHandFixture(t *testing.T, mana string) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	victor := lookup(t, reg, "Reigning Victor")
	e := quietBaseWith(t, reg, []*cards.Card{victor})
	id := addHand(t, e, 0, victor)
	e.G.Players[0].LandsPlayed = 1
	for _, r := range mana {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
	}
	// Preconditions every assertion below depends on.
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Reigning Victor %d is not in seat 0's hand: %+v", id, o)
	}
	pool := map[byte]int32{'R': 0, 'W': 0, 'B': 0}
	for _, r := range []byte(mana) {
		pool[r]++
	}
	for r, want := range pool {
		if want == 0 {
			continue
		}
		slot := state.ManaIndex(r)
		if e.G.Players[0].Pool[slot] != want {
			t.Fatalf("precondition: pool %c = %d, want %d", r, e.G.Players[0].Pool[slot], want)
		}
	}
	if e.G.Players[0].LandsPlayed != 1 {
		t.Fatalf("precondition: LandsPlayed = %d, want 1 (the drop is spent)", e.G.Players[0].LandsPlayed)
	}
	return e, id
}

// TestQuietProofTwobridCastFloor is the probe board: R, W and B in the pool
// pay Reigning Victor's {2/R}{2/W}{2/B} three-pip cheapest payment, so the
// proof must block on the hand spell and the priority round must offer the
// cast (with the mana-value floor of 6 the proof said quiet and the window
// was skipped -- or, in verify mode, panicked).
func TestQuietProofTwobridCastFloor(t *testing.T) {
	e, _ := twobridHandFixture(t, "RBW")
	if b := e.quietBlocker(0); b != qbHandSpell {
		t.Fatalf("quietBlocker(0) = %v, want qbHandSpell (R+W+B pays the three twobrid pips)", b)
	}
	e.priorityRound()
	if castByName(t, e, 0, "Reigning Victor") == nil {
		t.Fatalf("the priority round did not offer the Reigning Victor cast: %+v", e.Pending())
	}
}

// TestQuietProofTwobridStillQuietBelowFloor keeps Reigning Victor in hand
// with only R+W in the pool and no untapped land: the cheapest payment needs
// three coloured pips, so the floor still proves quiet and the round must
// not panic under verify mode.
func TestQuietProofTwobridStillQuietBelowFloor(t *testing.T) {
	e, id := twobridHandFixture(t, "RW")
	// Tap every untapped land: no source is open, so the mana ceiling stays
	// 2 and the three-pip floor is unaffordable.
	for _, lid := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(lid); o != nil && o.Face() != nil && o.Face().IsLand() && !o.Tapped {
			e.emit(events.Event{Kind: events.Tap, Obj: lid})
		}
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Reigning Victor %d is not in seat 0's hand: %+v", id, o)
	}
	if b := e.quietBlocker(0); b != qbNone {
		t.Fatalf("quietBlocker(0) = %v, want qbNone (two mana cannot pay three pips)", b)
	}
	e.priorityRound()
	if castByName(t, e, 0, "Reigning Victor") != nil {
		t.Fatalf("the round offered a cast two mana cannot pay: %+v", e.Pending())
	}
}

// TestQuietFaceFactsTwobridFloor checks the floors themselves: a twobrid
// face prices its cast at the cheapest payment (pips counted once, CR
// 107.4e), while faces without a twobrid pip keep castFloor == Cmc().
func TestQuietFaceFactsTwobridFloor(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	twobrid := []struct {
		name  string
		floor int32
	}{
		{"Reigning Victor", 3},       // {2/R}{2/W}{2/B}, mana value 6
		{"Karn, Gilded Guardian", 5}, // {2/W}{2/U}{2/B}{2/R}{2/G}, mana value 10
		{"Wildgrowth Archaic", 2},    // {2/G}{2/G}, mana value 4
	}
	deck := make([]*cards.Card, 0, len(twobrid)+2)
	for _, c := range twobrid {
		deck = append(deck, lookup(t, reg, c.name))
	}
	deck = append(deck, lookup(t, reg, "Lightning Helix"), lookup(t, reg, "Grizzly Bears"))
	e := New(Config{Names: []string{"x"}, Decks: [][]*cards.Card{deck}})
	for _, c := range twobrid {
		f := lookup(t, reg, c.name).Faces[0]
		ff := e.walkFaceFactsOf(f)
		if ff == nil {
			t.Fatalf("precondition: no walk facts for %q", c.name)
		}
		if got := ff.quiet.castFloor; got != c.floor {
			t.Errorf("%s: castFloor = %d, want %d (each twobrid pip pays as 1)", c.name, got, c.floor)
		}
		if ff.quiet.castOpen {
			t.Errorf("%s: castOpen = true, want false", c.name)
		}
	}
	// Controls with no twobrid pip: the floor is the mana value.
	for _, name := range []string{"Lightning Helix", "Grizzly Bears"} {
		f := lookup(t, reg, name).Faces[0]
		ff := e.walkFaceFactsOf(f)
		if ff == nil {
			t.Fatalf("precondition: no walk facts for %q", name)
		}
		if got := ff.quiet.castFloor; got != f.Cmc() {
			t.Errorf("%s: castFloor = %d, want Cmc %d", name, got, f.Cmc())
		}
	}
}
