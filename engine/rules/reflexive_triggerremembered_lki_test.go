package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// These tests are the TriggerRemembered sibling of cr-g297's
// TestReflexiveSacrificedPowerUsesLKIOfTransformedFace: the reflexive body's
// X is `TriggerRemembered$CardPower`/`CardToughness`, a different referent
// spelling over the same sacrificed object. Eddie Brock transformed into
// Venom, Lethal Protector is a 5/5 permanent whose graveyard face is the
// printed 3/3, so CR 608.2h requires the reflexive body to read 5, not 3.

// driveReflexiveSacrifice answers every decision of a reflexive sacrifice
// until the stack is empty: the sacrifice pick names sac, the reflexive
// target (if any) names target (0 = none), and priority is passed.
func driveReflexiveSacrifice(t *testing.T, e *Engine, sac, target state.ObjID) {
	t.Helper()
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			return
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				return
			}
			passFirst(t, e)
			continue
		}
		switch d.Kind {
		case decision.KChoose:
			chooseObj(t, e, "sacrifice", sac)
		case decision.KTarget:
			if target == 0 {
				t.Fatalf("unexpected target ask: %+v", d.Options)
			}
			targetObject(t, e, target)
		case decision.KTriggerOptional:
			submitChoices(t, e, 0)
		case decision.KTriggerOrder:
			idx := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				idx = append(idx, o.Index)
			}
			submitChoices(t, e, idx...)
		default:
			t.Fatalf("unexpected decision %q: %+v", d.Kind, d.Options)
		}
	}
	t.Fatal("reflexive sacrifice never drained the stack")
}

// transformEddieToVenom asserts the DFC is on the battlefield with its 3/3
// front face and transforms it, asserting the 5/5 back face actually differs.
func transformEddieToVenom(t *testing.T, e *Engine, eddie state.ObjID) {
	t.Helper()
	if f := e.G.Obj(eddie).Face(); f == nil || f.Name != "Eddie Brock" || f.Power() != 3 || f.Toughness() != 3 {
		t.Fatalf("precondition: front face = %+v, want Eddie Brock at 3/3", e.G.Obj(eddie).Face())
	}
	e.emit(events.Event{Kind: events.FlipFace, Obj: eddie, Amount: 1, Text: "Transformed"})
	if f := e.G.Obj(eddie).Face(); f == nil || f.Name != "Venom, Lethal Protector" {
		t.Fatalf("precondition: transformed face = %+v, want Venom, Lethal Protector", e.G.Obj(eddie).Face())
	}
	if got := e.Power(eddie); got != 5 {
		t.Fatalf("precondition: transformed Venom power = %d, want 5", got)
	}
	if got := e.Toughness(eddie); got != 5 {
		t.Fatalf("precondition: transformed Venom toughness = %d, want 5", got)
	}
}

// TestReflexiveTriggerRememberedPowerUsesSacrificeLKI pins Minsc & Boo,
// Timeless Heroes' -2: "Sacrifice a creature. When you do, deals X damage to
// any target, X = that creature's power." With Venom sacrificed the damage
// must be 5; a graveyard-face read deals 3.
func TestReflexiveTriggerRememberedPowerUsesSacrificeLKI(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := dsBoard(t, reg, "Minsc & Boo, Timeless Heroes", "Eddie Brock", "Grizzly Bears")
	minsc := ids["Minsc & Boo, Timeless Heroes"]
	eddie := ids["Eddie Brock"]
	bears := ids["Grizzly Bears"]

	transformEddieToVenom(t, e, eddie)

	minscCard := mustCorpusCard(t, reg, "Minsc & Boo, Timeless Heroes")
	if got := e.G.Obj(minsc).Counter("LOYALTY"); got < 2 {
		t.Fatalf("precondition: Minsc loyalty = %d, want at least 2", got)
	}
	minusTwo := jaceAbility(t, minscCard, "SubCounter", 2)
	e.Advance()
	submitChoices(t, e, abilityOption(t, e, minsc, minusTwo).Index)
	driveReflexiveSacrifice(t, e, eddie, bears)

	if z := e.G.Obj(eddie).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: sacrificed Eddie zone = %s, want graveyard", z)
	}
	if f := e.G.Obj(eddie).Face(); f == nil || f.Power() != 3 {
		t.Fatalf("precondition: graveyard face = %+v, want the 3-power front face", e.G.Obj(eddie).Face())
	}

	damage := int32(-1)
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Obj == bears {
			damage = ev.Amount
		}
	}
	if damage != 5 {
		t.Fatalf("Minsc's reflexive damage = %d, want 5 (Venom's last-known power, CR 608.2h)", damage)
	}
	if z := e.G.Obj(bears).Zone; z != state.ZGraveyard {
		t.Fatalf("Grizzly Bears zone = %s, want graveyard after 5 lethal damage", z)
	}
	replayCheck(t, e, cfg)
}

// TestReflexiveTriggerRememberedToughnessUsesSacrificeLKI pins Tip the Scales'
// CardToughness sibling: the sacrificed Venom's last-known toughness is 5, so
// a 6/6 Colossal Dreadmaw becomes 1/1. A graveyard-face read gives -3/-3 and
// leaves it 3/3.
func TestReflexiveTriggerRememberedToughnessUsesSacrificeLKI(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := dsBoard(t, reg, "Tip the Scales", "Eddie Brock", "Colossal Dreadmaw")
	tip := ids["Tip the Scales"]
	eddie := ids["Eddie Brock"]
	dreadmaw := ids["Colossal Dreadmaw"]

	transformEddieToVenom(t, e, eddie)
	if got := e.Toughness(dreadmaw); got != 6 {
		t.Fatalf("precondition: Colossal Dreadmaw toughness = %d, want 6", got)
	}

	moveToHand(t, e, tip)
	addMana(t, e, 0, "CCB")
	castFirst(t, e, "cast")
	driveReflexiveSacrifice(t, e, eddie, 0)

	if z := e.G.Obj(eddie).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: sacrificed Eddie zone = %s, want graveyard", z)
	}
	if f := e.G.Obj(eddie).Face(); f == nil || f.Toughness() != 3 {
		t.Fatalf("precondition: graveyard face = %+v, want the 3-toughness front face", e.G.Obj(eddie).Face())
	}
	if got := e.Toughness(dreadmaw); got != 1 {
		t.Fatalf("Colossal Dreadmaw toughness after Tip the Scales = %d, want 1 (-5/-5 from Venom's last-known toughness, CR 608.2h)", got)
	}
	replayCheck(t, e, cfg)
}
