// cr-g01a: CR 903.9a / 704.6d (2026-09-25). A commander put into a
// graveyard or exile goes there FOR REAL -- "dies" and "put into a
// graveyard" triggers fire and every other replacement effect applies to
// that move -- and only then, as a state-based action, may its owner put it
// into the command zone. (CR 903.9b, the hand/library replacement, stays in
// commander_zone_replacement_test.go.)
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// commanderMovedTo reports whether the log carries a MoveZone of id into to.
func commanderMovedTo(l *events.Log, id state.ObjID, to state.Zone) bool {
	for _, ev := range l.Events {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == to {
			return true
		}
	}
	return false
}

// TestCR903_9aDyingCommanderFiresDiesTriggersBeforeTheCommandZoneChoice is
// the foundations-wretched-ranks repro: seat 1's Blood Artist watches seat
// 0's commander die to lethal damage. The commander must really reach the
// graveyard (a logged battlefield->graveyard MoveZone), Blood Artist's
// "another creature dies" trigger must be queued, and only then is the owner
// asked; accepting moves it graveyard -> command zone.
func TestCR903_9aDyingCommanderFiresDiesTriggersBeforeTheCommandZoneChoice(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{tinyCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)
	artist := onBoardCard(t, e, 1, corpusCard(t, "Blood Artist"))
	e.G.Obj(cmd).Damage = 1 // lethal (CR 704.5g)
	before := len(e.pendingTriggers)

	e.checkStateBased()

	if !commanderMovedTo(e.L, cmd, state.ZGraveyard) {
		t.Fatalf("CR 903.9a: the dying commander must be put into the graveyard for real before the command-zone choice; log has no commander MoveZone to the graveyard (zone now %v)", e.G.Obj(cmd).Zone)
	}
	witnessed := 0
	for _, tr := range e.pendingTriggers[before:] {
		if tr.Source == artist {
			witnessed++
		}
	}
	if witnessed != 1 {
		t.Fatalf("CR 903.9a: Blood Artist queued %d dies triggers for the commander, want 1", witnessed)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 || d.Source != cmd {
		t.Fatalf("pending = %+v, want seat 0's commander_zone decision for the commander in the graveyard", d)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "command_zone" || d.Options[1].Kind != "leave" {
		t.Fatalf("options = %+v, want [command_zone, leave]", d.Options)
	}
	submit(t, e, 0)
	if got := e.G.Obj(cmd).Zone; got != state.ZCommand {
		t.Fatalf("accepted commander zone = %v, want command", got)
	}
}

// TestCR903_9aDeclinedCommanderMeetsDauthiVoidwalker is the
// valgavoth-endless-punishment repro: seat 1 controls Dauthi Voidwalker
// ("if a card would be put into an opponent's graveyard from anywhere,
// instead exile it with a void counter on it"). Seat 0's commander dies: the
// replacement applies to that move, so it is exiled with a void counter; the
// owner is then asked (exile is a CR 903.9a zone) and a decline leaves it
// there. Exactly one ask.
func TestCR903_9aDeclinedCommanderMeetsDauthiVoidwalker(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{tinyCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)
	onBoardCard(t, e, 1, corpusCard(t, "Dauthi Voidwalker"))
	e.G.Obj(cmd).Damage = 1

	e.checkStateBased()

	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's commander_zone decision", d)
	}
	submit(t, e, 1) // leave it where it is
	o := e.G.Obj(cmd)
	if o.Zone != state.ZExile {
		t.Fatalf("CR 903.9a: declined commander zone = %v, want exile (Dauthi Voidwalker's replacement applies to the commander's graveyard-bound move)", o.Zone)
	}
	if got := o.Counter("VOID"); got != 1 {
		t.Fatalf("exiled commander VOID counters = %d, want 1", got)
	}
	assertAskCount(t, e.L, 1)
}

// TestCR903_9aCounteredCommanderIsAskedAtTheNextStateBasedCheck pins the
// "from anywhere" source and the SBA timing together: a commander spell
// countered off the stack lands in the graveyard with no ask at all, and the
// next state-based-action check (one that follows a quiet check, so the
// quiet skip cannot hide the arrival) offers the command zone.
func TestCR903_9aCounteredCommanderIsAskedAtTheNextStateBasedCheck(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{tinyCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: cmd, From: state.ZBattlefield, To: state.ZStack})
	e.checkStateBased() // nothing to do: records the quiet key
	if d := e.Pending(); d != nil {
		t.Fatalf("a commander on the stack must not be asked: pending = %+v", d)
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: cmd, From: state.ZStack, To: state.ZGraveyard, Text: "countered"})
	if d := e.Pending(); d != nil {
		t.Fatalf("CR 903.9a: the countered commander's move to the graveyard is not replaced; nothing may be asked before the state-based check, pending = %+v", d)
	}
	if got := e.G.Obj(cmd).Zone; got != state.ZGraveyard {
		t.Fatalf("countered commander zone = %v, want graveyard", got)
	}

	e.checkStateBased()
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
		t.Fatalf("pending = %+v, want the CR 903.9a commander_zone decision at the next check", d)
	}
	submit(t, e, 0)
	if got := e.G.Obj(cmd).Zone; got != state.ZCommand {
		t.Fatalf("accepted commander zone = %v, want command", got)
	}
	assertAskCount(t, e.L, 1)
}

// TestCR903_9aDepartedOwnersAskDoesNotBlockLaterCommanders: in a three-seat
// game seat 0's commander reaches the graveyard and, at the same
// state-based check, seat 0 loses (CR 704.5a). The ask posed to the departed
// owner is released (CR 800.4a), and its parked entry must go with it --
// otherwise every later commander's CR 903.9a choice waits behind it
// forever. Seat 1's commander then dies and seat 1 is asked.
func TestCR903_9aDepartedOwnersAskDoesNotBlockLaterCommanders(t *testing.T) {
	t.Parallel()
	e, _ := commanderGame(t, commanderDamageSeed, FormatCommander, 40, [][]string{{tinyCmdSrc}, {tinyCmdSrc2}, {}})
	a := fieldCommander(t, e, 0, 0)
	b := fieldCommander(t, e, 1, 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: a, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -40})

	e.checkStateBased()

	if !e.G.Players[0].Lost {
		t.Fatal("fixture: seat 0 at 0 life must lose at the check")
	}
	if len(e.cmdZone) != 0 {
		t.Fatalf("the departed owner's released ask left %d parked command-zone entries", len(e.cmdZone))
	}
	e.emit(events.Event{Kind: events.Damage, Obj: b, Amount: 1})
	d := passPriorityUntilNonPriority(t, e)
	if d.Kind != decision.KCommanderZone || d.Player != 1 || d.Source != b {
		t.Fatalf("pending = %s for seat %d, want seat 1's commander_zone decision for its dead commander", pendingKind(d), d.Player)
	}
}

// TestCR903_9aDepartedOwnersReleasedAskDoesNotStrandTheQueueBehindIt is the
// multi-entry case of the departure release: two hand-bound (CR 903.9b)
// parks in one burst -- seat 0's commander asked, seat 1's parked behind --
// and then seat 0 loses at the same state-based check. Releasing seat 0's
// ask (CR 800.4a) must drop seat 0's entry, keep seat 1's, and pose seat 1's
// ask right there: the released ask was the one that would have handed the
// queue on, and commanderZoneSBA refuses to pose while the queue is
// non-empty, so a stranded entry would block every later commander-zone ask
// for the rest of the game.
func TestCR903_9aDepartedOwnersReleasedAskDoesNotStrandTheQueueBehindIt(t *testing.T) {
	t.Parallel()
	e, _ := commanderGame(t, commanderDamageSeed, FormatCommander, 40, [][]string{{tinyCmdSrc}, {tinyCmdSrc2}, {}})
	a := fieldCommander(t, e, 0, 0)
	b := fieldCommander(t, e, 1, 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: a, From: state.ZBattlefield, To: state.ZHand})
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
		t.Fatalf("fixture: pending after seat 0's hand-bound park = %s, want seat 0's commander_zone ask", pendingKind(d))
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: b, From: state.ZBattlefield, To: state.ZHand})
	if len(e.cmdZone) != 2 {
		t.Fatalf("fixture: %d parked moves, want 2 (seat 1's parked behind seat 0's ask)", len(e.cmdZone))
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -40})

	e.checkStateBased()

	if !e.G.Players[0].Lost {
		t.Fatal("fixture: seat 0 at 0 life must lose at the check")
	}
	if len(e.cmdZone) != 1 {
		t.Fatalf("parked entries after the release = %d, want 1 (seat 1's; seat 0's dropped with the released ask)", len(e.cmdZone))
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KCommanderZone || d.Player != 1 || d.Source != b {
		t.Fatalf("pending = %s for seat %d, want seat 1's commander_zone ask posed by the release", pendingKind(d), d.Player)
	}
	if got := e.G.Obj(b).Zone; got != state.ZBattlefield {
		t.Fatalf("seat 1's commander zone while asked = %v, want battlefield (its move is still parked)", got)
	}
	submit(t, e, 0) // seat 1 accepts: into the command zone
	if got := e.G.Obj(b).Zone; got != state.ZCommand {
		t.Fatalf("accepted commander zone = %v, want command", got)
	}
	if len(e.cmdZone) != 0 {
		t.Fatalf("parked entries after the answer = %d, want 0", len(e.cmdZone))
	}
	// The queue is clean, so a later commander death is asked as usual.
	fieldCommanderByID(t, e, b)
	e.G.Obj(b).Damage = 1
	d = passPriorityUntilNonPriority(t, e)
	if d.Kind != decision.KCommanderZone || d.Player != 1 {
		t.Fatalf("pending after the later death = %s, want seat 1's commander_zone ask", pendingKind(d))
	}
}
