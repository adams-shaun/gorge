package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// extraTurnOnceSrc grants an extra turn only while its controller has not yet
// begun one this game, the shape of a "haven't begun an extra turn this game"
// rider.
const extraTurnOnceSrc = `Name:Test Fresh Hour
ManaCost:U
Types:Sorcery
A:SP$ AddTurn | NumTurns$ 1 | ConditionCheckSVar$ Begun | ConditionSVarCompare$ EQ0 | SpellDescription$ Take an extra turn after this one if you have not begun one this game.
SVar:Begun:Count$YourExtraTurns
Oracle:x
`

// TestYourExtraTurnsCountsOnlyTurnsThatBegan pins Count$YourExtraTurns: a
// pending grant is not a begun turn, so a second copy cast before the first
// extra turn starts still grants (CR 500.7); once the extra turn has begun,
// the gate reads 1 and a copy cast during it grants nothing. The count is read
// from the event log, so the game replays identically.
func TestYourExtraTurnsCountsOnlyTurnsThatBegan(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	hour := card(t, extraTurnOnceSrc)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{hour, hour, hour}, nil)
	grants := func() int {
		return countEvents(e, func(ev events.Event) bool {
			return ev.Kind == events.ExtraTurn && ev.Player == 0 && ev.Amount > 0
		})
	}
	cast := func() {
		moveByName(t, e, 0, "Test Fresh Hour", state.ZHand)
		addMana(t, e, 0, "U")
		castNamed(t, e, "Test Fresh Hour")
		passUntilStackEmpty(t, e, 40)
	}
	if got := e.ExtraTurnsBegun(0); got != 0 {
		t.Fatalf("before any extra turn: ExtraTurnsBegun = %d, want 0", got)
	}
	cast()
	cast()
	if got := grants(); got != 2 {
		t.Fatalf("two casts before any extra turn began granted %d turns, want 2", got)
	}
	e.setStep(state.StepCleanup)
	e.advanceStep()
	if got := e.ExtraTurnsBegun(0); got != 1 {
		t.Fatalf("after the first extra turn began: ExtraTurnsBegun = %d, want 1", got)
	}
	driveToStep(t, e, e.G.Turn, 0, state.StepMain1)
	cast()
	if got := grants(); got != 2 {
		t.Fatalf("a cast after an extra turn began granted another (%d grants, want 2)", got)
	}
	replayCheck(t, e, cfg)
}

const whaleFixtureSrc = "Name:Test Breaching Whale\nManaCost:3 U\nTypes:Creature Whale\nPT:4/4\nOracle:x\n"

// TestBountyExtraTurnRiderReadsBegunTurns plays the real H.M.S. Bounty's
// extra-turn rider: with a Whale in play and no extra turn begun, it grants one;
// once the grant has been consumed (the turn began), the same rider grants no
// second, though the Whale is still there.
func TestBountyExtraTurnRiderReadsBegunTurns(t *testing.T) {
	t.Parallel()
	bounty := corpusCardNamed(t, "H.M.S. Bounty, Unlikely Timeship")
	rider := svarSA(t, bounty, "DBExtraTurn")
	e := fixtureEngine(t, 2)
	src := placeCard(t, e, 0, bounty, state.ZBattlefield)
	placeCard(t, e, 0, card(t, whaleFixtureSrc), state.ZBattlefield)
	run := func() int {
		start := len(e.L.Events)
		runProbeSA(t, e, rider, func() *effects.Ctx { return &effects.Ctx{Source: src, Controller: 0} }, nil)
		return countEventKindWhere(eventsSince(e, start), func(ev events.Event) bool {
			return ev.Kind == events.ExtraTurn && ev.Player == 0 && ev.Amount > 0
		})
	}
	if got := run(); got != 1 {
		t.Fatalf("no extra turn begun yet: rider granted %d, want 1", got)
	}
	e.emit(events.Event{Kind: events.ExtraTurn, Player: 0, Amount: -1})
	if got := e.ExtraTurnsBegun(0); got != 1 {
		t.Fatalf("ExtraTurnsBegun after a consumed grant = %d, want 1", got)
	}
	if got := run(); got != 0 {
		t.Fatalf("an extra turn had begun: rider granted %d, want 0", got)
	}
}

func countEventKindWhere(evs []events.Event, f func(events.Event) bool) int {
	n := 0
	for _, ev := range evs {
		if f(ev) {
			n++
		}
	}
	return n
}
