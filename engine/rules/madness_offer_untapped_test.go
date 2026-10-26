package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// madnessAskAfterDiscard discards Fiery Temper with lands untapped Mountains
// on the battlefield and an empty pool, then resolves the madness trigger up
// to its cast-or-not ask.
func madnessAskAfterDiscard(t *testing.T, lands int) (*Engine, Config, *decision.Decision, state.ObjID) {
	t.Helper()
	e, cfg, _ := altCostEngine(t, 935, []string{"Fiery Temper"}, nil, nil)
	for i := 0; i < lands; i++ {
		moveToBattlefieldByName(t, e, 0, "Mountain")
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool = %d, want empty", got)
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != lands {
		t.Fatalf("precondition: %d permanents, want %d untapped Mountains", got, lands)
	}
	temper := findCardObj(t, e, 0, "Fiery Temper", state.ZHand)
	e.pending = nil
	e.emit(events.Discard(temper, 0))
	e.priorityRound()
	passOnceP(t, e)
	passOnceP(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional || d.Source != temper {
		t.Fatalf("madness resolution choice: %+v", d)
	}
	return e, cfg, d, temper
}

// TestMadnessOfferedWithUntappedLands: CR 702.35b with CR 601.2g -- the
// madness cast is offered whenever untapped sources can pay for it, not only
// against floating mana; the cast then taps a Mountain for the {R}.
func TestMadnessOfferedWithUntappedLands(t *testing.T) {
	t.Parallel()
	t.Run("two untapped Mountains", func(t *testing.T) {
		e, cfg, d, temper := madnessAskAfterDiscard(t, 2)
		yes, no := -1, -1
		for _, o := range d.Options {
			switch o.Kind {
			case "yes":
				yes = o.Index
			case "no":
				no = o.Index
			}
		}
		if yes < 0 || no < 0 || len(d.Options) != 2 {
			t.Fatalf("madness ask options = %+v, want yes and no", d.Options)
		}
		submitChoices(t, e, yes)
		// Answer the cast's own asks (Temper's target, then the payment
		// window) until priority returns; the payment taps a Mountain.
		for i := 0; i < 8; i++ {
			d := e.Pending()
			if d == nil || d.Kind == decision.KPriority {
				break
			}
			submitChoices(t, e, 0)
		}
		if z := e.G.Obj(temper).Zone; z != state.ZStack {
			t.Fatalf("Fiery Temper in %s after choosing yes, want stack (pending %+v)", z, e.Pending())
		}
		tapped := 0
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if e.G.Obj(id).Tapped {
				tapped++
			}
		}
		if tapped != 1 {
			t.Fatalf("%d Mountains tapped, want exactly 1 for {R}", tapped)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("no untapped red source", func(t *testing.T) {
		_, _, d, _ := madnessAskAfterDiscard(t, 0)
		if len(d.Options) != 1 || d.Options[0].Kind != "no" {
			t.Fatalf("madness ask options = %+v, want only no", d.Options)
		}
	})
}
