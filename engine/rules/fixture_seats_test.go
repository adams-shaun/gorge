package rules

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// fixtureSeatsSrc is a plain vanilla creature: the seat-count fixture tests
// care where the card lands, not what it does.
const fixtureSeatsSrc = "Name:Seat Fixture Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// TestFixtureDeckSeatsMatchesTwoSeatFixture pins the seat-count form at two
// seats to the existing fixture: the same seed builds the same game event for
// event, under the same seat names.
func TestFixtureDeckSeatsMatchesTwoSeatFixture(t *testing.T) {
	t.Parallel()
	e2, cfg2, id2 := newFixtureDeck(t, 7, fixtureSeatsSrc)
	eN, cfgN, idN := newFixtureDeckSeats(t, 7, 2, fixtureSeatsSrc)
	if len(e2.L.Events) == 0 {
		t.Fatalf("the two-seat fixture logged no events")
	}
	if !reflect.DeepEqual(e2.L.Events, eN.L.Events) {
		t.Fatalf("event logs differ: newFixtureDeck %d events, newFixtureDeckSeats(2) %d events",
			len(e2.L.Events), len(eN.L.Events))
	}
	if !reflect.DeepEqual(cfg2.Names, cfgN.Names) {
		t.Fatalf("Names = %v, want %v", cfgN.Names, cfg2.Names)
	}
	if cfg2.Seed != cfgN.Seed || id2 != idN {
		t.Fatalf("seed/fixture = %d/%d, want %d/%d", cfgN.Seed, idN, cfg2.Seed, id2)
	}
	if cfgN.Tokens == nil {
		t.Fatalf("Config.Tokens is nil, want the non-nil map the two-seat fixture carries")
	}
}

// TestFixtureDeckSeatsBuildsFourSeats checks the four-seat board: seat 0
// holds the fixture in hand and every other seat owns a 40-card Mountain deck.
func TestFixtureDeckSeatsBuildsFourSeats(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeckSeats(t, 7, 4, fixtureSeatsSrc)
	if got := len(e.G.Players); got != 4 {
		t.Fatalf("players = %d, want 4", got)
	}
	if want := []string{"a", "b", "c", "d"}; !reflect.DeepEqual(cfg.Names, want) {
		t.Fatalf("Names = %v, want %v", cfg.Names, want)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand || o.Owner != 0 {
		t.Fatalf("fixture %d is not in seat 0's hand: %+v", id, o)
	}
	for p := state.PlayerID(1); p < 4; p++ {
		owned := 0
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand} {
			for _, oid := range e.G.Zone(z, p) {
				if name := e.G.Obj(oid).Face().Name; name != "Mountain" {
					t.Fatalf("seat %d owns %q, want only Mountains", p, name)
				}
				owned++
			}
		}
		if owned != 40 {
			t.Fatalf("seat %d owns %d Mountains, want 40", p, owned)
		}
	}
	replayCheck(t, e, cfg)
}

// TestCommanderTaxGameSeatsBuildsAPod checks the four-seat Commander pod:
// one commander per command zone, Commander life, seat 0's turn 1 main 1.
func TestCommanderTaxGameSeatsBuildsAPod(t *testing.T) {
	t.Parallel()
	e, _, cmd := commanderTaxGameSeats(t, 3, 4)
	if got := len(e.G.Players); got != 4 {
		t.Fatalf("players = %d, want 4", got)
	}
	for p := state.PlayerID(0); p < 4; p++ {
		if got := len(e.G.Zone(state.ZCommand, p)); got != 1 {
			t.Fatalf("seat %d command zone holds %d, want 1", p, got)
		}
		if got := e.G.Players[p].Life; got != 40 {
			t.Fatalf("seat %d life = %d, want 40", p, got)
		}
	}
	if cmd != e.G.Players[0].Commanders[0] {
		t.Fatalf("returned commander %d, want seat 0's %d", cmd, e.G.Players[0].Commanders[0])
	}
	if e.G.Turn != 1 || e.G.Step != state.StepMain1 || e.G.Active != 0 || e.G.Priority != 0 {
		t.Fatalf("turn %d step %v active %d priority %d, want turn 1 main 1 with seat 0 active and holding priority",
			e.G.Turn, e.G.Step, e.G.Active, e.G.Priority)
	}
}

// TestSwingAtHitsTheNamedSeat checks swingAt sends the damage at the named
// defender only: seat 2 takes 5 (life and commander damage), seats 1 and 3
// take nothing.
func TestSwingAtHitsTheNamedSeat(t *testing.T) {
	t.Parallel()
	e, _ := commanderGame(t, commanderDamageSeed, FormatCommander, 40,
		[][]string{{cmdCreature(5)}, {}, {}, {}})
	cmd := fieldCommander(t, e, 0, 0)
	swingAt(t, e, 0, 2, cmd)
	for p := state.PlayerID(1); p < 4; p++ {
		wantLife, wantCmd := int32(40), int32(0)
		if p == 2 {
			wantLife, wantCmd = 35, 5
		}
		pl := &e.G.Players[p]
		if pl.Life != wantLife {
			t.Fatalf("seat %d life = %d, want %d", p, pl.Life, wantLife)
		}
		if len(pl.CmdDamage) == 0 || pl.CmdDamage[0] != wantCmd {
			t.Fatalf("seat %d CmdDamage = %v, want %d from seat 0's commander", p, pl.CmdDamage, wantCmd)
		}
	}
}
