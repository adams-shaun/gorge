package rules

// Effect-delivered spell-flash grants (Mode$ CastWithFlash | ValidSA$ Spell |
// Caster$ You behind an Effect's StaticAbilities$): Borne Upon a Wind,
// Quicken, Teferi, Time Raveler, Alchemist's Refuge, Emergence Zone,
// Winding Canyons. Every scenario holds a Lightning Bolt on the stack so the
// seat is at an instant-speed window (non-empty stack, main phase): a sorcery
// or creature is castable there only through the grant.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// castOfferedFor reports whether the pending priority decision offers a cast
// of id.
func castOfferedFor(e *Engine, id state.ObjID) bool {
	d := e.Pending()
	if d == nil {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			return true
		}
	}
	return false
}

// boltOnStack casts Lightning Bolt (seat 0, at the opponent) and leaves it on
// the stack with seat 0 holding priority, then asserts that precondition.
func boltOnStack(t *testing.T, e *Engine) {
	t.Helper()
	addMana(t, e, 0, "R")
	castNamed(t, e, "Lightning Bolt")
	answerHybridPayment(t, e)
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: Lightning Bolt is not alone on the stack (%d objects)", len(e.G.Stack))
	}
	if d := e.Pending(); d == nil || d.Player != 0 {
		t.Fatalf("precondition: seat 0 does not hold priority: %+v", d)
	}
}

// resolveTop passes priority until the stack is one object shorter.
func resolveTop(t *testing.T, e *Engine) {
	t.Helper()
	n := len(e.G.Stack)
	for i := 0; i < 6 && len(e.G.Stack) >= n; i++ {
		passPriorityOnce(t, e)
	}
	if len(e.G.Stack) >= n {
		t.Fatalf("the top of the stack did not resolve (stack %d, was %d)", len(e.G.Stack), n)
	}
}

func TestEffectFlashGrantBorneUponAWind(t *testing.T) {
	t.Parallel()
	e, cfg, _, hand := effectGrantEngine(t, 9401, nil,
		[]string{"Lightning Bolt", "Borne Upon a Wind", "Divination", "Grizzly Bears"})
	div, bears := hand[2], hand[3]
	boltOnStack(t, e)
	addMana(t, e, 0, "UU")
	if castOfferedFor(e, div) || castOfferedFor(e, bears) {
		t.Fatal("precondition: a sorcery or a creature is offered with a spell on the stack before any grant")
	}
	castNamed(t, e, "Borne Upon a Wind")
	resolveTop(t, e)
	if d := e.Pending(); d == nil || d.Player != 0 || len(e.G.Stack) != 1 {
		t.Fatalf("after Borne resolved seat 0 should hold priority over Bolt: stack %d, %+v", len(e.G.Stack), d)
	}
	addMana(t, e, 0, "UUUGG")
	if !castOfferedFor(e, div) || !castOfferedFor(e, bears) {
		t.Fatalf("Borne Upon a Wind did not grant flash to a sorcery and a creature: %+v", e.Pending().Options)
	}
	if e.castWithFlash(1, div) {
		t.Fatal("Caster$ You: the opponent was granted flash")
	}
	// The grant lasts the turn only.
	driveToStep(t, e, 2, 1, state.StepMain1)
	if e.castWithFlash(0, div) {
		t.Fatal("the grant survived the turn it was made in")
	}
	replayCheck(t, e, cfg)
}

func TestEffectFlashGrantQuickenEndsAfterTheNextSorcery(t *testing.T) {
	t.Parallel()
	e, cfg, _, hand := effectGrantEngine(t, 9402, nil,
		[]string{"Lightning Bolt", "Quicken", "Divination", "Night's Whisper", "Grizzly Bears"})
	div, whisper, bears := hand[2], hand[3], hand[4]
	boltOnStack(t, e)
	addMana(t, e, 0, "U")
	castNamed(t, e, "Quicken")
	resolveTop(t, e)
	addMana(t, e, 0, "UUUBB")
	if !castOfferedFor(e, div) || !castOfferedFor(e, whisper) {
		t.Fatalf("Quicken did not grant flash to a sorcery: %+v", e.Pending().Options)
	}
	if castOfferedFor(e, bears) {
		t.Fatal("Quicken (ValidCard$ Sorcery) granted flash to a creature")
	}
	castNamed(t, e, "Divination")
	if castOfferedFor(e, whisper) || e.castWithFlash(0, whisper) {
		t.Fatal("the grant outlived the next sorcery spell cast (ForgetOnCast$)")
	}
	replayCheck(t, e, cfg)
}

func TestEffectFlashGrantWindingCanyonsCreaturesOnly(t *testing.T) {
	t.Parallel()
	e, cfg, battle, hand := effectGrantEngine(t, 9403, []string{"Winding Canyons"},
		[]string{"Lightning Bolt", "Divination", "Grizzly Bears"})
	div, bears := hand[1], hand[2]
	driveToStep(t, e, 3, 0, state.StepMain1)
	boltOnStack(t, e)
	addMana(t, e, 0, "CC")
	if castOfferedFor(e, bears) {
		t.Fatal("precondition: a creature is offered before the grant")
	}
	opt, ok := findAbilityOptionByLabel(e, battle[0], "Until end of turn, you may cast creature spells as though they had flash.")
	if !ok {
		t.Fatalf("Winding Canyons' grant ability is not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	resolveTop(t, e)
	addMana(t, e, 0, "GGG")
	if !castOfferedFor(e, bears) {
		t.Fatalf("Winding Canyons did not grant flash to a creature: %+v", e.Pending().Options)
	}
	if castOfferedFor(e, div) {
		t.Fatal("Winding Canyons (ValidCard$ Creature) granted flash to a sorcery")
	}
	replayCheck(t, e, cfg)
}
