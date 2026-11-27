package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Mindleech Mass (Valid$ Card.ControlledBy TriggeredTarget) and Silent-Blade
// Oni (Valid$ Card.nonLand+OwnedBy TriggeredTarget) deal combat damage to a
// player; the trigger looks at that player's hand and its Play must offer
// that hand's spells. The hand walk widens to the trigger's damaged seat only.
func TestPlayTriggeredPlayerHandOffersDamagedSeat(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, carrier := range []string{"Mindleech Mass", "Silent-Blade Oni"} {
		t.Run(carrier, func(t *testing.T) {
			t.Parallel()
			e, _ := combatTriggerBoard(t, reg, []string{carrier}, nil,
				[]string{"Lightning Bolt", "Grizzly Bears"}, nil)
			e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
			e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
			attacker := findBattlefield(t, e, 0, carrier, 0)
			want := map[state.ObjID]bool{}
			for _, name := range []string{"Lightning Bolt", "Grizzly Bears"} {
				id := findBattlefield(t, e, 1, name, 0)
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZHand})
				if o := e.G.Obj(id); o.Zone != state.ZHand || o.Owner != 1 {
					t.Fatalf("precondition: %s = %+v, want seat 1's hand", name, o)
				}
				want[id] = true
			}
			startLife := e.G.Players[1].Life

			e.askAttackers()
			submitAttackersOnly(t, e, attacker)
			ask := driveToPlayAsk(t, e)
			if e.G.Players[1].Life >= startLife {
				t.Fatalf("precondition: defender life %d, want combat damage from %s", e.G.Players[1].Life, carrier)
			}
			if ask == nil {
				t.Fatalf("%s: no play ask after combat damage; events tail=%+v", carrier, tailNotes(e))
			}
			if ask.Player != 0 {
				t.Fatalf("play ask posed to seat %d, want the trigger's controller 0", ask.Player)
			}
			got := map[state.ObjID]bool{}
			for _, o := range ask.Options {
				if o.Obj == 0 {
					continue
				}
				if !want[o.Obj] {
					t.Fatalf("play option %+v is not a damaged-hand card %v", o, want)
				}
				got[o.Obj] = true
			}
			if len(got) != len(want) {
				t.Fatalf("play options %+v, want both damaged-hand cards %v", ask.Options, want)
			}
		})
	}
}

// driveToPlayAsk answers the trigger's path (passes priority, accepts the
// optional trigger, acknowledges the look) until the Play ask poses.
func driveToPlayAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			return nil
		}
		if d.ResumeKind == "play" {
			return d
		}
		if e.G.Step > state.StepEndCombat && len(e.G.Stack) == 0 {
			return nil
		}
		t.Logf("decision %v resume=%q", d.Kind, d.ResumeKind)
		idx := -1
		for _, o := range d.Options {
			if d.Kind == decision.KPriority && o.Kind == "pass" {
				idx = o.Index
				break
			}
			if d.Kind != decision.KPriority && (o.Kind == "yes" || o.Kind == "ok" || o.Kind == "accept") {
				idx = o.Index
				break
			}
		}
		if idx < 0 && len(d.Options) > 0 {
			idx = d.Options[0].Index
		}
		if idx < 0 {
			submitChoices(t, e)
		} else {
			submitChoices(t, e, idx)
		}
	}
	return nil
}

func tailNotes(e *Engine) []string {
	var out []string
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note {
			out = append(out, ev.Text)
		}
	}
	if len(out) > 8 {
		out = out[len(out)-8:]
	}
	return out
}
