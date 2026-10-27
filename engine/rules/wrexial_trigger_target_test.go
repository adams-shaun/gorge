package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Wrexial, the Risen Deep's combat-damage trigger is
// `DB$ Play | TgtZone$ Graveyard | ValidTgts$ Instant.OwnedBy
// Player.TriggeredTarget,Sorcery.OwnedBy Player.TriggeredTarget |
// WithoutManaCost$ True ...`. Player.TriggeredTarget is the damaged player's
// player-spec spelling; before cr-g242 the control/ownership referent
// classifier did not admit it, so the trigger offered no target and nothing
// was cast. This runs the real corpus script: the damaged player's Lava Spike
// is offered, cast for free (3 damage back to them), and exiled by the
// ReplaceGraveyard$ rider.

// driveWrexialTrigger walks the post-combat decisions through the optional
// trigger, the graveyard target, the free cast's own target and the priority
// passes until Lava Spike reaches exile.
func driveWrexialTrigger(t *testing.T, e *Engine, lava state.ObjID) {
	t.Helper()
	offered := false
	for i := 0; i < 100; i++ {
		if o := e.G.Obj(lava); o != nil && o.Zone == state.ZExile {
			return
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while driving the Wrexial trigger (Lava Spike in %v)", e.G.Obj(lava).Zone)
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KTriggerOptional:
			submitChoices(t, e, optionIndexOfKind(t, d, "yes"))
		case decision.KTarget:
			if !offered {
				// The trigger's own target ask: the damaged player's
				// graveyard must offer Lava Spike.
				idx := -1
				for _, o := range d.Options {
					if o.Obj == lava {
						idx = o.Index
						break
					}
				}
				if idx < 0 {
					t.Fatalf("Lava Spike (obj %d) not offered by the trigger's target ask: %+v", lava, d.Options)
				}
				offered = true
				submitChoices(t, e, idx)
				continue
			}
			// Lava Spike's own target ask: "target player or planeswalker".
			idx := -1
			for _, o := range d.Options {
				if o.Player == 1 && o.Obj == 0 {
					idx = o.Index
					break
				}
			}
			if idx < 0 {
				t.Fatalf("Lava Spike's target ask does not offer player 1: %+v", d.Options)
			}
			submitChoices(t, e, idx)
		case decision.KModes:
			submitChoices(t, e, 0)
		case decision.KChoose:
			// The Play SA's Optional$ ask (and any cast-mode pick) answered
			// with its first offered option.
			submitChoices(t, e, 0)
		default:
			t.Fatalf("unexpected decision %d while driving the Wrexial trigger: kind=%v resume=%q", i, d.Kind, d.ResumeKind)
		}
	}
	t.Fatalf("Lava Spike never reached exile (zone %v)", e.G.Obj(lava).Zone)
}

func TestWrexialCastsFromDamagedPlayersGraveyard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg,
		[]string{"Wrexial, the Risen Deep"}, nil,
		[]string{"Lava Spike"}, nil)
	wrexial := findBattlefield(t, e, 0, "Wrexial, the Risen Deep", 0)
	lava := findBattlefield(t, e, 1, "Lava Spike", 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: lava, From: state.ZBattlefield, To: state.ZGraveyard})

	// PRECONDITION: the instant is in the damaged player's (seat 1's)
	// graveyard, the zone "that player's graveyard" names.
	o := e.G.Obj(lava)
	if o == nil || o.Zone != state.ZGraveyard || o.Owner != 1 {
		t.Fatalf("precondition: Lava Spike = %+v, want seat 1's graveyard", o)
	}
	if o.Face() == nil || !o.Face().IsSorcery() {
		t.Fatalf("precondition: Lava Spike is not an instant or sorcery: %+v", o.Face())
	}

	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.askAttackers()
	submitAttackers(t, e, wrexial)

	driveWrexialTrigger(t, e, lava)

	// Wrexial's 5 combat damage then Lava Spike's 3 free damage.
	if got := e.G.Players[1].Life; got != 12 {
		t.Fatalf("damaged player's life = %d, want 12 (20 - 5 combat - 3 free Lava Spike)", got)
	}
	if o := e.G.Obj(lava); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Lava Spike rests in %v, want exile (ReplaceGraveyard$ Exile)", o)
	}
	replayCheck(t, e, cfg)
}
