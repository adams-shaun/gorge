package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 603.10a: "leaves-the-battlefield abilities, abilities that trigger when
// a player sacrifices a permanent, ..." look back in time: they trigger from
// the board as it was immediately before the event, so a permanent that
// leaves in the same event still triggers. A Mode$ ChangesZoneAll line with
// Origin$ Battlefield ("whenever one or more other creatures die") and a
// Mode$ Sacrificed line are such abilities. Real corpus cards only.

// cr603LookBackDrive passes priority until the stack is empty and nothing is
// pending, answering a trigger-order ask with the offered order, a target ask
// with the first offered opponent (else the first option), and a choose ask
// with its first option.
func cr603LookBackDrive(t *testing.T, e *Engine, d *decision.Decision) {
	t.Helper()
	for i := 0; i < 60; i++ {
		if d == nil {
			t.Fatalf("CR 603.10a seq %d: no decision pending", len(e.L.Events))
		}
		switch d.Kind {
		case decision.KTriggerOrder:
			answerTriggerOrders(t, e)
		case decision.KTarget:
			pick := d.Options[0].Index
			for _, o := range d.Options {
				if o.Obj == 0 && o.Player != d.Player {
					pick = o.Index
					break
				}
			}
			submitChoices(t, e, pick)
		case decision.KChoose:
			submitChoices(t, e, d.Options[0].Index)
		case decision.KPriority:
			if stackEmpty(e) && len(e.pendingTriggers) == 0 {
				return
			}
		default:
			t.Fatalf("CR 603.10a seq %d: unexpected %s decision", len(e.L.Events), d.Kind)
		}
		d = passUntilResolved(t, e, 40)
	}
	t.Fatalf("CR 603.10a seq %d: the game did not settle", len(e.L.Events))
}

// TestCR603_10aMorbidOpportunistSeesItsOwnWrath: Day of Judgment destroys
// Morbid Opportunist together with Grizzly Bears. Its "whenever one or more
// other creatures die, draw a card" (Mode$ ChangesZoneAll | Origin$
// Battlefield) looks back, so it triggers for the Bears whichever of the two
// the sweep moves first: one draw. "bears first" is the guard (the
// Opportunist is still on the battlefield when the Bears die, true before
// and after the fix); "opportunist first" fails without the look-back.
func TestCR603_10aMorbidOpportunistSeesItsOwnWrath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		bearsFirst bool
	}{{"opportunist first", false}, {"bears first", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := searchTestRegistry(t)
			e, cfg := searchEngine(t, reg, "Day of Judgment", "Morbid Opportunist")
			var opp, bears state.ObjID
			if tc.bearsFirst {
				bears = searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
				opp = searchMoveByName(t, e, "Morbid Opportunist", state.ZBattlefield)
			} else {
				opp = searchMoveByName(t, e, "Morbid Opportunist", state.ZBattlefield)
				bears = searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
			}
			draws0 := zallDraws(t, e, 0)

			d := castSpellNamed(t, e, reg, "Day of Judgment", "WWWW")
			cr603LookBackDrive(t, e, d)

			for _, id := range []state.ObjID{opp, bears} {
				if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
					t.Fatalf("CR 603.10a Day of Judgment seq %d: object %d did not die", len(e.L.Events), id)
				}
			}
			if got := zallDraws(t, e, 0) - draws0; got != 1 {
				t.Errorf("CR 603.10a Day of Judgment seq %d: %d draw(s), want 1 (Morbid Opportunist looks back at the Bears dying with it)", len(e.L.Events), got)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestCR603_10aMayhemDevilSeesSacrificesWithIt: Living Death makes seat 0
// sacrifice Mayhem Devil and Grizzly Bears together, the Devil first.
// "Whenever a player sacrifices a permanent" (Mode$ Sacrificed) looks back,
// so the Devil triggers for both sacrifices, its own and the Bears', and
// deals 2 damage to the opponent. Without the look-back the Bears' sacrifice
// is matched after the Devil has left: 1 damage.
func TestCR603_10aMayhemDevilSeesSacrificesWithIt(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Living Death", "Mayhem Devil")
	devil := searchMoveByName(t, e, "Mayhem Devil", state.ZBattlefield)
	bears := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	life1 := e.G.Players[1].Life

	d := castSpellNamed(t, e, reg, "Living Death", "BBBBB")
	cr603LookBackDrive(t, e, d)

	for _, id := range []state.ObjID{devil, bears} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("CR 603.10a Living Death seq %d: object %d was not sacrificed", len(e.L.Events), id)
		}
	}
	if got := e.G.Players[1].Life; got != life1-2 {
		t.Errorf("CR 603.10a Living Death seq %d: opponent life %d, want %d (Mayhem Devil sees both sacrifices)",
			len(e.L.Events), got, life1-2)
	}
	replayCheck(t, e, cfg)
}
