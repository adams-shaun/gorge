// cr-g01b: CR 903.9b (2026-09-25). The hand/library commander replacement
// stays a replacement, but declining it must leave the move subject to every
// other replacement effect that applies to it.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// fleetingCmdSrc is a 1/1 legendary commander carrying its own "if this
// would leave the battlefield, exile it instead" replacement (the shape the
// corpus's Origin$ Battlefield self-replacements take), so a bounce to hand
// meets BOTH the CR 903.9b commander replacement and a card replacement.
const fleetingCmdSrc = `Name:Fleeting Tiny
ManaCost:0
Types:Legendary Creature Spirit
PT:1/1
R:Event$ Moved | ActiveZones$ Battlefield | Origin$ Battlefield | ExcludeDestination$ Exile | ValidCard$ Card.Self | ReplaceWith$ LeaveExile | Description$ If this would leave the battlefield, exile it instead.
SVar:LeaveExile:DB$ ChangeZone | Hidden$ True | Origin$ All | Destination$ Exile | Defined$ ReplacedCard
Oracle:x
`

// TestCR903_9bDeclineKeepsOtherReplacementsOnTheMove is the CR 903.9b
// decline arm: the commander is bounced to its owner's hand while carrying
// its own "exile it instead" leave-the-battlefield replacement. Declining
// the command zone must not suppress that replacement: the commander is
// exiled, not returned to hand -- and, now in exile, it is offered the
// command zone again by CR 903.9a at the next check.
func TestCR903_9bDeclineKeepsOtherReplacementsOnTheMove(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{fleetingCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)

	e.emit(events.Event{Kind: events.MoveZone, Obj: cmd, From: state.ZBattlefield, To: state.ZHand})
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone {
		t.Fatalf("pending = %+v, want the CR 903.9b commander_zone decision for the bounce", d)
	}
	submit(t, e, 1) // decline the command zone

	if got := e.G.Obj(cmd).Zone; got != state.ZExile {
		t.Fatalf("CR 903.9b: declined bounce zone = %v, want exile (the commander's own exile-instead replacement still applies)", got)
	}
	if commanderMovedTo(e.L, cmd, state.ZHand) {
		t.Fatal("the commander reached the hand: the decline suppressed the other replacement")
	}
	// Submit's tail ran the state-based check: the exile arrival is asked.
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone || d.Source != cmd {
		t.Fatalf("pending = %+v, want the CR 903.9a commander_zone decision for the exiled commander", d)
	}
	assertAskCount(t, e.L, 2)
}

// TestCR903_9bDeclineIsNotReofferedForTheSameMove is the guard half: a plain
// bounce (no other replacement) declined goes to the hand and the decline
// is never re-offered for that move -- exactly one ask.
func TestCR903_9bDeclineIsNotReofferedForTheSameMove(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{tinyCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)

	e.emit(events.Event{Kind: events.MoveZone, Obj: cmd, From: state.ZBattlefield, To: state.ZHand})
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone {
		t.Fatalf("pending = %+v, want the CR 903.9b commander_zone decision", d)
	}
	submit(t, e, 1)
	if got := e.G.Obj(cmd).Zone; got != state.ZHand {
		t.Fatalf("declined bounce zone = %v, want hand", got)
	}
	if d := e.Pending(); d == nil || d.Kind == decision.KCommanderZone {
		t.Fatalf("after the decline the game must continue to a real decision, got %+v", d)
	}
	assertAskCount(t, e.L, 1)
}
