package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCommanderOneCommanderTokenCease plays the one-commander duel (seed 5)
// of the partner fixture's decks in lockstep. A token dies in the same
// combat-damage step as a commander: gorge keeps it in the graveyard until
// the commander's CR 903.9a "put it into the command zone?" ask is answered,
// then ceases it in the next state-based pass (CR 704.5d); the vanilla
// kernel path must do the same. The precondition check reads gorge's event
// log, so the test fails rather than passing vacuously if seed 5 stops
// putting a token and a commander into the graveyard together.
func TestCommanderOneCommanderTokenCease(t *testing.T) {
	m, tbl := partnerDuel(t)
	m.Partners = [2]int{-1, -1}
	rc := RunConfig{Config: Config{PassElision: true}, MaxTurns: 60}
	lead, err := NewGorgeSide(m, 5, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	mk := RealKernel(tbl, nil, rc.Config)
	k, err := mk(m, 5)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, 5, lead, k, RandomPolicy(5), rc)
	CloseSide(k)
	if res.Div != nil {
		t.Fatalf("seed 5: diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if !tokenCeasedAfterCommanderAsk(lead.E) {
		t.Fatal("seed 5 never put a token in the graveyard beside a commander's CR 903.9a ask, so the lockstep is vacuous")
	}
}

// tokenCeasedAfterCommanderAsk reports whether e's log holds a token that
// fell to the graveyard, then a commander's move from the graveyard to the
// command zone (the CR 903.9a ask answered), then that token ceasing out of
// the graveyard (CR 704.5d in the pass after the ask).
func tokenCeasedAfterCommanderAsk(e *rules.Engine) bool {
	tokens := map[state.ObjID]bool{}
	asked := false
	for _, ev := range e.L.Events {
		if ev.Kind != events.MoveZone {
			continue
		}
		switch {
		case ev.From == state.ZBattlefield && ev.To == state.ZGraveyard:
			if o := e.G.Obj(ev.Obj); o != nil && o.IsToken {
				tokens[ev.Obj] = true
			}
		case ev.From == state.ZGraveyard && ev.To == state.ZCommand:
			asked = true
		case asked && tokens[ev.Obj] && events.IsCease(ev):
			return true
		}
	}
	return false
}
