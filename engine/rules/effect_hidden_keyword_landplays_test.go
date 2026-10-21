package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	hiddenTestNonflyer = "Name:Test Nonflyer\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	hiddenTestFlyer    = "Name:Test Flyer\nManaCost:1 U\nTypes:Creature Bird\nPT:1/1\nK:Flying\nOracle:x\n"
	hiddenTestAttacker = "Name:Test Attacker\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
)

// TestFalterEffectCantBlock pins the reported card end to end: after Falter
// resolves, a creature without flying is not offered as a blocker this turn,
// a flyer still is, and the restriction is gone on the next turn.
func TestFalterEffectCantBlock(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Falter"), card(t, hiddenTestAttacker)},
		[]*cards.Card{card(t, hiddenTestNonflyer), card(t, hiddenTestFlyer)})
	ensureInHand(t, e, 0, "Falter")
	attacker := moveByName(t, e, 0, "Test Attacker", state.ZBattlefield)
	nonflyer := moveByName(t, e, 1, "Test Nonflyer", state.ZBattlefield)
	flyer := moveByName(t, e, 1, "Test Flyer", state.ZBattlefield)

	addMana(t, e, 0, "1R")
	castCardNow(t, e, "Falter")
	passUntilStackEmpty(t, e, 40)

	declareAttackersOnly(t, e, 0, 1, attacker)
	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("no blockers decision after Falter resolved")
	}
	if findBlockOption(d, nonflyer, attacker) != nil {
		t.Fatalf("nonflyer offered as a blocker despite Falter: %+v", d.Options)
	}
	if findBlockOption(d, flyer, attacker) == nil {
		t.Fatalf("flyer not offered as a blocker: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("submit empty blockers: %v", err)
	}

	driveToStepAny(t, e, 3, 0, state.StepMain1)
	declareAttackersOnly(t, e, 0, 1, attacker)
	d = askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("no blockers decision on the following turn")
	}
	if findBlockOption(d, nonflyer, attacker) == nil {
		t.Fatalf("nonflyer still blocked on the next turn (Falter did not expire): %+v", d.Options)
	}
}

// TestExploreExtraLandPlay pins the reported card end to end: after Explore
// resolves its controller is offered a second land play that turn, and the
// extra drop is gone on the next turn.
func TestExploreExtraLandPlay(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Explore")}, nil)
	ensureInHand(t, e, 0, "Explore")
	addMana(t, e, 0, "1G")
	castCardNow(t, e, "Explore")
	passUntilStackEmpty(t, e, 40)
	if e.Pending() == nil {
		e.priorityRound()
	}

	lands := handLandIDs(e, 0)
	if len(lands) < 4 {
		t.Fatalf("precondition: hand has %d lands, want at least 4", len(lands))
	}
	if !playOneLand(t, e, 0, lands[0]) {
		t.Fatalf("first land not offered after Explore: %+v", e.Pending().Options)
	}
	if !playOneLand(t, e, 0, lands[1]) {
		t.Fatalf("second land not offered after Explore: %+v", e.Pending().Options)
	}
	if got := e.G.Players[0].LandsPlayed; got != 2 {
		t.Fatalf("LandsPlayed = %d after Explore's extra drop, want 2", got)
	}

	driveToStepAny(t, e, 3, 0, state.StepMain1)
	if e.Pending() == nil {
		e.priorityRound()
	}
	lands = handLandIDs(e, 0)
	if len(lands) < 2 {
		t.Fatalf("precondition: next turn's hand has %d lands, want at least 2", len(lands))
	}
	if !playOneLand(t, e, 0, lands[0]) {
		t.Fatalf("ordinary land drop not offered on the next turn: %+v", e.Pending().Options)
	}
	if playOneLand(t, e, 0, lands[1]) {
		t.Fatal("Explore's extra land drop survived into the next turn")
	}
	replayCheck(t, e, cfg)
}

// handLandIDs returns the land cards in p's hand in zone order.
func handLandIDs(e *Engine, p state.PlayerID) []state.ObjID {
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZHand, p) {
		o := e.G.Obj(id)
		if o != nil && o.Face() != nil && slices.Contains(o.Face().Types, "Land") {
			out = append(out, id)
		}
	}
	return out
}
