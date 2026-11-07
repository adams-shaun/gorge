package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 107.3m, 603.3c: a modal enters trigger whose mode count is X ("choose up
// to X", CharmNum$ X with SVar:X:Count$xPaid) reads the X chosen for the spell
// that became the permanent. When the legend rule (CR 704.5j) bins the newer
// copy before its trigger is put on the stack, events.Move has cleared the
// live X, so the mode bounds must come from the fire-time TriggerPaidX, not
// read 0 and remove the trigger as "no mode chosen".
//
// The fixture is an inline synthetic script shaped like The Ruinous Wrecking
// Crew (X B R, choose up to X); no Forge script text is committed.

const charmXFixtureSrc = "Name:Charm X Fixture\nManaCost:X B R\nTypes:Legendary Creature Human\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Execute$ ModalEntry | Destination$ Battlefield | Origin$ Any | ValidCard$ Card.Self\n" +
	"SVar:ModalEntry:DB$ Charm | Choices$ FxGain,FxDraw | CharmNum$ X | MinCharmNum$ 0\n" +
	"SVar:FxGain:DB$ GainLife | LifeAmount$ 3 | Defined$ You | SpellDescription$ gain\n" +
	"SVar:FxDraw:DB$ Draw | NumCards$ 1 | Defined$ You | SpellDescription$ draw\n" +
	"SVar:X:Count$xPaid\n" +
	"Oracle:x\n"

func TestEtbCharmXCountSurvivesLegendRule(t *testing.T) {
	t.Parallel()
	e, _, find := etbConfig(t, 233, []string{charmXFixtureSrc, charmXFixtureSrc}, nil)
	old := find("Charm X Fixture", 0)
	if old == 0 {
		t.Fatal("fixture: first Charm X Fixture not found")
	}
	var young state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Charm X Fixture" && id != old {
				young = id
			}
		}
	}
	if young == 0 {
		t.Fatal("fixture: second Charm X Fixture not found")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: old, From: e.G.Obj(old).Zone, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: young, From: e.G.Obj(young).Zone, To: state.ZHand})
	e.pending = nil
	// The older copy entered uncast: X=0, "choose up to zero", removed.
	if e.putTriggersOnStack() {
		t.Fatalf("the older copy's X=0 charm trigger posed an ask: %+v", e.Pending())
	}
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}

	addMana(t, e, 0, "BR"+strings.Repeat("C", 2)) // {X}{B}{R}, X=2
	castFirst(t, e, "cast")
	if d := e.Pending(); d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("expected the cast's {X} announce ask, got %+v", d)
	}
	submitChoices(t, e, 2)
	legend := passToLegendAsk(t, e)
	submitKeep(t, e, legend, keepOption(t, legend, old))
	if o := e.G.Obj(young); o.Zone != state.ZGraveyard || o.X != 0 {
		t.Fatalf("precondition: binned newer copy zone %v live X %d, want graveyard and 0", o.Zone, o.X)
	}

	d := e.Pending()
	if d == nil {
		t.Fatalf("no decision after the legend rule binned the trigger's source (stack %v)", e.G.Stack)
	}
	if d.Kind != decision.KModes {
		t.Fatalf("pending %v, want the mode ask (trigger removed?); stack %v", d.Kind, e.G.Stack)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack %v, want the charm trigger alone", e.G.Stack)
	}
	if got := e.triggerContexts[e.G.Stack[0]].TriggerPaidX; got != 2 {
		t.Fatalf("precondition: trigger context paid X = %d, want the captured 2", got)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("mode ask Min/Max = %d/%d, want 0/2 (the cast's X)", d.Min, d.Max)
	}
}
