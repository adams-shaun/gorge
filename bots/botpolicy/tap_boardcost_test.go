package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The board-consuming activation decline. A mana ability whose cost
// consumes a battlefield permanent (Food Chain's "Exile a creature you
// control: Add X mana", Phyrexian Altar's "Sacrifice a creature: Add one
// mana of any colour") is not a tap, and the tap gate must never take it:
// unlike a tap (bounded by the untapped set) or a converter (bounded by the
// pool), its repetitions are bounded only by the fodder, and the seat's own
// casts re-supply that fodder -- the measured seed-129300 four-seat
// commander loop, where Food Chain exiled the Eternal Scourge the bot had
// just cast, one 35-event cycle per iteration, 400+ events inside one main
// phase until the engine's livelock watcher fired.

// loopBoard is the seed-129300 shape: an intended Herald-of-Kozilek-shaped
// castable card ({1}{U}{R}, CMC 4) the pool of 3 cannot pay, and a
// Food-Chain-shaped battlefield source whose production is a colour choice
// (Any) the gate cannot steer or price.
func loopBoard() (Board, *decision.Decision) {
	var anyProd cards.ManaProduction
	anyProd.Any = true
	anyProd.Colour[5] = 1 // the conditional/Any source claims no guaranteed slot
	var uProd cards.ManaProduction
	uProd.Colour[state.MU] = 1

	pool := state.Mana{}
	pool[state.MW] = 2
	restricted := pool
	restricted[state.MW] = 1

	b := Board{IsMain: true, MyTurn: true, Pool: pool, PoolRestricted: restricted,
		Cards: TableOf(map[state.ObjID]Card{
			1: {Castable: true, CMC: 4, ManaCost: "1 U R"},
			2: {OnBattlefield: true, Produces: anyProd},
			3: {OnBattlefield: true, Produces: uProd},
		})}
	// Precondition: the intended spell really wants a tap (the pool of 3
	// covers neither its 4 mana nor both pips), the Food-Chain source's
	// production does satisfy the satisfiability filter (its Any claims the
	// U and R pips), and the restricted unit is genuinely restricted.
	if !b.tapWants(nil) {
		panic("precondition failed: the pool already pays the intended card")
	}
	if pips := colourPips("1 U R"); pips[state.MU] != 1 || pips[state.MR] != 1 {
		panic("precondition failed: the intended card carries no U/R pips")
	}
	if !b.offeredClosesPips(b.Cards.Get(1), b.offeredColours(&decision.Decision{
		Options: []decision.Option{{Kind: "activate", Obj: 2}, {Kind: "activate", Obj: 3}},
	})) {
		panic("precondition failed: the offered sources cannot close the pips")
	}
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "activate", Obj: 2, Label: "Activate Food Chain for mana", Cost: "Exile<1/Creature>"},
			{Index: 1, Kind: "pass"},
		}}
	return b, d
}

// TestChooseTapDeclinesABattlefieldConverter pins the decline on the
// measured shape: the only offered activation exiles a creature you
// control, so the gate falls through to the pass even though the
// satisfiability filter says its production could close the intended
// card's pips.
func TestChooseTapDeclinesABattlefieldConverter(t *testing.T) {
	b, d := loopBoard()
	if got := b.chooseTap(d); got != -1 {
		t.Fatalf("chooseTap = %d, want -1: a mana ability whose cost exiles a creature you control is never a tap", got)
	}
}

// TestChooseTapDeclinesASacrificeConverter is the same decline over the
// Sac<X/Spec> head (Phyrexian Altar's "Sacrifice a creature").
func TestChooseTapDeclinesASacrificeConverter(t *testing.T) {
	b, d := loopBoard()
	d.Options[0].Cost = "Sac<1/Creature>"
	if got := b.chooseTap(d); got != -1 {
		t.Fatalf("chooseTap = %d, want -1: a mana ability whose cost sacrifices a creature you control is never a tap", got)
	}
}

// TestChooseTapStillTakesAPlainTapBesideABattlefieldConverter pins the
// decline's boundary: it removes only the board-consuming option from the
// ranking, not the gate's tap. A plain {T} source that demonstrably
// produces a needed colour is still taken.
func TestChooseTapStillTakesAPlainTapBesideABattlefieldConverter(t *testing.T) {
	b, d := loopBoard()
	d.Options = []decision.Option{
		{Index: 0, Kind: "activate", Obj: 2, Label: "Activate Food Chain for mana", Cost: "Exile<1/Creature>"},
		{Index: 1, Kind: "activate", Obj: 3, Label: "Activate Volcanic Island for mana"},
		{Index: 2, Kind: "pass"},
	}
	if got := b.chooseTap(d); got != 1 {
		t.Fatalf("chooseTap = %d, want the plain U tap (option 1), not the board-consuming source", got)
	}
}
