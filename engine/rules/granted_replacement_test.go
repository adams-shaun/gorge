package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The two DFT max-speed AddReplacementEffect$ grants, driven directly at the
// engine: a granted replacement must
// land on the ordinary replacement path (engine/rules/replacement_granted.go), with
// the granting permanent as its host and source. Each test asserts its own
// precondition (the host is on the battlefield the ActiveZones$ Battlefield
// body reads, the gate really holds, the compared values differ) and its
// gate-off control, so a grant that ignored its Condition$ MaxSpeed gate or
// never registered cannot pass.

// TestGrantedReplacementFarFortuneDamage: at max speed Far Fortune, End Boss'
// granted DamageDone replacement adds 1 to a source p0 controls dealing damage
// to an opponent; at speed 0 the same damage is unmodified.
func TestGrantedReplacementFarFortuneDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name  string
		speed int32
		want  int32
	}{
		{"gate on (max speed): damage plus 1", maxSpeed, 17},
		{"gate off (speed 0): printed damage", 0, 18},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := stealEngine(t, 811)
			ff := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Far Fortune, End Boss"))
			// Precondition: the ActiveZones$ Battlefield body reads this
			// permanent's zone, and the recipient starts at a distinct life.
			if got := e.G.Obj(ff).Zone; got != state.ZBattlefield {
				t.Fatalf("precondition: Far Fortune zone = %v, want battlefield", got)
			}
			if got := e.G.Players[1].Life; got != 20 {
				t.Fatalf("precondition: p1 life = %d, want 20", got)
			}
			if e.G.Players[0].Speed != 0 {
				t.Fatalf("precondition: p0 speed = %d, want 0", e.G.Players[0].Speed)
			}
			if tc.speed != 0 {
				e.emit(events.Event{Kind: events.SpeedChange, Player: 0, Amount: tc.speed})
				if got := e.G.Players[0].Speed; got != tc.speed {
					t.Fatalf("precondition: p0 speed = %d, want %d", got, tc.speed)
				}
			}
			// The damage source is p0's own Far Fortune (ValidSource$
			// Card.YouCtrl); the recipient is p1 (ValidTarget$ Player.Opponent).
			e.damaging = ff
			e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
			e.damaging = 0
			if got := e.G.Players[1].Life; got != tc.want {
				t.Fatalf("p1 life after a 2-damage Shock = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestGrantedReplacementVnwxtDraw: at max speed Vnwxt, Verbose Host' granted
// Draw replacement turns one draw into two; at speed 0 the same draw stays
// one.
func TestGrantedReplacementVnwxtDraw(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name  string
		speed int32
		want  int
	}{
		{"gate on (max speed): draw two instead", maxSpeed, 2},
		{"gate off (speed 0): the printed draw", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := stealEngine(t, 812)
			vw := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Vnwxt, Verbose Host"))
			if got := e.G.Obj(vw).Zone; got != state.ZBattlefield {
				t.Fatalf("precondition: Vnwxt zone = %v, want battlefield", got)
			}
			// A zero hand so the post-draw size is the draw count itself.
			setDrawHand(t, e, 0, 0)
			if got := len(e.G.Zone(state.ZHand, 0)); got != 0 {
				t.Fatalf("precondition: seat 0 hand = %d, want 0", got)
			}
			if e.G.Players[0].Speed != 0 {
				t.Fatalf("precondition: p0 speed = %d, want 0", e.G.Players[0].Speed)
			}
			if tc.speed != 0 {
				e.emit(events.Event{Kind: events.SpeedChange, Player: 0, Amount: tc.speed})
				if got := e.G.Players[0].Speed; got != tc.speed {
					t.Fatalf("precondition: p0 speed = %d, want %d", got, tc.speed)
				}
			}
			before := countDraw(e)
			emitDraw(t, e, 0)
			if got := countDraw(e) - before; got != tc.want {
				t.Fatalf("Draw events for one replaced draw = %d, want %d", got, tc.want)
			}
			if got := len(e.G.Zone(state.ZHand, 0)); got != tc.want {
				t.Fatalf("seat 0 hand after the draw = %d, want %d", got, tc.want)
			}
		})
	}
}
