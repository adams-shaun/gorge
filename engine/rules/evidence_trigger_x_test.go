package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The announced-X half of a triggered collect-evidence cost (CR 701.59a with
// CR 601.2b's announce-before-pay ordering): Incinerator of the Guilty's
// `Cost$ CollectEvidence<X>` with `SVar:X:Count$xPaid`. Before this change the
// evidence part carried a dynamic amount the trigger window could not resolve,
// so it offered decline only and emitted the loud dynamic-amount Note; the
// controller could never collect evidence and the reflexive damage never ran.
// All fixtures are inline synthetic scripts plus the real corpus Incinerator;
// no Forge text is committed.

const evidenceXScript = "Name:Evidence X\nManaCost:2 U\nTypes:Creature Wizard\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCost\n" +
	"SVar:TrigCost:AB$ ImmediateTrigger | Cost$ CollectEvidence<X> | RememberSVarAmount$ X | Execute$ TrigGain\n" +
	"SVar:X:Count$xPaid\n" +
	"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ Count$TriggerRememberAmount\nOracle:x\n"

// evidenceXWindow seeds a graveyard of mana values 1 and 4 and opens the
// window; it returns the engine, its replay config, the two card ids and the X
// ask. The fixture cards travel from the deck through logged MoveZone events,
// so the returned config replays the whole scenario byte for byte.
func evidenceXWindow(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID, *decision.Decision) {
	t.Helper()
	const oneSrc = "Name:Evidence One\nManaCost:1\nTypes:Artifact\nOracle:x\n"
	const fourSrc = "Name:Evidence Four\nManaCost:4\nTypes:Artifact\nOracle:x\n"
	e, cfg, src := newFixtureDeck(t, 1, evidenceXScript, oneSrc, fourSrc)
	e.pending = nil
	one := namedCardInZones(t, e, 0, "Evidence One", state.ZHand, state.ZLibrary)
	four := namedCardInZones(t, e, 0, "Evidence Four", state.ZHand, state.ZLibrary)
	for _, id := range []state.ObjID{one, four} {
		o := e.G.Obj(id)
		if got := o.Zone; got != state.ZGraveyard {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: got, To: state.ZGraveyard})
		}
	}
	if o := e.G.Obj(src); o.Zone != state.ZHand {
		e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: o.Zone, To: state.ZHand})
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the trigger-cost X ask, got %+v", d)
	}
	return e, cfg, one, four, d
}

// namedCardInZones returns seat p's first card named name in one of zones.
func namedCardInZones(t *testing.T, e *Engine, p state.PlayerID, name string, zones ...state.Zone) state.ObjID {
	t.Helper()
	for _, z := range zones {
		for _, id := range e.G.Zone(z, p) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				return id
			}
		}
	}
	t.Fatalf("seat %d has no %q in %v", p, name, zones)
	return 0
}

// xOption returns the index of the trigger_cost_x option announcing x.
func xOption(t *testing.T, d *decision.Decision, x int) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == "trigger_cost_x" && o.Amount == x {
			return o.Index
		}
	}
	t.Fatalf("no trigger_cost_x option for X=%d: %+v", x, d.Options)
	return -1
}

// evidenceOption returns the index of the evidence option naming obj.
func evidenceOption(t *testing.T, d *decision.Decision, obj state.ObjID) int {
	t.Helper()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected an evidence ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "evidence" && o.Obj == obj {
			return o.Index
		}
	}
	t.Fatalf("evidence ask offers no card %d: %+v", obj, d.Options)
	return -1
}

// drainEvidenceXTriggers pushes every queued trigger (the ImmediateTrigger mints
// its reflexive body through the ordinary queue) and drains the stack. A
// settle resumes the spawning ability and leaves the reflexive half pending
// until priority is next granted, so a test that only drains the current
// stack would assert before the body ever runs.
func drainEvidenceXTriggers(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) > 0 {
			// putTriggersOnStack pushes without granting priority, so a fresh
			// trigger has no pending decision yet: resolve it directly, the
			// same drive the queue tests use. A suspended ask drains by
			// passing priority instead.
			if e.Pending() == nil {
				e.resolveTop()
				continue
			}
			passUntilStackEmpty(t, e, 30)
			continue
		}
		return
	}
	t.Fatal("queued triggers did not drain")
}

func TestTriggerCollectEvidenceXAnnouncesAndPays(t *testing.T) {
	t.Parallel()
	e, cfg, one, four, d := evidenceXWindow(t)
	// The graveyard's total mana value is 5, so the offered range is 0..5 in
	// ascending order (option 0 the decline-equivalent).
	if len(d.Options) != 6 {
		t.Fatalf("X ask options = %+v, want X = 0..5", d.Options)
	}
	for i, o := range d.Options {
		if o.Kind != "trigger_cost_x" || o.Amount != i {
			t.Fatalf("option %d = %+v, want trigger_cost_x X=%d ascending", i, o, i)
		}
	}
	mark := len(e.L.Events)
	life := e.G.Players[0].Life
	submitChoices(t, e, xOption(t, d, 4))

	// The folded cost re-opens the ordinary pay/decline ask; the evidence
	// stage then poses its mana-value-descending selection.
	d = e.Pending()
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("CollectEvidence<4> was not offered as payable: %+v", d.Options)
	}
	submitChoices(t, e, pay)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "evidence" {
		t.Fatalf("evidence ask = %+v", d)
	}
	submitChoices(t, e, evidenceOption(t, d, four))
	drainEvidenceXTriggers(t, e)

	if got := e.G.Players[0].Life - life; got != 4 {
		t.Fatalf("life gained = %d, want 4", got)
	}
	if e.G.Obj(four).Zone != state.ZExile || e.G.Obj(one).Zone != state.ZGraveyard {
		t.Fatalf("4-drop in %s, 1-drop in %s; want exile / graveyard", e.G.Obj(four).Zone, e.G.Obj(one).Zone)
	}
	if ids := evidenceEvents(e, mark); len(ids) != 1 || ids[0] != four {
		t.Fatalf("evidence events = %v, want just the 4-drop %d", ids, four)
	}
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "collect evidence amount is dynamic") {
			t.Fatalf("dynamic-amount Note emitted: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)
}

func TestTriggerCollectEvidenceXZeroIsDecline(t *testing.T) {
	t.Parallel()
	e, _, one, four, d := evidenceXWindow(t)
	mark := len(e.L.Events)
	life := e.G.Players[0].Life
	submitChoices(t, e, xOption(t, d, 0))
	// X = 0 folds to a zero threshold: the pay ask declines it (no "pay"
	// option) and the body never runs.
	if d = e.Pending(); d != nil {
		if pay, decline := windowPayDecline(t, d); pay >= 0 {
			t.Fatalf("X=0 offered a pay option: %+v", d.Options)
		} else {
			submitChoices(t, e, decline)
		}
	}
	drainEvidenceXTriggers(t, e)
	if got := e.G.Players[0].Life - life; got != 0 {
		t.Fatalf("life gained = %d, want 0", got)
	}
	if ids := evidenceEvents(e, mark); len(ids) != 0 {
		t.Fatalf("X=0 exiled %v", ids)
	}
	if e.G.Obj(one).Zone != state.ZGraveyard || e.G.Obj(four).Zone != state.ZGraveyard {
		t.Fatal("X=0 moved a graveyard card")
	}
}

// TestIncineratorOfTheGuiltyDealsX drives the real corpus Incinerator: a
// combat-damage hit on seat 1 announces X = 3, collects the mana-value-3
// graveyard card as evidence, and the reflexive body deals 3 to each creature
// seat 1 controls -- killing the 3-toughness one and leaving the 4-toughness
// one at 3 damage.
func TestIncineratorOfTheGuiltyDealsX(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := combatTriggerBoard(t, reg, []string{"Incinerator of the Guilty"}, nil, nil,
		[]string{
			"Name:Small Guy\nManaCost:2\nTypes:Creature Bear\nPT:2/3\nOracle:x\n",
			"Name:Big Guy\nManaCost:4\nTypes:Creature Bear\nPT:3/4\nOracle:x\n",
		})
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	inc := findBattlefield(t, e, 0, "Incinerator of the Guilty", 0)
	small := findBattlefield(t, e, 1, "Small Guy", 0)
	big := findBattlefield(t, e, 1, "Big Guy", 0)
	three := evidenceGraveCard(t, e, card(t, "Name:Evidence Three\nManaCost:3\nTypes:Artifact\nOracle:x\n"))
	if got := e.G.Obj(three).Zone; got != state.ZGraveyard {
		t.Fatalf("precondition: evidence card is in %s, want graveyard", got)
	}

	// The combat leg: e.damaging names the dealer and e.combatDamaging is set
	// for the emit, exactly as dealCombatDamage's assignment loop does.
	e.damaging = inc
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 6})
	e.combatDamaging = false
	e.damaging = 0
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the Incinerator X ask, got %+v", d)
	}
	submitChoices(t, e, xOption(t, d, 3))

	d = e.Pending()
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("CollectEvidence<3> was not offered as payable: %+v", d.Options)
	}
	submitChoices(t, e, pay)
	// The single mana-value-3 card is the only way to reach the threshold, so
	// the evidence stage records it without posing a choice.
	if d = e.Pending(); d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "evidence" {
		t.Fatalf("single-candidate evidence ask should settle without a choice: %+v", d)
	}
	drainEvidenceXTriggers(t, e)
	// The Damage events mark the lethal damage; the CR 704.5g state-based
	// action is what actually moves the 3-toughness creature off the board.
	e.pending = nil
	e.checkStateBased()

	if e.G.Obj(three).Zone != state.ZExile {
		t.Fatalf("evidence card in %s, want exile", e.G.Obj(three).Zone)
	}
	if e.G.Obj(small).Zone != state.ZGraveyard {
		t.Fatalf("3-toughness creature in %s, want graveyard", e.G.Obj(small).Zone)
	}
	if o := e.G.Obj(big); o.Zone != state.ZBattlefield || o.Damage != 3 {
		t.Fatalf("4-toughness creature zone=%s damage=%d, want battlefield / 3", o.Zone, o.Damage)
	}
}
