package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 107.3m: an enters-the-battlefield trigger that refers to X reads the X
// chosen for the spell that became the permanent, even though the permanent's
// own X is 0 (and events.Move clears its live X the moment it leaves the
// battlefield). CR 603.3d requires the trigger's targets to be chosen as it is
// put on the stack. When a second copy of a legend enters and the legend rule
// (CR 704.5j) bins it, the trigger is pushed after that departure, so a
// placement-time bound read of the live permanent sees 0 and poses no target
// ask at all. The captured TriggerPaidX must drive the bound instead.
//
// The fixture is an inline synthetic script shaped like The Rollercrusher Ride
// (X 2 R, "deals X damage to each of up to X target creatures"); no Forge
// script text is committed.

const rollercrusherFixtureSrc = "Name:Rollercrusher Fixture\nManaCost:X 2 R\nTypes:Legendary Enchantment\n" +
	"T:Mode$ ChangesZone | ValidCard$ Card.Self | Origin$ Any | Destination$ Battlefield | Execute$ TrigDealDamage | TriggerDescription$ When CARDNAME enters, it deals X damage to each of up to X target creatures.\n" +
	"SVar:TrigDealDamage:DB$ DealDamage | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ X | NumDmg$ X\n" +
	"SVar:X:Count$xPaid\n" +
	"Oracle:x\n"

const rollercrusherSpongeSrc = "Name:Rollercrusher Sponge\nTypes:Creature Sponge\nPT:0/6\nOracle:x\n"

// rollercrusherPair returns the first copy (the one this test puts on the
// battlefield first) and the second copy still in the library.
func rollercrusherPair(t *testing.T, e *Engine, find func(string, state.PlayerID) state.ObjID) (state.ObjID, state.ObjID) {
	t.Helper()
	old := find("Rollercrusher Fixture", 0)
	if old == 0 {
		t.Fatal("fixture: first Rollercrusher Fixture not found")
	}
	var young state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == "Rollercrusher Fixture" && id != old {
				young = id
				break
			}
		}
		if young != 0 {
			break
		}
	}
	if young == 0 {
		t.Fatal("fixture: second Rollercrusher Fixture not found")
	}
	return old, young
}

// passToLegendAsk submits pass priority until the CR 704.5j keep ask is
// pending, failing on any other non-priority decision.
func passToLegendAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while waiting for the legend-rule keep ask (stack %v)", e.G.Stack)
		}
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "keep" {
			return d
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("non-priority decision %v while waiting for the legend-rule keep ask: %+v", d.Kind, d)
		}
		submitChoicePass(t, e)
	}
	t.Fatal("the legend-rule keep ask never became pending")
	return nil
}

// keepOption returns the legend keep option naming id.
func keepOption(t *testing.T, d *decision.Decision, id state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == "keep" && o.Obj == id {
			return o.Index
		}
	}
	t.Fatalf("legend keep ask %+v offers no option for obj %d", d.Options, id)
	return -1
}

// castRollercrusherForX casts the named hand copy and answers its {X}.
func castRollercrusherForX(t *testing.T, e *Engine, x int) {
	t.Helper()
	addMana(t, e, 0, "RCCCC") // {X}{2}{R} with X=2
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("expected the cast's {X} announce ask, got %+v", d)
	}
	submitChoices(t, e, x)
}

// TestEtbTriggerXBoundSurvivesLegendRule is the ticket's headline: cast the
// second copy with X=2, keep the OLDER copy at the legend ask, and the ETB
// trigger must still ask for up to two targets and deal 2 to each.
func TestEtbTriggerXBoundSurvivesLegendRule(t *testing.T) {
	t.Parallel()

	t.Run("second_copy_binned_by_legend_rule", func(t *testing.T) {
		e, _, find := etbConfig(t, 231, []string{
			rollercrusherFixtureSrc, rollercrusherFixtureSrc,
			rollercrusherSpongeSrc, rollercrusherSpongeSrc,
		}, nil)
		old, young := rollercrusherPair(t, e, find)
		sponge1 := putCreature(t, e, 0, rollercrusherSpongeSrc)
		sponge2 := putCreature(t, e, 0, rollercrusherSpongeSrc)
		if e.G.Obj(sponge1).Damage != 0 || e.G.Obj(sponge2).Damage != 0 {
			t.Fatalf("precondition: sponges already marked (%d, %d)",
				e.G.Obj(sponge1).Damage, e.G.Obj(sponge2).Damage)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: old,
			From: e.G.Obj(old).Zone, To: state.ZBattlefield})
		e.emit(events.Event{Kind: events.MoveZone, Obj: young,
			From: e.G.Obj(young).Zone, To: state.ZHand})
		e.pending = nil
		// The older copy entered without being cast, so its own ETB trigger
		// reads X=0 and asks for nothing. Drain it before the real cast.
		if e.putTriggersOnStack() {
			t.Fatalf("the older copy's X=0 ETB trigger posed an ask: %+v", e.Pending())
		}
		for len(e.G.Stack) > 0 {
			e.resolveTop()
		}

		castRollercrusherForX(t, e, 2)
		legend := passToLegendAsk(t, e)
		if got := e.G.Obj(old).Zone; got != state.ZBattlefield {
			t.Fatalf("precondition: older copy not on the battlefield (zone %v)", got)
		}
		if got := e.G.Obj(young).Zone; got != state.ZStack && got != state.ZBattlefield {
			t.Fatalf("precondition: newer copy zone %v, want stack or battlefield", got)
		}
		submitKeep(t, e, legend, keepOption(t, legend, old))
		if o := e.G.Obj(young); o.Zone != state.ZGraveyard {
			t.Fatalf("the unchosen newer copy left to %v, want the graveyard", o.Zone)
		}

		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("no target ask after the legend rule binned the trigger's source: %+v", d)
		}
		if d.Max != 2 || d.Min != 0 {
			t.Fatalf("target ask Min/Max = %d/%d, want 0/2 (the ability's X)", d.Min, d.Max)
		}
		// The ask must offer both sponges and nothing else.
		idx := make([]int, 0, 2)
		for _, o := range d.Options {
			if o.Obj == sponge1 || o.Obj == sponge2 {
				idx = append(idx, o.Index)
			}
		}
		if len(idx) != 2 {
			t.Fatalf("target ask offers %d sponge targets, want 2: %+v", len(idx), d.Options)
		}
		submitChoices(t, e, idx...)
		passUntilStackEmpty(t, e, 20)
		if got := e.G.Obj(sponge1).Damage; got != 2 {
			t.Fatalf("first sponge has %d damage, want 2 (X=2)", got)
		}
		if got := e.G.Obj(sponge2).Damage; got != 2 {
			t.Fatalf("second sponge has %d damage, want 2 (X=2)", got)
		}
	})

	// Control: with no legend-rule departure the live permanent still carries
	// its cast X, so a single copy asks for up to 2 exactly as before.
	t.Run("single_copy_control", func(t *testing.T) {
		e, _, _ := etbConfig(t, 232, []string{
			rollercrusherFixtureSrc, rollercrusherSpongeSrc, rollercrusherSpongeSrc,
		}, nil)
		sponge1 := putCreature(t, e, 0, rollercrusherSpongeSrc)
		sponge2 := putCreature(t, e, 0, rollercrusherSpongeSrc)
		castRollercrusherForX(t, e, 2)
		var d *decision.Decision
		for i := 0; i < 20; i++ {
			d = e.Pending()
			if d != nil && d.Kind == decision.KTarget {
				break
			}
			if d == nil || d.Kind != decision.KPriority {
				t.Fatalf("unexpected decision while the single copy resolved: %+v", d)
			}
			submitChoicePass(t, e)
		}
		if d == nil || d.Kind != decision.KTarget || d.Max != 2 {
			t.Fatalf("single copy target ask = %+v, want KTarget Max 2", d)
		}
		idx := make([]int, 0, 2)
		for _, o := range d.Options {
			if o.Obj == sponge1 || o.Obj == sponge2 {
				idx = append(idx, o.Index)
			}
		}
		if len(idx) != 2 {
			t.Fatalf("single copy ask offers %d sponge targets, want 2: %+v", len(idx), d.Options)
		}
		submitChoices(t, e, idx...)
		passUntilStackEmpty(t, e, 20)
		if e.G.Obj(sponge1).Damage != 2 || e.G.Obj(sponge2).Damage != 2 {
			t.Fatalf("single copy damage = %d/%d, want 2/2",
				e.G.Obj(sponge1).Damage, e.G.Obj(sponge2).Damage)
		}
	})
}
