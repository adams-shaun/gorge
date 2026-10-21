// cr-g53a: CR 702.22c-i, 802.3b. Banding's attack half: as attackers are
// declared, the attacking player may put attacking creatures with banding
// and up to one attacking creature without banding, all attacking the same
// defender, into a band; a creature blocking one member blocks every member.
// Authored fixtures (the keyword is the corpus's own K:Banding spelling).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	bandKnightSrc = "Name:Band Knight\nManaCost:W\nTypes:Creature Human Knight\nPT:1/1\nK:Banding\nOracle:x\n"
	bandBearSrc   = "Name:Plain Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	bandBear2Src  = "Name:Other Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	bandWallSrc   = "Name:Wall Bear\nManaCost:2 G\nTypes:Creature Bear\nPT:0/6\nOracle:x\n"
)

// bandAskOptionFor returns the pending band ask's option index naming obj,
// failing when the pending decision is not a band ask.
func bandAskOptionFor(t *testing.T, e *Engine, obj state.ObjID) (int, *decision.Decision) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "band" {
		t.Fatalf("CR 702.22c: pending = %+v, want the band declaration ask", d)
	}
	for _, o := range d.Options {
		if o.Obj == obj && obj != 0 {
			return o.Index, d
		}
	}
	return -1, d
}

// TestCR702_22cBandIsDeclaredAndABlockerBlocksTheWholeBand: seat 0 attacks
// seat 1 with Band Knight (banding) and Plain Bear. The band ask offers the
// Bear; banding them makes one band. Seat 1 blocks only the Bear with Wall
// Bear, and the Knight becomes blocked by the Wall too (CR 702.22h).
func TestCR702_22cBandIsDeclaredAndABlockerBlocksTheWholeBand(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	knight := onBoardReady(t, e, 0, bandKnightSrc)
	bear := onBoardReady(t, e, 0, bandBearSrc)
	wall := onBoard(t, e, 1, bandWallSrc)
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	submitAttackersOnly(t, e, knight, bear)
	idx, d := bandAskOptionFor(t, e, bear)
	if idx < 0 || d.Source != knight || d.Player != 0 {
		t.Fatalf("band ask = %+v, want seat 0 asked to band Plain Bear with Band Knight", d)
	}
	submitChoices(t, e, idx)
	drainCombatPriority(t, e)
	e.G.Step = state.StepDeclareBlockers
	e.askBlockers()
	d = e.Pending()
	pick := -1
	for _, o := range d.Options {
		if o.Obj == wall && o.Attacker == bear {
			pick = o.Index
		}
	}
	if d == nil || d.Kind != decision.KBlockers || pick < 0 {
		t.Fatalf("blockers decision = %+v, want Wall Bear blocking Plain Bear offered", d)
	}
	submitChoices(t, e, pick)
	if got := e.G.Obj(knight).BlockedBy; len(got) != 1 || got[0] != wall {
		t.Fatalf("CR 702.22h: Band Knight BlockedBy = %v, want [Wall Bear %d]", got, wall)
	}
	if got := e.G.Obj(bear).BlockedBy; len(got) != 1 || got[0] != wall {
		t.Fatalf("Plain Bear BlockedBy = %v, want [Wall Bear %d]", got, wall)
	}
}

// TestCR702_22cBandTakesAtMostOneCreatureWithoutBanding: with two Bears
// attacking beside Band Knight, once one Bear is in the band the other is no
// longer offered.
func TestCR702_22cBandTakesAtMostOneCreatureWithoutBanding(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	knight := onBoardReady(t, e, 0, bandKnightSrc)
	bear := onBoardReady(t, e, 0, bandBearSrc)
	bear2 := onBoardReady(t, e, 0, bandBear2Src)
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	submitAttackersOnly(t, e, knight, bear, bear2)
	idx, _ := bandAskOptionFor(t, e, bear)
	if idx < 0 {
		t.Fatal("CR 702.22c: Plain Bear not offered for Band Knight's band")
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "band" {
		t.Fatalf("CR 702.22c: a second creature without banding was offered for the band: %+v", d.Options)
	}
}
