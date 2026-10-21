package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// lastExtraPhaseConsume returns the source of the most recent ExtraPhase
// consume (-1) event: the grant whose extra phase the walk just entered.
func lastExtraPhaseConsume(e *Engine) state.ObjID {
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		if ev := e.L.Events[i]; ev.Kind == events.ExtraPhase && ev.Amount == -1 {
			return ev.Obj
		}
	}
	return 0
}

// TestCR500_8MostRecentExtraPhaseOccursFirst: CR 500.8, "If multiple extra
// phases are created after the same phase, the most recently created phase
// will occur first." In one precombat main phase Moraug, Fury of Akoum's
// landfall grants an extra combat (whose beginning untaps the team), and
// then Relentless Assault grants another. Relentless Assault's combat, the
// more recent, comes first: the tapped Bear is still tapped at its attackers
// ask, and Moraug's combat follows and untaps it.
func TestCR500_8MostRecentExtraPhaseOccursFirst(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := addPhaseEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Moraug, Fury of Akoum"), lookup(t, reg, "Relentless Assault"), card(t, bearSrc)},
		[]*cards.Card{})
	moraug := moveByName(t, e, 0, "Moraug, Fury of Akoum", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Bear", state.ZBattlefield)
	for driveToTurn(t, e, 3, 0) {
	}
	// Grant 1: Moraug's landfall in Main1.
	moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	passOnce(t, e)
	passUntilStackEmpty(t, e, 40)
	// Grant 2: Relentless Assault, cast in the same Main1.
	ra := moveByName(t, e, 0, "Relentless Assault", state.ZHand)
	for _, r := range "CCRR" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
	}
	e.priorityRound()
	d := e.Pending()
	idx := -1
	if d != nil && d.Kind == decision.KPriority {
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == ra {
				idx = o.Index
			}
		}
	}
	if idx < 0 {
		t.Fatalf("CR 500.8 Relentless Assault seq %d: no cast option (pending %+v)", len(e.L.Events), d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("CR 500.8 Relentless Assault seq %d: cast: %v", len(e.L.Events), err)
	}
	passUntilStackEmpty(t, e, 40)
	if e.G.Step != state.StepMain1 || len(e.G.ExtraPhases) != 2 ||
		e.G.ExtraPhases[0].Source != moraug || e.G.ExtraPhases[1].Source != ra ||
		e.G.ExtraPhases[0].AfterStep != state.StepMain1 || e.G.ExtraPhases[1].AfterStep != state.StepMain1 {
		t.Fatalf("CR 500.8 seq %d: precondition: step %s, want two grants after Main1, Moraug's then Relentless Assault's: %+v",
			len(e.L.Events), e.G.Step, e.G.ExtraPhases)
	}
	// Tap the Bear now, after this turn's untap step: only Moraug's
	// extra-combat beginning can untap it this turn.
	e.emit(events.Event{Kind: events.Tap, Obj: bear})
	// Leave Main1: the FIRST extra combat is the most recently created one.
	passToKind(t, e, decision.KAttackers)
	if got := lastExtraPhaseConsume(e); got != ra {
		t.Fatalf("CR 500.8 seq %d: first extra combat entered was grant source %d, want Relentless Assault %d (most recent first; Moraug is %d)",
			len(e.L.Events), got, ra, moraug)
	}
	if !e.G.Obj(bear).Tapped {
		t.Errorf("CR 500.8 seq %d: Moraug's untap fired at the first extra combat; it belongs to the second", len(e.L.Events))
	}
	// Decline: Moraug's extra combat comes next and its beginning untaps the Bear.
	submitNoAttackers(t, e)
	passToKind(t, e, decision.KAttackers)
	if got := lastExtraPhaseConsume(e); got != moraug {
		t.Fatalf("CR 500.8 seq %d: second extra combat entered was grant source %d, want Moraug %d", len(e.L.Events), got, moraug)
	}
	if e.G.Obj(bear).Tapped {
		t.Errorf("CR 500.8 seq %d: Moraug's extra combat began without untapping the Bear", len(e.L.Events))
	}
	// Decline again: both grants have completed, and the ordinary combat
	// follows.
	submitNoAttackers(t, e)
	passToKind(t, e, decision.KAttackers)
	if e.G.Turn != 3 || e.G.Step != state.StepDeclareAttackers {
		t.Fatalf("CR 500.8 seq %d: step %s turn %d, want the ordinary combat's attackers ask on turn 3", len(e.L.Events), e.G.Step, e.G.Turn)
	}
	submitNoAttackers(t, e)
	driveToStep(t, e, 3, 0, state.StepEnd)
	if len(e.G.ExtraPhases) != 0 {
		t.Fatalf("CR 500.8 seq %d: ExtraPhases not drained: %+v", len(e.L.Events), e.G.ExtraPhases)
	}
	replayCheck(t, e, cfg)
}
