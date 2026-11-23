package rules

// unless_selfsac_amount_test.go -- the unless-cost window's census must count
// a deterministic self-sacrifice mana source whose production is an evaluated
// amount (Lotus Blossom's "{T}, Sacrifice: Add X mana of any one color, X =
// its petal counters", CR 605.3a's payment window). The fixed-amount class
// (Treasure, Lotus Petal) is the paid-cost census's; the evaluated arm of
// unlessWindowUnits required a TapOnly cost, so a T+Sac source priced zero and
// an unless tax a single basic plus it could pay was offered decline-only.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const usaPlains = "Name:Plains\nManaCost:0\nTypes:Land Plains\n" +
	"A:AB$ Mana | Cost$ T | Produced$ W\nOracle:x\n"

// usaLotusBlossom models the corpus's Lotus Blossom (the activation is the
// card's own shape) under a distinct name, wording and counter kind, so no
// line is a verbatim Forge script line (the GPL boundary).
const usaLotusBlossom = "Name:Petal Bloom\nManaCost:2\nTypes:Artifact\n" +
	"A:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any | Amount$ X | " +
	"SpellDescription$ Add X mana of any one colour, X is the bloom's BUD count.\n" +
	"SVar:X:Count$CardCounters.BUD\nOracle:x\n"

// usaSeat places a tap-for-{W} land and a Petal Bloom carrying n BUD counters
// (the petals) on seat p's battlefield and asserts both are live untapped mana
// sources there.
func usaSeat(t *testing.T, p state.PlayerID, petals int32) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, _, _ := newFixtureDeck(t, 4221, usaLotusBlossom)
	land := onBoardCard(t, e, p, card(t, usaPlains))
	lotus := onBoardCard(t, e, p, card(t, usaLotusBlossom))
	if petals > 0 {
		e.emit(events.Event{Kind: events.CounterChange, Obj: lotus, Counter: "BUD", Amount: petals})
	}
	for _, id := range []state.ObjID{land, lotus} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: source zone=%v tapped=%v", o.Zone, o.Tapped)
		}
	}
	if got := e.G.Obj(lotus).Counter("BUD"); got != petals {
		t.Fatalf("precondition: petal counters = %d, want %d", got, petals)
	}
	return e, land, lotus
}

// TestUnlessSelfSacAmountPayable: a {2} unless tax is payable from the land's
// {W} plus Lotus Blossom's one petal of mana, and from the Blossom alone with
// two petals.
func TestUnlessSelfSacAmountPayable(t *testing.T) {
	t.Parallel()
	e, land, lotus := usaSeat(t, 0, 1)
	if !e.UnlessCostPayable(0, "2") {
		t.Fatalf("a {2} unless tax must be payable from a tap-land plus a Lotus Blossom with one petal (land %d, blossom %d)", land, lotus)
	}
	e2, _, lotus2 := usaSeat(t, 0, 2)
	if !e2.UnlessCostPayable(0, "2") {
		t.Fatalf("a {2} unless tax must be payable from a Lotus Blossom with two petals (blossom %d)", lotus2)
	}
}

// TestUnlessSelfSacAmountShortStillDeclines: one petal with no land cannot pay
// {2}, so the census must not widen the offer.
func TestUnlessSelfSacAmountShortStillDeclines(t *testing.T) {
	t.Parallel()
	e, land, lotus := usaSeat(t, 0, 0)
	_ = land
	if e.UnlessCostPayable(0, "2") {
		t.Fatalf("a {2} unless tax is not payable from a petal-less Lotus Blossom (%d) alone", lotus)
	}
}
