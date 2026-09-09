package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestMogisUpkeepTriggerMatchesOpponentOnly(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	mogis, ok := reg.Lookup("Mogis, God of Slaughter")
	if !ok {
		t.Fatal("corpus fixture Mogis, God of Slaughter is missing")
	}

	for _, tc := range []struct {
		name   string
		active state.PlayerID
		want   int
	}{
		{name: "controller", active: 0, want: 0},
		{name: "opponent", active: 1, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := layerEngine(t)
			o := e.G.AddObject(mogis, 0)
			e.G.SetZone(state.ZLibrary, 0, append(e.G.Zone(state.ZLibrary, 0), o.ID))
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})

			triggers := o.Face().Triggers
			if len(triggers) != 1 || triggers[0].Mode != "Phase" || triggers[0].Params["ValidPlayer"] != "Player.Opponent" {
				t.Fatalf("Mogis trigger fixture changed: %+v", triggers)
			}

			e.G.Active = tc.active
			e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})

			got := 0
			for _, pending := range e.pendingTriggers {
				if pending.Source == o.ID {
					got++
				}
			}
			if got != tc.want {
				t.Fatalf("Mogis triggers during active player %d's upkeep = %d, want %d", tc.active, got, tc.want)
			}
		})
	}
}
