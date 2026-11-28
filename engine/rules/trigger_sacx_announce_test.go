package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A trigger's own announced Sac<X/Spec> cost (Sephiroth, One-Winged Angel's
// "you may sacrifice any number of other creatures. If you do, draw that many
// cards"; CR 601.2b, 118.3): the payer announces X over 0..the other creatures
// they control, sacrifices exactly X of them, and the body draws X. Before the
// fix the window posed decline only. Inline synthetic fixtures only.

const sacXDrawScript = "Name:SacX Draw\nManaCost:B\nTypes:Creature Human\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw\n" +
	"SVar:TrigDraw:AB$ Draw | NumCards$ X | Cost$ Sac<X/Creature.Other/another creature>\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

// sacXWindow puts two other creatures onto seat 0's battlefield, then the
// fixture, and returns the engine, the two creatures and the X ask.
func sacXWindow(t *testing.T) (*Engine, Config, [2]state.ObjID, *decision.Decision) {
	t.Helper()
	const bearA = "Name:SacX Bear A\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
	const bearB = "Name:SacX Bear B\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
	e, cfg, src := newFixtureDeck(t, 1, sacXDrawScript, bearA, bearB)
	e.pending = nil
	bears := [2]state.ObjID{
		namedCardInZones(t, e, 0, "SacX Bear A", state.ZHand, state.ZLibrary),
		namedCardInZones(t, e, 0, "SacX Bear B", state.ZHand, state.ZLibrary),
	}
	for _, id := range append(bears[:], src) {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield})
		}
	}
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the trigger-cost X ask, got %+v", d)
	}
	return e, cfg, bears, d
}

func TestTriggerSacXAnnouncesAndPays(t *testing.T) {
	t.Parallel()
	e, cfg, bears, d := sacXWindow(t)
	// Two other creatures: X = 0..2, ascending (option 0 the decline-equivalent).
	if len(d.Options) != 3 {
		t.Fatalf("X ask options = %+v, want X = 0..2", d.Options)
	}
	for i, o := range d.Options {
		if o.Kind != "trigger_cost_x" || o.Amount != i {
			t.Fatalf("option %d = %+v, want trigger_cost_x X=%d ascending", i, o, i)
		}
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, xOption(t, d, 2))
	d = e.Pending()
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("Sac<2> was not offered as payable: %+v", d.Options)
	}
	submitChoices(t, e, pay)
	// The settle walk asks for each sacrificed creature (or picks the exact
	// candidates itself); answer every pick with its first option.
	for i := 0; i < 4; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Player != 0 ||
			(d.Options[0].Obj != bears[0] && d.Options[0].Obj != bears[1]) {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	drainEvidenceXTriggers(t, e)
	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("creature %d in %s, want graveyard (sacrificed)", id, z)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 2 {
		t.Fatalf("cards drawn = %d, want 2", got)
	}
	replayCheck(t, e, cfg)
}

func TestTriggerSacXZeroIsDecline(t *testing.T) {
	t.Parallel()
	e, _, bears, d := sacXWindow(t)
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, xOption(t, d, 0))
	if d = e.Pending(); d != nil {
		if pay, decline := windowPayDecline(t, d); pay >= 0 {
			t.Fatalf("X=0 offered a pay option: %+v", d.Options)
		} else {
			submitChoices(t, e, decline)
		}
	}
	drainEvidenceXTriggers(t, e)
	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Fatalf("X=0 moved creature %d to %s", id, z)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 0 {
		t.Fatalf("X=0 drew %d cards", got)
	}
}
