package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestReflexiveSacrificedPowerUsesLKIOfTransformedFace is cr-g297: the
// reflexive half of Bolg of the North's ETB trigger sizes its damage with
// `Spawner>Sacrificed$CardPower`. The sacrificed creature is Eddie Brock
// transformed into Venom, Lethal Protector (a 5/5 permanent), so CR 608.2h
// requires the damage to be 5, the power it had when it was sacrificed. The
// front face Eddie Brock is a 3/3, and that is the face the card has in the
// graveyard; a read of the live object after the sacrifice deals 3 instead.
// The Goblin Army the excess amasses then carries three +1/+1 counters rather
// than one, which is the second assertion below. Heart-Piercer Manticore
// carries the same chained spelling; this is the class, not one card.
func TestReflexiveSacrificedPowerUsesLKIOfTransformedFace(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := dsBoard(t, reg, "Bolg of the North", "Eddie Brock", "Grizzly Bears")
	// dsBoard's hand-built Config does not carry the token scripts; the
	// symptom's Goblin Army needs them, and replayCheck rebuilds from cfg.
	e.G.Tokens = reg.Tokens
	cfg.Tokens = reg.Tokens
	bolg := ids["Bolg of the North"]
	eddie := ids["Eddie Brock"]
	bears := ids["Grizzly Bears"]

	// PRECONDITION: the DFC is on the battlefield wearing its front face, and
	// the two faces' powers differ -- otherwise a front-face read would pass
	// this test for the wrong reason.
	if f := e.G.Obj(eddie).Face(); f == nil || f.Name != "Eddie Brock" || f.Power() != 3 {
		t.Fatalf("precondition: front face = %+v, want Eddie Brock at power 3", e.G.Obj(eddie).Face())
	}
	e.emit(events.Event{Kind: events.FlipFace, Obj: eddie, Amount: 1, Text: "Transformed"})
	if f := e.G.Obj(eddie).Face(); f == nil || f.Name != "Venom, Lethal Protector" {
		t.Fatalf("precondition: transformed face = %+v, want Venom, Lethal Protector", e.G.Obj(eddie).Face())
	}
	if got := e.Power(eddie); got != 5 {
		t.Fatalf("precondition: transformed Venom power = %d, want 5", got)
	}

	// Re-enter Bolg so its real ETB trigger fires. dsBoard parks the named
	// permanents on the battlefield, so the entry has to be a logged move.
	moveToHand(t, e, bolg)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bolg, From: state.ZHand, To: state.ZBattlefield})
	e.pending = nil
	e.Advance()

	// Drive the ETB's triggered-cost window (pay the "you may sacrifice
	// another creature" cost) and the reflexive trigger's target ask. Eddie is
	// the only creature the cost can select, so the sacrifice itself is not
	// posed as a pick; the bears are the reflexive damage target.
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			passFirst(t, e)
			continue
		}
		switch d.Kind {
		case decision.KTarget:
			targetObject(t, e, bears)
		case decision.KTriggerOptional:
			submitChoices(t, e, 0)
		case decision.KTriggerOrder:
			idx := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				idx = append(idx, o.Index)
			}
			submitChoices(t, e, idx...)
		case decision.KChoose:
			switch {
			case len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_pay":
				submitChoices(t, e, 0)
			case len(d.Options) > 0 && d.Options[0].Kind == "sacrifice":
				chooseObj(t, e, "sacrifice", eddie)
			default:
				t.Fatalf("unexpected choose decision while resolving Bolg: %+v", d)
			}
		default:
			t.Fatalf("unexpected decision %q while resolving Bolg", d.Kind)
		}
	}
	passUntilStackEmpty(t, e, 30)

	if z := e.G.Obj(eddie).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: sacrificed Eddie zone = %s, want graveyard", z)
	}
	// The graveyard face is the front face again: this is exactly the value a
	// live read would use, so it distinguishes the LKI answer from that bug.
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
		t.Fatalf("Bolg's reflexive damage = %d, want 5 (Venom's last-known power, CR 608.2h)", damage)
	}
	if z := e.G.Obj(bears).Zone; z != state.ZGraveyard {
		t.Fatalf("Grizzly Bears zone = %s, want graveyard after 5 lethal damage", z)
	}
	army := state.ObjID(0)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Goblin Army Token" {
			army = id
		}
	}
	if army == 0 {
		t.Fatal("no Goblin Army token: the excess damage did not amass")
	}
	if got := e.G.Obj(army).Counter("P1P1"); got != 3 {
		t.Fatalf("Goblin Army +1/+1 counters = %d, want 3 (5 damage - 2 toughness)", got)
	}
	replayCheck(t, e, cfg)
}
