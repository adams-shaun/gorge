package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 603.3d: a triggered ability's targets are chosen as the ability is put
// on the stack, not at resolution. A permanent's Support N is exactly such a
// trigger (Forge expands the keyword to an ETB trigger whose body is
// `DB$ PutCounter | Support$ N`), so "up to N other target creatures" must
// be announced at placement -- the same timing gorge already gives an
// instant/sorcery's Support at cast (CR 701.41a/601.2c).
const supportPermanentCarrierSrc = "Name:Support Permanent Carrier\nManaCost:2 W\nTypes:Creature Human Soldier\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigSupport | TriggerDescription$ When CARDNAME enters, support 2.\n" +
	"SVar:TrigSupport:DB$ PutCounter | Support$ 2\nOracle:x\n"

const supportPermanentBearSrc = "Name:Support Permanent Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// TestSupportPermanentTargetsAtTrigger enters a Support 2 creature with two
// other creatures already on the battlefield. The target decision must be
// pending before any player receives priority, while the trigger is still on
// the stack; the chosen creatures then receive the counters at resolution.
func TestSupportPermanentTargetsAtTrigger(t *testing.T) {
	t.Parallel()
	e, cfg, _ := newFixtureDeck(t, 7301, supportPermanentCarrierSrc,
		supportPermanentBearSrc, supportPermanentBearSrc)
	first := putCreature(t, e, 0, supportPermanentBearSrc)
	second := putCreature(t, e, 0, supportPermanentBearSrc)
	carrier := putCreature(t, e, 0, supportPermanentCarrierSrc)
	if e.G.Obj(carrier).Zone != state.ZBattlefield {
		t.Fatalf("precondition: carrier zone = %v, want battlefield", e.G.Obj(carrier).Zone)
	}
	for _, id := range []state.ObjID{first, second} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: support candidate %d is not on the battlefield", id)
		}
	}
	// The entry queued exactly the carrier's Support trigger; nothing has
	// been put on the stack or asked yet.
	if n := len(e.pendingTriggers); n != 1 {
		t.Fatalf("precondition: pending triggers = %d, want the carrier's one ETB trigger", n)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: stack = %v before placement", e.G.Stack)
	}
	if !e.putTriggersOnStack() {
		t.Fatal("putTriggersOnStack asked nothing: a permanent's Support targets were not chosen at trigger-put time (CR 603.3d)")
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v after placement, want the Support trigger", e.G.Stack)
	}
	abilityID := e.G.Stack[0]
	ability := e.G.Obj(abilityID)
	if ability == nil || ability.Ability == nil || ability.Source != carrier {
		t.Fatalf("top of stack = %+v, want the carrier's triggered ability", ability)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending decision = %+v, want the Support target ask at trigger-put time", d)
	}
	if d.Source != abilityID {
		t.Fatalf("target ask source = %d, want the trigger object %d", d.Source, abilityID)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("target ask bounds = %d..%d, want up to 2", d.Min, d.Max)
	}
	want := map[state.ObjID]bool{first: true, second: true}
	for _, o := range d.Options {
		if o.Obj == carrier {
			t.Fatalf("the carrier offered itself as a Support target: %+v", d.Options)
		}
		if !want[o.Obj] {
			t.Fatalf("unexpected Support target option %d: %+v", o.Obj, d.Options)
		}
	}
	if len(d.Options) != 2 {
		t.Fatalf("target options = %+v, want both other creatures and not the carrier", d.Options)
	}
	submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
	// Resolution must use the announced pair, not pose a second pick.
	e.resolveTop()
	if p := e.Pending(); p != nil && p.Kind == decision.KChoose {
		t.Fatalf("resolution posed a second creature pick: %+v", p)
	}
	for _, id := range []state.ObjID{first, second} {
		if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
			t.Fatalf("creature %d +1/+1 counters = %d, want 1", id, got)
		}
	}
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 0 {
		t.Fatalf("carrier +1/+1 counters = %d, want 0 (Support says other)", got)
	}
	replayCheck(t, e, cfg)
}

// TestSupportPermanentDeclinedTargetsAreNotReasked: "up to N" is a real
// choice, and a zero-target election is recorded at placement. Resolution
// must not fall back to the counter_pick question the pre-fix code asked,
// because that would let the player choose creatures after priority passed.
func TestSupportPermanentDeclinedTargetsAreNotReasked(t *testing.T) {
	t.Parallel()
	e, cfg, _ := newFixtureDeck(t, 7302, supportPermanentCarrierSrc,
		supportPermanentBearSrc, supportPermanentBearSrc)
	first := putCreature(t, e, 0, supportPermanentBearSrc)
	second := putCreature(t, e, 0, supportPermanentBearSrc)
	carrier := putCreature(t, e, 0, supportPermanentCarrierSrc)
	if e.G.Obj(carrier).Zone != state.ZBattlefield {
		t.Fatalf("precondition: carrier zone = %v, want battlefield", e.G.Obj(carrier).Zone)
	}
	if !e.putTriggersOnStack() {
		t.Fatal("putTriggersOnStack asked nothing at trigger-put time")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending decision = %+v, want the placement target ask", d)
	}
	submitChoices(t, e) // choose zero of up to two
	e.resolveTop()
	if p := e.Pending(); p != nil && p.Kind == decision.KChoose {
		t.Fatalf("resolution re-posed the Support pick after a zero-target answer: %+v", p)
	}
	for _, id := range []state.ObjID{first, second, carrier} {
		if got := e.G.Obj(id).Counter("P1P1"); got != 0 {
			t.Fatalf("creature %d +1/+1 counters = %d after declining, want 0", id, got)
		}
	}
	replayCheck(t, e, cfg)
}

// A permanent's ACTIVATED Support is the same targeting rule (CR 602.2b
// applies the casting process, 601.2c's target announcement included). This
// is the Joraga Auxiliary shape: `A:AB$ PutCounter | Cost$ ... | Support$ N`.
func TestSupportActivatedTargetsAtActivation(t *testing.T) {
	t.Parallel()
	const activatedCarrier = "Name:Support Activated Carrier\nManaCost:1 G\nTypes:Creature Elf Soldier\nPT:1/1\n" +
		"A:AB$ PutCounter | Cost$ G | Support$ 2\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 7303, activatedCarrier,
		supportPermanentBearSrc, supportPermanentBearSrc)
	first := putCreature(t, e, 0, supportPermanentBearSrc)
	second := putCreature(t, e, 0, supportPermanentBearSrc)
	carrier := putCreature(t, e, 0, activatedCarrier)
	if e.G.Obj(carrier).Zone != state.ZBattlefield {
		t.Fatalf("precondition: carrier zone = %v, want battlefield", e.G.Obj(carrier).Zone)
	}
	addMana(t, e, 0, "G")
	submitChoices(t, e, abilityOption(t, e, carrier, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("activation offered no Support target ask: %+v", d)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("target ask bounds = %d..%d, want up to 2", d.Min, d.Max)
	}
	seen := map[state.ObjID]bool{}
	for _, o := range d.Options {
		if o.Obj == carrier {
			t.Fatalf("the activated ability offered its own source: %+v", d.Options)
		}
		seen[o.Obj] = true
	}
	if !seen[first] || !seen[second] || len(d.Options) != 2 {
		t.Fatalf("target options = %+v, want both other creatures and not the source", d.Options)
	}
	submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
	e.resolveTop()
	if p := e.Pending(); p != nil && p.Kind == decision.KChoose {
		t.Fatalf("resolution re-posed the Support pick: %+v", p)
	}
	for _, id := range []state.ObjID{first, second} {
		if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
			t.Fatalf("creature %d +1/+1 counters = %d, want 1", id, got)
		}
	}
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 0 {
		t.Fatalf("carrier +1/+1 counters = %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}
