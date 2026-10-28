package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// evokeAdamantRun casts the named Incarnation for its evoke cost with the
// given mana, answers the KTriggerOrder ask so the evoke sacrifice resolves
// first (or last), answers every other non-priority ask with option 0, and
// passes until the stack is empty. It returns the card id.
func evokeAdamantRun(t *testing.T, name, mana string, sacrificeFirst bool, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, _ := altCostEngine(t, seed, []string{name}, nil, nil)
	id := findCardObj(t, e, 0, name, state.ZHand)
	addMana(t, e, 0, mana)
	submitChoices(t, e, castModeOption(t, e, id, "evoked"))
	sawOrder := false
	for i := 0; i < 80 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending (stack %v)", e.G.Stack)
		}
		switch d.Kind {
		case decision.KPriority:
			if len(e.G.Stack) == 0 && e.G.Obj(id).Zone != state.ZStack {
				goto done
			}
			passOnceP(t, e)
		case decision.KTriggerOrder:
			sawOrder = true
			sac := -1
			for _, o := range d.Options {
				if strings.Contains(o.Label, "sacrifice it (evoked)") {
					sac = o.Index
				}
			}
			if sac < 0 || len(d.Options) != 2 {
				t.Fatalf("trigger order did not offer the evoke sacrifice with one enter trigger: %+v", d.Options)
			}
			// Choice[0] is pushed first, so it resolves last.
			order := []int{sac, 1 - sac}
			if sacrificeFirst {
				order = []int{1 - sac, sac}
			}
			submitChoices(t, e, order...)
		default:
			submitChoices(t, e, d.Options[0].Index)
		}
	}
done:
	if !sawOrder {
		t.Fatalf("the evoke sacrifice and the enter trigger were never offered for ordering")
	}
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("%s ended in %s, want graveyard", name, o.Zone)
	}
	return e, cfg, id
}

func interveningIfFizzles(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && strings.Contains(ev.Text, "fizzled: intervening-if no longer holds") {
			n++
		}
	}
	return n
}

func tokensControlledBy(e *Engine, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.IsToken {
			n++
		}
	}
	return n
}

func TestEvokedCatharsisSacrificeFirstStillCreatesTokens(t *testing.T) {
	t.Parallel()
	e, cfg, _ := evokeAdamantRun(t, "Catharsis", "WW", true, 911)
	if got := tokensControlledBy(e, 0); got != 2 {
		t.Fatalf("tokens = %d, want 2", got)
	}
	if n := interveningIfFizzles(e); n != 0 {
		t.Fatalf("%d intervening-if fizzle events, want none", n)
	}
	replayCheck(t, e, cfg)
}

func TestEvokedCatharsisSacrificeLastStillCreatesTokens(t *testing.T) {
	t.Parallel()
	e, cfg, _ := evokeAdamantRun(t, "Catharsis", "WW", false, 911)
	if got := tokensControlledBy(e, 0); got != 2 {
		t.Fatalf("tokens = %d, want 2", got)
	}
	if n := interveningIfFizzles(e); n != 0 {
		t.Fatalf("%d intervening-if fizzle events, want none", n)
	}
	replayCheck(t, e, cfg)
}

func TestEvokedCatharsisWrongColourStillFizzlesAfterSacrifice(t *testing.T) {
	t.Parallel()
	e, cfg, _ := evokeAdamantRun(t, "Catharsis", "RR", true, 911)
	if got := tokensControlledBy(e, 0); got != 0 {
		t.Fatalf("tokens = %d, want 0 (only {R}{R} was spent)", got)
	}
	if n := interveningIfFizzles(e); n != 0 {
		t.Fatalf("%d fizzle events: the {W}{W} trigger must never fire, the {R}{R} one must resolve", n)
	}
	replayCheck(t, e, cfg)
}

func TestEvokedVibranceSacrificeFirstGainsLife(t *testing.T) {
	t.Parallel()
	e, cfg, _ := evokeAdamantRun(t, "Vibrance", "GG", true, 913)
	// Seat 1 never gains or loses life here, so it is the baseline.
	if got := e.G.Players[0].Life - e.G.Players[1].Life; got != 2 {
		t.Fatalf("seat 0 life %d vs seat 1 %d, want a gain of exactly 2", e.G.Players[0].Life, e.G.Players[1].Life)
	}
	replayCheck(t, e, cfg)
}
