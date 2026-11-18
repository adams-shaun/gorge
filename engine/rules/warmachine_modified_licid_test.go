package rules

// The end-to-end real-card path behind the gorge-vs-dirac lockstep
// divergence at `projection Cards[50].Chars.Keywords` (pair 41 of the
// tail-63-c000 evidence loop, land ae57987d13): War Machine, Avenging
// Arsenal attacks and its trigger
//
//	DB$ PumpAll | ValidCards$ Creature.attacking+modified+YouCtrl |
//	KW$ Double Strike
//
// must reach a fellow attacker enchanted by a Convulsing Licid that its own
// Animate has turned into an Aura. CR 700.9 counts an Aura the permanent's
// controller controls as a modification, and CR 613.1d puts the Licid's
// Aura-ness on the LAYER-4 type line -- its printed face is still "Creature
// Licid" -- so the filter must read the published derived types, not the
// face. gorge read the printed face, called the bear unmodified, and never
// granted the double strike the kernel granted.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestWarMachineAnimatedLicidAuraGrantsDoubleStrike drives the real
// activation (Analyze -> Attach), the real attack, and the real trigger.
func TestWarMachineAnimatedLicidAuraGrantsDoubleStrike(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{
		lookup(t, reg, "War Machine, Avenging Arsenal"),
		lookup(t, reg, "Convulsing Licid"),
		lookup(t, reg, "Grizzly Bears"),
	}, []*cards.Card{})
	wm := moveByName(t, e, 0, "War Machine, Avenging Arsenal", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	licid := moveByName(t, e, 0, "Convulsing Licid", state.ZBattlefield)
	// SetupSettled is the setup harness's event for "this permanent has been
	// here since before the game": it clears summoning sickness, which
	// pay.TapCostSick requires before the {T} activation cost is payable.
	e.emit(events.Event{Kind: events.SetupSettled, Obj: licid})
	addMana(t, e, 0, "R")

	// Activate the Licid's Animate ({R}, {T}): it becomes an Aura and its
	// Attach sub-ability attaches it to a creature.
	activateAbilityOn(t, e, licid)
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending during the Licid activation")
		}
		switch d.Kind {
		case decision.KTarget:
			answerKTarget(t, e, bear)
		case decision.KPriority:
			if len(e.G.Stack) == 0 {
				goto activated
			}
			for _, o := range d.Options {
				if o.Kind == "pass" {
					submitChoices(t, e, o.Index)
					break
				}
			}
		default:
			t.Fatalf("unexpected decision %s during the Licid activation: %+v", d.Kind, d.Options)
		}
	}
	t.Fatal("the Licid activation never drained its stack")
activated:

	// Precondition: the Animate really published the layer-4 Aura type for
	// the Licid (CR 613.1d), the Licid is attached to the bear under the same
	// controller, and neither creature carries a printed double strike yet.
	auraInLayer4 := false
	for _, d := range e.EffectiveTypes() {
		if d.ID != licid {
			continue
		}
		for _, ty := range d.Types {
			if ty == "Aura" {
				auraInLayer4 = true
			}
		}
	}
	if !auraInLayer4 {
		t.Fatalf("precondition failed: the Licid's layer-4 type entry has no Aura (effective types %+v)", e.EffectiveTypes())
	}
	if got := e.G.Obj(licid).AttachedTo; got != bear {
		t.Fatalf("precondition failed: Licid attached to %d, want the bear %d", got, bear)
	}
	if e.G.Obj(licid).Controller != e.G.Obj(bear).Controller {
		t.Fatalf("precondition failed: Licid controller=%d bear controller=%d, want the same",
			e.G.Obj(licid).Controller, e.G.Obj(bear).Controller)
	}
	if e.HasKeyword(bear, "Double Strike") || e.HasKeyword(wm, "Double Strike") {
		t.Fatalf("precondition failed: bear=%t war machine=%t already carry double strike",
			e.HasKeyword(bear, "Double Strike"), e.HasKeyword(wm, "Double Strike"))
	}

	// Both attack: War Machine's "whenever War Machine attacks" trigger fires,
	// and its PumpAll reaches the bear only if the Licid's Aura counts.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{bear, wm}})
	e.putTriggersOnStack()
	if len(e.G.Stack) == 0 {
		t.Fatal("War Machine's attacks trigger did not reach the stack")
	}
	e.resolveTop()

	if !e.HasKeyword(bear, "Double Strike") {
		t.Fatalf("the attacker enchanted by its controller's animated Licid Aura must gain double strike (CR 700.9, CR 613.1d)")
	}
	if e.HasKeyword(wm, "Double Strike") {
		t.Fatalf("War Machine is attacking but UNMODIFIED (no counters, nothing attached): its own grant must not reach it")
	}
	replayCheck(t, e, cfg)
}

// TestWarMachineUnmodifiedAttackerStaysUntouched is the negative that keeps
// the grant honest with no Licid in play at all: an unmodified attacker is
// not granted anything by War Machine's trigger.
func TestWarMachineUnmodifiedAttackerStaysUntouched(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{
		lookup(t, reg, "War Machine, Avenging Arsenal"),
		lookup(t, reg, "Grizzly Bears"),
	}, []*cards.Card{})
	wm := moveByName(t, e, 0, "War Machine, Avenging Arsenal", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)

	if len(e.G.Obj(bear).Counters) != 0 || e.G.Obj(bear).AttachedTo != 0 {
		t.Fatalf("precondition failed: bear counters=%v attachedTo=%d, want none", e.G.Obj(bear).Counters, e.G.Obj(bear).AttachedTo)
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{bear, wm}})
	e.putTriggersOnStack()
	if len(e.G.Stack) == 0 {
		t.Fatal("War Machine's attacks trigger did not reach the stack")
	}
	e.resolveTop()
	if e.HasKeyword(bear, "Double Strike") || e.HasKeyword(wm, "Double Strike") {
		t.Fatalf("an unmodified board must not be granted double strike: bear=%t war machine=%t",
			e.HasKeyword(bear, "Double Strike"), e.HasKeyword(wm, "Double Strike"))
	}
	replayCheck(t, e, cfg)
}
