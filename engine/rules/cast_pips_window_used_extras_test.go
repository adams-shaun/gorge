// The low-life Phyrexian ceiling must count the no-tap self-sacrifice
// activation a plan's OWN source can still take after that plan tapped it
// (CR 602.2b, 605.1a), and must not price a source the plan already paid
// for a second time.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const pipsWindowSwampSrc = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"

// pipsWindowUsedExtrasBoard is the repro board with one Swamp replaced by the
// real corpus Eye of Ramos: Thundering Tanadon in hand at 1 life, two Swamps,
// one Forest, the unpriced Gemstone Mine with three mining counters recorded
// G, and the Eye. The {4} witness taps Eye ({U}), Forest ({G}) and both
// Swamps ({B}{B}); the window then needs the Eye's SECOND activation (its
// no-tap "Sacrifice Eye of Ramos: Add {U}") plus the Mine's unpriced {G} to
// reach the two Phyrexian colour faces at 1 life. The plan's Eye activation
// does not sacrifice it, so both are really available.
type pipsWindowUsedExtrasBoard struct {
	e                  *Engine
	tanadon, eye, mine state.ObjID
}

func newPipsWindowUsedExtrasBoard(t *testing.T, seed uint64) pipsWindowUsedExtrasBoard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	tanadonCard := mustCorpusCard(t, reg, "Thundering Tanadon")
	eyeCard := mustCorpusCard(t, reg, "Eye of Ramos")
	mineCard := mustCorpusCard(t, reg, "Gemstone Mine")
	swamp := card(t, pipsWindowSwampSrc)
	forest := card(t, pipsWindowForestSrc)
	deck0 := []*cards.Card{tanadonCard, eyeCard, mineCard, swamp, swamp, forest}
	deck0 = append(deck0, mountainDeck(t, 40-len(deck0))...)
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck0, mountainDeck(t, 40)}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	tanadon := searchMoveByNameSeat(t, e, 0, "Thundering Tanadon", state.ZHand)
	mine := searchMoveByNameSeat(t, e, 0, "Gemstone Mine", state.ZBattlefield)
	eye := searchMoveByNameSeat(t, e, 0, "Eye of Ramos", state.ZBattlefield)
	for range 2 {
		searchMoveByNameSeat(t, e, 0, "Swamp", state.ZBattlefield)
	}
	searchMoveByNameSeat(t, e, 0, "Forest", state.ZBattlefield)
	// The Mine's Any-colour ability needs a recorded colour for its base
	// ceiling to resolve, as in newPipsWindowBoard.
	e.G.Obj(mine).ChosenColor = "G"
	e.G.Players[0].Life = 1
	e.pending = nil
	e.priorityRound()
	return pipsWindowUsedExtrasBoard{e: e, tanadon: tanadon, eye: eye, mine: mine}
}

// TestCastPipsWindowCountsUsedSourceExtras: the Eye's second {U} is the sixth
// mana. A ceiling that drops every source the plan tapped sees only the plan's
// four plus the Mine's one and withholds a payable cast.
func TestCastPipsWindowCountsUsedSourceExtras(t *testing.T) {
	t.Parallel()
	b := newPipsWindowUsedExtrasBoard(t, 9781)
	e := b.e
	// Preconditions: each object is where the rules read it.
	if got := e.G.Players[0].Life; got != 1 {
		t.Fatalf("precondition: payer life = %d, want 1", got)
	}
	if o := e.G.Obj(b.tanadon); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Thundering Tanadon zone = %v, want hand", zoneOf(o))
	}
	for _, id := range []state.ObjID{b.eye, b.mine} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: source %d = %+v, want an untapped battlefield permanent", id, o)
		}
	}
	o := e.G.Obj(b.mine)
	if len(o.Counters) != 1 || o.Counters[0].Kind != "MINING" || o.Counters[0].N != 3 {
		t.Fatalf("precondition: Gemstone Mine counters = %v, want 3 MINING", o.Counters)
	}
	// The Eye must really carry the two activations the test depends on: a
	// free tap and a no-tap self-sacrifice beside it.
	probe := e.castWindowProbeAbilities(0, b.eye)
	if got := len(probe); got != 2 {
		t.Fatalf("precondition: Eye of Ramos probe abilities = %d, want 2", got)
	}
	selfSac := 0
	for _, ma := range probe {
		if castWindowNoTapSelfSac(e, 0, b.eye, ma) {
			selfSac++
		}
	}
	if selfSac != 1 {
		t.Fatalf("precondition: Eye no-tap self-sacrifice abilities = %d, want 1", selfSac)
	}

	got := faceCastPlan(t, e.PotentialPaymentPlans(0), b.tanadon, "")
	if got.Reason != "" || got.Plan == nil {
		t.Fatalf("Thundering Tanadon withheld (%+v/%q): the plan's four plus the Eye's second {U} and the Mine's {G} pay six at 1 life", got, got.Detail)
	}
	// The fix only matters if the offered witness is the one that taps the
	// Eye without sacrificing it: that is the activation its Extra follows.
	usedEye := false
	for _, a := range got.Plan.Activations {
		if a.Source != b.eye {
			continue
		}
		usedEye = true
		if a.Consequence != nil && a.Consequence.Sacrifice {
			t.Fatalf("plan sacrifices the Eye (%+v); its second activation is not available", a)
		}
	}
	if !usedEye {
		t.Fatalf("plan does not tap the Eye: %+v", got.Plan)
	}
}

// TestCastWindowUnpricedReachSkipsUsedSource pins the ceiling's used-source
// accounting: a source the plan already activated is in the plan's `produced`
// sum, so the unpriced term must not add its whole production again. Only a
// surviving no-tap second activation (none on the Mine) may be added.
func TestCastWindowUnpricedReachSkipsUsedSource(t *testing.T) {
	t.Parallel()
	b := newPipsWindowUsedExtrasBoard(t, 9782)
	pc := &pendingCast{player: 0, cost: ParseCost("4 GP GP")}
	units := b.e.castWindowUnits(pc)
	for _, u := range units {
		if u.ID == b.mine {
			t.Fatalf("precondition: the unpriced Mine is a priced unit")
		}
	}
	// Precondition: while the plan has not used it, the Mine's chosen G is the
	// ceiling's one unpriced unit.
	if reach, bounded := castWindowUnpricedReach(b.e, pc, units, nil, nil); !bounded || reach != 1 {
		t.Fatalf("precondition: unused Mine ceiling = (%d, %v), want (1, true)", reach, bounded)
	}
	used := map[state.ObjID]bool{b.mine: true}
	if reach, bounded := castWindowUnpricedReach(b.e, pc, units, used, nil); !bounded || reach != 0 {
		t.Fatalf("used Mine ceiling = (%d, %v), want (0, true): its production is already in the plan's sum", reach, bounded)
	}
}
