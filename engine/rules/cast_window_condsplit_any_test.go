package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Condition-split mana ability whose root gate provably fails makes its
// sub's mana instead (mana_condsplit.go). A lucky Gemstone Caverns (a luck
// counter on it) adds one mana of ANY colour, so the shared census's
// condition-split arm drops its unit, and the cast window's probe used to
// skip the root on the belief that the census had counted its plain {C} --
// the CR 601.2b X announcement of Walking Ballista ({X}{X}) then priced the
// CR 601.2g window at seven mana and offered X to 3 where eight mana-capable
// sources make X = 4 payable (CR 601.2g, 605.3b; the kernel's lucky Gemstone
// asks one colour of WUBRG, dirac/mana_condsplit.go). The probe now reads the
// live sub, so the window prices the Caverns like any other source.
func TestCastWindowPricesLuckyGemstoneAnyColourSub(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ballista := mustCorpusCard(t, reg, "Walking Ballista")
	board := []string{"Gemstone Caverns", "Mountain", "Mountain", "Mountain",
		"Swamp", "Swamp", "Academy Ruins", "Eiganjo, Seat of the Empire",
		"Shanid, Sleepers' Scourge"}
	var boardCards []*cards.Card
	for _, n := range board {
		boardCards = append(boardCards, mustCorpusCard(t, reg, n))
	}
	e := counterHands(t, []*cards.Card{ballista}, nil, boardCards, nil)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Face().Name == "Gemstone Caverns" {
			o := e.G.Obj(id)
			o.Counters = append(o.Counters, state.Counter{Kind: "LUCK", N: 1})
			e.staticEpoch = -1
			e.activeEpoch = -1
		}
	}
	// Precondition: the census prices all eight mana-capable lands once the
	// probe reads the lucky Caverns' any-colour sub (Shanid makes no mana).
	if n := len(e.windowManaUnits(0)); n != 7 {
		t.Fatalf("precondition: the shared census prices %d sources, want 7 (the Caverns' any-colour sub is not its domain)", n)
	}
	pc := &pendingCast{player: 0, card: handObj(t, e, 0, "Walking Ballista")}
	if n := len(e.castWindowUnits(pc)); n != 8 {
		t.Fatalf("castWindowUnits priced %d sources, want 8 (the eight lands)", n)
	}
	e.askPriority(0)
	submitChoices(t, e, passToCast(t, e, pc.card))
	for range 12 {
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision")
		}
		if d.Kind == decision.KChoose && d.Prompt == "Choose a value for X" {
			if m := maxXOf(d.Options); m != 4 {
				t.Fatalf("largest X offered = %d, want 4 (eight mana-capable sources make {X}{X} at 8 payable)", m)
			}
			return
		}
		submitChoices(t, e, 0)
	}
	t.Fatal("never reached an X ask")
}
