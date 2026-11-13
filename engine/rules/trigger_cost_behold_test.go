package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredBeholdCost (CR 701.4a): Sarkhan, Dragon Ascendant's ETB body
// `AB$ Token | Cost$ Behold<1/Dragon>` ("you may behold a Dragon. If you
// do, create a Treasure token") is payable through the triggered-cost
// window, not decline-only. The helpers are trigger_cost_forage_test.go's
// forageCostAsk/foragePayIndex/forageOption and search_library_test.go's
// searchEngine/searchMoveByName (corpus cards only).

// beholdFixture builds the corpus engine with the given Shivan Dragons moved
// to the battlefield and to the hand, then Sarkhan entering last, and returns
// the engine, its config and every Dragon id in seat 0's battlefield + hand.
func beholdFixture(t *testing.T, onField, inHand int) (*Engine, Config, []state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	fixtures := []string{"Sarkhan, Dragon Ascendant"}
	for i := 0; i < onField+inHand; i++ {
		fixtures = append(fixtures, "Shivan Dragon")
	}
	e, cfg := searchEngine(t, reg, fixtures...)
	for i := 0; i < onField; i++ {
		id := searchMoveByName(t, e, "Shivan Dragon", state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("fixture: Dragon %d not on the battlefield", id)
		}
	}
	for i := 0; i < inHand; i++ {
		searchMoveByName(t, e, "Shivan Dragon", state.ZHand)
	}
	searchMoveByName(t, e, "Sarkhan, Dragon Ascendant", state.ZBattlefield)
	var dragons []state.ObjID
	for _, z := range []state.Zone{state.ZBattlefield, state.ZHand} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Shivan Dragon" {
				dragons = append(dragons, id)
			}
		}
	}
	if len(dragons) != onField+inHand {
		t.Fatalf("fixture: %d Dragons on battlefield+hand, want %d", len(dragons), onField+inHand)
	}
	return e, cfg, dragons
}

func beholdTreasures(e *Engine) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Treasure Token" {
			n++
		}
	}
	return n
}

func beholdNote(evs []events.Event) *events.Event {
	for i := range evs {
		if evs[i].Kind == events.Note && strings.HasPrefix(evs[i].Text, "beheld ") && strings.HasSuffix(evs[i].Text, " as a cost") {
			return &evs[i]
		}
	}
	return nil
}

func TestTriggeredBeholdCost(t *testing.T) {
	t.Parallel()
	pay := func(t *testing.T, onField, inHand int, wantPick bool) {
		e, cfg, dragons := beholdFixture(t, onField, inHand)
		d := forageCostAsk(t, e)
		if tc := e.triggerCost; tc == nil || len(tc.amount.Behold) != 1 {
			t.Fatalf("precondition: pending window is not the Behold cost (ask %+v)", d)
		}
		payIdx := optionIndexForKind(d, "trigger_cost_pay")
		if payIdx < 0 {
			t.Fatalf("Behold<1/Dragon> with %d Dragon(s) offers no pay: %+v", len(dragons), d.Options)
		}
		if beholdTreasures(e) != 0 {
			t.Fatalf("precondition: a Treasure before paying")
		}
		start := len(e.L.Events)
		submitChoices(t, e, payIdx)
		beheld := dragons[0]
		if wantPick {
			pick := e.Pending()
			if pick == nil || pick.Kind != decision.KChoose || len(pick.Options) != len(dragons) {
				t.Fatalf("after pay with %d Dragons: %+v, want the behold pick over each", len(dragons), pick)
			}
			for _, o := range pick.Options {
				if o.Kind != "beholdcost" {
					t.Fatalf("behold pick option %+v, want kind beholdcost", o)
				}
			}
			beheld = pick.Options[1].Obj
			submitChoices(t, e, 1)
		}
		passUntilNonPriority(t, e, 50)
		note := beholdNote(e.L.Events[start:])
		if note == nil || len(note.IDs) != 1 || note.IDs[0] != beheld {
			t.Fatalf("behold Note = %+v, want one naming %d", note, beheld)
		}
		if got := beholdTreasures(e); got != 1 {
			t.Fatalf("Treasures after paying = %d, want 1", got)
		}
		for _, id := range dragons {
			if o := e.G.Obj(id); o == nil || (o.Zone != state.ZBattlefield && o.Zone != state.ZHand) {
				t.Fatalf("beholding moved Dragon %d: %+v", id, o)
			}
		}
		replayCheck(t, e, cfg)
	}
	t.Run("OneDragonOnBattlefieldUnasked", func(t *testing.T) { pay(t, 1, 0, false) })
	t.Run("OneDragonInHandUnasked", func(t *testing.T) { pay(t, 0, 1, false) })
	t.Run("TwoDragonsPosePick", func(t *testing.T) { pay(t, 1, 1, true) })
	t.Run("NoDragonDeclineOnly", func(t *testing.T) {
		e, _, _ := beholdFixture(t, 0, 0)
		d := forageCostAsk(t, e)
		if tc := e.triggerCost; tc == nil || len(tc.amount.Behold) != 1 {
			t.Fatalf("precondition: pending window is not the Behold cost (ask %+v)", d)
		}
		if len(d.Options) != 1 || d.Options[0].Kind != "trigger_cost_decline" {
			t.Fatalf("no Dragon: options %+v, want only trigger_cost_decline", d.Options)
		}
	})
}
