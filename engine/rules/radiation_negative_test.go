package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRadiationNegativePowerGivesNone drives the REAL corpus trigger through
// the Ghoul's own death: CR 107.1b, a negative number in "equal to its power"
// is treated as zero, so an opponent's rad counters are untouched. Eight M1M1
// counters make the 2/2 a -6/-6; the toughness SBA sends it to the graveyard
// and its dies trigger fires with that last power.
func TestRadiationNegativePowerGivesNone(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	ghoul := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Feral Ghoul"))
	trig := resolveSVarOf(t, e.G.Obj(ghoul).Face(), "TrigRadiation")
	if trig.API != "Radiation" || trig.Params["Defined"] != "Opponent" || trig.Params["Num"] != "TriggeredCard$CardPower" {
		t.Fatalf("test precondition: TrigRadiation = %+v, want DB$ Radiation/Opponent/TriggeredCard$CardPower", trig)
	}
	seedPlayerCounter(t, e, 1, "RAD", 4)
	if got := e.G.Players[1].Counter("RAD"); got != 4 {
		t.Fatalf("precondition: opponent RAD=%d, want 4", got)
	}

	e.emit(events.Event{Kind: events.CounterChange, Obj: ghoul, Counter: "M1M1", Amount: 8})
	e.checkStateBased()
	if got := e.G.Obj(ghoul).Zone; got != state.ZGraveyard {
		t.Fatalf("precondition: Ghoul zone=%v, want graveyard (the -6/-6 died)", got)
	}
	e.putTriggersOnStack()
	e.resolveTop()

	if got := e.G.Players[1].Counter("RAD"); got != 4 {
		t.Fatalf("opponent RAD=%d after a Ghoul died with negative power, want 4 (CR 107.1b: the negative counts as zero)", got)
	}
}
