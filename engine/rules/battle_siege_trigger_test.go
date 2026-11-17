package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 310.12b: a Siege's defeat is its intrinsic triggered ability, "When the
// last defense counter is removed from this permanent, exile it, then you may
// cast it transformed without paying its mana cost." It goes on the stack with
// the other triggers the same combat damage made (ordered with them, CR
// 603.3b), the Siege stays on the battlefield at 0 defense while that ability
// is pending (CR 704.5v), and the cast ask comes only when it resolves.

// singerSrc is a creature whose own combat-damage trigger the Siege's trigger
// must be ordered with.
const singerSrc = "Name:Singer\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n" +
	"T:Mode$ DamageDone | ValidSource$ Card.Self | CombatDamage$ True | Execute$ TrigGain | TriggerDescription$ gain 1\n" +
	"SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n"

func TestSiegeDefeatIsAStackTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, id := battleBoard(t, reg, "Invasion of Tolvada")
	protector := e.G.Obj(id).Protector
	big := onBoardReady(t, e, 0, "Name:Brute\nManaCost:3 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n")
	singer := onBoardReady(t, e, 0, singerSrc)
	if got := e.G.Obj(id).Counter("DEFENSE"); got != 5 {
		t.Fatalf("precondition: battle defense %d, want 5 (3 + 2 is lethal)", got)
	}
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("no declare-attackers decision posed, got %+v", d)
	}
	var picks []int
	for _, atk := range []state.ObjID{big, singer} {
		found := false
		for i := range d.Options {
			if d.Options[i].Battle == id && d.Options[i].Obj == atk {
				picks = append(picks, d.Options[i].Index)
				found = true
			}
		}
		if !found {
			t.Fatalf("precondition: attacker %d has no attack option at the battle (protector %d); options=%+v", atk, protector, d.Options)
		}
	}
	submitChoices(t, e, picks...)

	e.dealCombatDamage()
	e.checkStateBased()

	// CR 704.5v: at 0 defense, but the source of a triggered ability that has
	// not left the stack: still on the battlefield, not exiled or binned.
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.Counter("DEFENSE") != 0 {
		t.Fatalf("lethally damaged Siege is in %v with %d defense, want the battlefield at 0 until its trigger resolves", o.Zone, o.Counter("DEFENSE"))
	}
	var sieges, singers int
	for _, pt := range e.pendingTriggers {
		switch {
		case pt.Siege && pt.Source == id:
			sieges++
		case pt.Source == singer:
			singers++
		}
	}
	if sieges != 1 || singers != 1 {
		t.Fatalf("pending triggers: %d Siege defeat, %d Singer damage, want one of each (%+v)", sieges, singers, e.pendingTriggers)
	}

	// Both triggers are one controller's: they are put on the stack together,
	// behind one CR 603.3b ordering ask that names both.
	e.putTriggersOnStack()
	d = e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder || len(d.Options) != 2 {
		t.Fatalf("want a two-trigger ordering ask, got %+v", d)
	}
	labels := d.Options[0].Label + "|" + d.Options[1].Label
	if !strings.Contains(labels, "exile it, then cast it transformed") || !strings.Contains(labels, "Singer") {
		t.Fatalf("the ordering ask names %q, want the Siege defeat beside Singer's trigger", labels)
	}
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("the Siege left the battlefield (%v) while its trigger waited to be ordered", o.Zone)
	}
	if hasEvent(e, events.MoveZone, id) && e.G.Obj(id).Zone == state.ZExile {
		t.Fatal("the Siege was exiled before its trigger resolved")
	}
	// Whichever the owner orders first resolves first; the Siege's trigger
	// last would be the reverse. Put it on top: its exile and its ask come
	// before anything else of the pair resolves.
	siegeIdx := 0
	if !strings.Contains(d.Options[0].Label, "exile it, then cast it transformed") {
		siegeIdx = 1
	}
	order := []int{siegeIdx, 1 - siegeIdx}
	submitChoices(t, e, order...)
	for i := 0; i < 12; i++ {
		d = e.Pending()
		if d == nil || d.Kind == decision.KChoose {
			break
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %v while resolving the pair", d.Kind)
		}
		passPriority(t, e)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 || d.Options[0].Kind != "defeat_cast_yes" {
		t.Fatalf("the cast ask was not posed when the Siege trigger resolved, got %+v", d)
	}
	if got := e.G.Obj(id).Zone; got != state.ZExile {
		t.Fatalf("the Siege is in %v at the ask, want exile (the trigger exiles it, then asks)", got)
	}
	submitChoices(t, e, 1) // decline: it stays in exile
	if got := e.G.Obj(id); got.Zone != state.ZExile || got.FaceIdx != 0 {
		t.Fatalf("declined Siege is in %v face %d, want exile, front face", got.Zone, got.FaceIdx)
	}
}

// TestCounteredSiegeDefeatTriggerBinsTheBattle: CR 704.5v spares a Siege only
// while its trigger has not left the stack. A trigger that left without doing
// its exile (countered, Stifled) leaves a battle with 0 defense, which the
// state-based action puts into its owner's graveyard.
func TestCounteredSiegeDefeatTriggerBinsTheBattle(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, id := battleBoard(t, reg, "Invasion of Tolvada")
	n := e.G.Obj(id).Counter("DEFENSE")
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "DEFENSE", Amount: -n})
	e.checkStateBased()
	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition: Siege is in %v with its trigger pending, want the battlefield", got)
	}
	e.putTriggersOnStack()
	var trig state.ObjID
	for _, sid := range e.G.Stack {
		if so := e.G.Obj(sid); so != nil && so.Ability != nil && so.Source == id {
			trig = sid
		}
	}
	if trig == 0 {
		t.Fatalf("precondition: no Siege defeat ability on the stack (%v)", e.G.Stack)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: trig, From: state.ZStack, To: state.ZExile, Text: "countered"})
	e.checkStateBased()
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("Siege at 0 defense with its trigger gone is in %v, want its owner's graveyard (CR 704.5v)", got)
	}
}

// TestAttackersDealNothingToTheRecastSiege: the Siege defeated in the
// first-strike damage step is exiled and cast transformed while combat goes
// on, and it re-enters under the same object id as a nonbattle permanent. The
// creatures that were attacking the battle are not attacking that new object
// (CR 400.7), so their regular combat damage has no recipient.
func TestAttackersDealNothingToTheRecastSiege(t *testing.T) {
	t.Parallel()
	e, id, atk, _ := battleAttackBoard(t)
	e.askAttackers()
	d := e.Pending()
	opt := battleDefenderOption(d, id)
	if opt == nil {
		t.Fatalf("precondition: the battle was not offered; options=%+v", d.Options)
	}
	submitChoices(t, e, opt.Index)
	a := e.G.Obj(atk)
	if _, b, ok := e.combatDefenderTarget(a); !ok || b != id {
		t.Fatalf("precondition: the attacker's recipient is (%d, %v), want the battle %d", b, ok, id)
	}
	e.G.Obj(id).SetFaceIdx(1) // the same object id, now the transformed back face
	if _, b, ok := e.combatDefenderTarget(a); ok {
		t.Fatalf("the attacker still has recipient %d after the battle became its back face", b)
	}
}
