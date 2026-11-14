package rules

// The Madcap Jester's second trigger: "Whenever a spell or ability you control
// causes one or more cards to be exiled from an opponent's library, you may
// play those cards until the end of your next turn." Two reads carry it:
// RememberObjects$ TriggeredCards (the whole batch the trigger captured) and
// ChangesZoneAll ValidCause$ SpellAbility.YouCtrl (the resolving ability that
// moved the cards must be the Jester controller's).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mayPlayGrantRemembering reports whether a may-play grant remembers id.
func mayPlayGrantRemembering(e *Engine, id state.ObjID) bool {
	for i := range e.continuous {
		if ce := &e.continuous[i]; ce.MayPlay && objIDIn(ce.Remembered, id) {
			return true
		}
	}
	return false
}

// TestMadcapJesterBatchPlayable attacks with the Jester in a three-seat game:
// each opponent exiles their top card, the two moves are one batch, and the
// Jester's controller may play both exiled cards.
func TestMadcapJesterBatchPlayable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngineThree(t, reg, nil, nil, nil)
	jester := onBoardReadyCard(t, e, 0, choiceCorpusCard(t, "The Madcap Jester"))
	top1 := e.G.Zone(state.ZLibrary, 1)[0]
	top2 := e.G.Zone(state.ZLibrary, 2)[0]
	if mayPlayGrantRemembering(e, top1) || mayPlayGrantRemembering(e, top2) {
		t.Fatal("precondition: a may-play grant already remembers an opponent's top card")
	}

	// The attack goes at seat 1; the trigger digs every opponent.
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	d := e.Pending()
	pick := -1
	for _, o := range d.Options {
		if o.Obj == jester && o.Player == 1 {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("no attack at seat 1 offered: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 40)

	for _, id := range []state.ObjID{top1, top2} {
		if z := e.G.Obj(id).Zone; z != state.ZExile {
			t.Fatalf("opponent card %d zone = %s, want exile (the attack trigger's dig)", id, z)
		}
		if !mayPlayGrantRemembering(e, id) {
			t.Fatalf("no may-play grant remembers exiled card %d: %+v", id, e.continuous)
		}
	}

	// The grant is live: both exiled lands are offered in the controller's
	// second main phase.
	driveToStep(t, e, e.G.Turn, 0, state.StepMain2)
	for _, id := range []state.ObjID{top1, top2} {
		if optionOf(t, e, "play_land", id) < 0 {
			t.Fatalf("exiled card %d is not offered as playable: %+v", id, e.Pending().Options)
		}
	}
}

// mirrorCard is an opposing permanent whose end-step trigger exiles the top
// card of its OWN controller's library: an opponent's impulse draw, which the
// Jester's "a spell or ability you control" must not see.
const madcapMirror = "Name:Impulse Mirror\nManaCost:1\nTypes:Artifact\n" +
	"T:Mode$ Phase | Phase$ End of Turn | TriggerZones$ Battlefield | Execute$ TrigDig | TriggerDescription$ Exile your top card.\n" +
	"SVar:TrigDig:DB$ Dig | Defined$ You | DestinationZone$ Exile | DigNum$ 1 | ChangeNum$ All\nOracle:x\n"

// TestMadcapJesterIgnoresOpponentCause has an opponent exile their own top
// card; the Jester's controller does not control that ability, so the second
// trigger stays silent and nothing becomes playable.
func TestMadcapJesterIgnoresOpponentCause(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngineThree(t, reg, nil, nil, nil)
	onBoardReadyCard(t, e, 0, choiceCorpusCard(t, "The Madcap Jester"))
	onBoard(t, e, 1, madcapMirror)
	top1 := e.G.Zone(state.ZLibrary, 1)[0]

	// No attack: the Jester's own trigger must not be what exiles anything.
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitChoices(t, e)
	driveToStep(t, e, e.G.Turn, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)

	if z := e.G.Obj(top1).Zone; z != state.ZExile {
		t.Fatalf("precondition: the opponent's own dig left card %d in %s, want exile", top1, z)
	}
	if mayPlayGrantRemembering(e, top1) {
		t.Fatalf("the Jester granted play of a card an opponent's own ability exiled: %+v", e.continuous)
	}
}
