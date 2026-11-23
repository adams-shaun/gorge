// The CR 601.2g window ceiling beside the low-life Phyrexian floor. The
// ceiling may only over-produce: castWindowUnpricedReach must fail open
// (bounded=false) when an unpriced source's own tap trigger or replacement
// would add mana, because castWindowProbeCeiling prices the ability's base
// Amount$ and cannot see the extra. The floor it is compared against counts
// only Phyrexian pips (PlanManaHalf keeps hybrid-Phyrexian pips in the
// witness, so their colour face is never stranded). And the ceiling's
// "have" total must not add the floating pool's provenance tallies -- Snow
// and ManaUnits partition the pool, they are not additional mana.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const pipsWindowForestSrc = "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"

// pipsWindowBoard is the Thundering Tanadon board: the {4}{G/P}{G/P} spell in
// seat 0's hand at 1 life, `forests` untapped Forests (the pip-free {4} half)
// and the real corpus Gemstone Mine with its three mining counters. Gemstone
// Mine's {T}, remove a mining counter ability is a fail-closed census
// exclusion, so it is the unpriced source. withHaven stages the real corpus
// Wolfwillow Haven attached to the Mine; its TapsForMana trigger adds {G}
// whenever the enchanted land is tapped for mana, which is the alteration the
// ceiling must not price.
type pipsWindowBoard struct {
	e                    *Engine
	tanadon, mine, haven state.ObjID
}

func newPipsWindowBoard(t *testing.T, seed uint64, forests int, withHaven bool) pipsWindowBoard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	tanadonCard := mustCorpusCard(t, reg, "Thundering Tanadon")
	mineCard := mustCorpusCard(t, reg, "Gemstone Mine")
	havenCard := mustCorpusCard(t, reg, "Wolfwillow Haven")
	forest := card(t, pipsWindowForestSrc)
	deck0 := []*cards.Card{tanadonCard, mineCard, havenCard}
	for range forests {
		deck0 = append(deck0, forest)
	}
	deck0 = append(deck0, mountainDeck(t, 40-len(deck0))...)
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck0, mountainDeck(t, 40)}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	tanadon := searchMoveByNameSeat(t, e, 0, "Thundering Tanadon", state.ZHand)
	mine := searchMoveByNameSeat(t, e, 0, "Gemstone Mine", state.ZBattlefield)
	for range forests {
		searchMoveByNameSeat(t, e, 0, "Forest", state.ZBattlefield)
	}
	var haven state.ObjID
	if withHaven {
		haven = findByName(e, "Wolfwillow Haven", 0)
		if haven == 0 {
			t.Fatal("Wolfwillow Haven not found in the fixture deck")
		}
		stageAuraEntry(t, e, haven, mine)
	}
	// The Mine's Any-colour ability needs a recorded colour for its base
	// ceiling to resolve; the repro board had one from an earlier activation.
	// Without it the ceiling reads zero and the extra below would not be the
	// only missing mana.
	e.G.Obj(mine).ChosenColor = "G"
	e.G.Players[0].Life = 1
	e.pending = nil
	e.priorityRound()
	return pipsWindowBoard{e: e, tanadon: tanadon, mine: mine, haven: haven}
}

// pipsWindowPreconditions asserts the board is where the rules read it: the
// payer at 1 life (so a Phyrexian pip cannot take its two-life face), the
// spell in hand, the Forests and Gemstone Mine untapped on the battlefield,
// the Mine carrying its three mining counters, and (for the Haven board) the
// Aura attached to the Mine.
func (b pipsWindowBoard) pipsWindowPreconditions(t *testing.T, forests int, withHaven bool) {
	t.Helper()
	e := b.e
	if got := e.G.Players[0].Life; got != 1 {
		t.Fatalf("precondition: payer life = %d, want 1", got)
	}
	if o := e.G.Obj(b.tanadon); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Thundering Tanadon zone = %v, want hand", zoneOf(o))
	}
	untapped := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != "Forest" {
			continue
		}
		if o.Tapped {
			t.Fatalf("precondition: Forest %d is tapped", id)
		}
		untapped++
	}
	if untapped != forests {
		t.Fatalf("precondition: %d untapped Forests, want %d", untapped, forests)
	}
	o := e.G.Obj(b.mine)
	if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Gemstone Mine = %+v, want an untapped battlefield permanent", o)
	}
	if len(o.Counters) != 1 || o.Counters[0].Kind != "MINING" || o.Counters[0].N != 3 {
		t.Fatalf("precondition: Gemstone Mine counters = %v, want 3 MINING", o.Counters)
	}
	if withHaven {
		h := e.G.Obj(b.haven)
		if h == nil || h.AttachedTo != b.mine {
			t.Fatalf("precondition: Wolfwillow Haven not attached to the Mine %d", b.mine)
		}
	}
}

// TestCastWindowUnpricedAlteredProductionFailsOpen pins the boundary the
// ceiling's doc comment promises: an unpriced source whose tap trigger adds
// mana cannot be bounded at the ability's base Amount$. Four Forests fund
// the pip-free {4}; the unpriced Gemstone Mine's base ceiling is one, so the
// window looks like 5 units against the six this {4}{G/P}{G/P} cast needs at
// 1 life -- unless Wolfwillow Haven's attached-land trigger is counted. A
// bounded ceiling would withhold a cast the window can really pay (5 + the
// trigger's {G} = 6); the altered production must fail open and keep the
// offer. The Haven-less control stays withheld: with only five real units the
// cast is unpayable, so the offer above is the fix, not a vacuous one.
func TestCastWindowUnpricedAlteredProductionFailsOpen(t *testing.T) {
	t.Parallel()
	b := newPipsWindowBoard(t, 9771, 4, true)
	b.pipsWindowPreconditions(t, 4, true)
	probe := b.e.castWindowProbeAbilities(0, b.mine)
	if len(probe) == 0 {
		t.Fatal("precondition: Gemstone Mine has no probe-admitted mana ability")
	}
	if !b.e.manaProductionAltered(b.mine, probe[0]) {
		t.Fatal("precondition: Wolfwillow Haven's TapsForMana trigger does not alter the Mine's production")
	}
	if n, ok := castWindowProbeCeiling(b.e, 0, b.mine, probe[0]); !ok || n != 1 {
		t.Fatalf("precondition: Mine's base ceiling = (%d, %v), want (1, true)", n, ok)
	}
	if got := faceCastPlan(t, b.e.PotentialPaymentPlans(0), b.tanadon, ""); got.Reason != "" || got.Plan == nil {
		t.Fatalf("Thundering Tanadon withheld (%+v/%q): with four Forests, the Mine's base one and Wolfwillow Haven's extra {G} the window can pay six", got, got.Detail)
	}

	// Negative control: the same board without the Haven can really make only
	// five, so the cast must stay withheld.
	c := newPipsWindowBoard(t, 9772, 4, false)
	c.pipsWindowPreconditions(t, 4, false)
	if got := faceCastPlan(t, c.e.PotentialPaymentPlans(0), c.tanadon, ""); got.Reason == "" && got.Plan != nil {
		t.Fatalf("Thundering Tanadon kept without the Haven (%+v/%q): the window's five units cannot pay six", got, got.Detail)
	}
}

// TestPhyrexianWindowCeilingDoesNotDoubleCountTypedPool pins the other half
// of `have`: Available.Typed is Player.ManaUnits(), the exclusive provenance
// partition of the floating pool, so adding its Total() on top of Pool counts
// every floating typed unit twice. Three Forests plus one floating typed {G}
// fund the pip-free {4}; the window can really make one (pool) + three
// (Forest production) + one (unpriced Mine base) = five, short of the six at
// 1 life, so the cast must stay withheld. The old double count read six and
// kept the unpayable offer.
func TestPhyrexianWindowCeilingDoesNotDoubleCountTypedPool(t *testing.T) {
	t.Parallel()
	b := newPipsWindowBoard(t, 9773, 3, false)
	b.pipsWindowPreconditions(t, 3, false)
	b.e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "ArtifactG", Amount: 1})
	b.e.pending = nil
	b.e.priorityRound()
	// Precondition: the unit really is floating and really is typed, so the
	// have-sum below would double it without the fix.
	if got := b.e.G.Players[0].Pool.Total(); got != 1 {
		t.Fatalf("precondition: pool total = %d, want the one floating typed unit", got)
	}
	if got := b.e.G.Players[0].ManaUnits()[state.TypedArtifact][state.MG]; got != 1 {
		t.Fatalf("precondition: typed Artifact green units = %d, want 1", got)
	}
	if got := faceCastPlan(t, b.e.PotentialPaymentPlans(0), b.tanadon, ""); got.Reason == "" && got.Plan != nil {
		t.Fatalf("Thundering Tanadon kept (%+v/%q): the window can make only five (one floating, three Forests, one Mine) against six", got, got.Detail)
	}
}

// TestCastPipsManaFloorCountsOnlyPhyrexianPips pins the floor's pip model:
// only the pips pay.PlanManaHalf strips ({G/P}) take their colour face out of
// the window, so only they add a unit the life total cannot pay. A
// hybrid-Phyrexian pip survives PlanManaHalf and is funded by the witness,
// so it must not reach the floor (the guard at castPlanPipsOutcome only
// enters for cost.Phyrexian).
func TestCastPipsManaFloorCountsOnlyPhyrexianPips(t *testing.T) {
	t.Parallel()
	hybrid := ParseCost("2 PRG")
	if len(hybrid.Phyrexian) != 0 || len(hybrid.HybridPhyrexian) != 1 {
		t.Fatalf("precondition: ParseCost(2 PRG) = %+v, want one hybrid-Phyrexian and no Phyrexian pip", hybrid)
	}
	if got := castPipsManaFloor(&hybrid, 1); got != 2 {
		t.Fatalf("hybrid-Phyrexian floor at 1 life = %d, want 2 (generic only; the witness funds the pip)", got)
	}
	phy := ParseCost("2 GP")
	if len(phy.Phyrexian) != 1 {
		t.Fatalf("precondition: ParseCost(2 GP) Phyrexian = %v, want one pip", phy.Phyrexian)
	}
	if got := castPipsManaFloor(&phy, 1); got != 3 {
		t.Fatalf("Phyrexian floor at 1 life = %d, want 3 (generic 2 plus the one pip life cannot pay)", got)
	}
	if got := castPipsManaFloor(&phy, 2); got != 2 {
		t.Fatalf("Phyrexian floor at 2 life = %d, want 2 (two life pays the pip)", got)
	}
}
