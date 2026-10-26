package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestCantBeCopiedBlocksStormCopies(t *testing.T) {
	protected := "Name:Test Uncopyable Storm\nManaCost:0\nTypes:Instant\nK:Storm\n" +
		"S:Mode$ CantBeCopied | ValidCard$ Card.Self | EffectZone$ Stack\n" +
		"A:SP$ Draw | NumCards$ 1\nOracle:x\n"
	setup := "Name:Test Setup Spell\nManaCost:R\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 97, protected, setup)
	setupID := addToHand(t, e, 0, setup)
	stormID := addToHand(t, e, 0, protected)
	addMana(t, e, 0, "R")
	castObj(t, e, setupID)
	passUntilStackEmpty(t, e, 20)
	if e.CastThisTurn() != 1 {
		t.Fatalf("precondition: setup spell cast count = %d, want one before Storm", e.CastThisTurn())
	}

	castCardNow(t, e, "Test Uncopyable Storm")
	if e.G.Obj(stormID) == nil || e.G.Obj(stormID).Zone != state.ZStack {
		t.Fatalf("precondition: protected Storm spell %d is not on the stack", stormID)
	}
	if e.SpellCopyAllowed(stormID) {
		t.Fatal("precondition: CantBeCopied static did not prohibit this stack spell")
	}
	passUntilStackEmpty(t, e, 30)
	for _, o := range e.G.Objs {
		if o.IsCopy {
			t.Fatalf("protected spell produced copy %d", o.ID)
		}
	}
}

const cantCopySifterSrc = "Name:X Sifter\nTypes:Artifact\n" +
	"A:AB$ Draw | Cost$ X T | NumCards$ X | CantCopy$ True | SpellDescription$ Draw X cards.\n" +
	"Oracle:Draw X cards. This ability can't be copied.\n"

func cantCopyBoard(t *testing.T, seed uint64, sifter string) (*Engine, Config, state.ObjID, int) {
	t.Helper()
	e, cfg, _ := newFixtureDeck(t, seed, enigmaJewelAbilityCopySrc, sifter, plainSifterSrc)
	moveSeeded(t, e, 0, enigmaJewelAbilityCopySrc, state.ZBattlefield)
	moveSeeded(t, e, 0, sifter, state.ZBattlefield)
	moveSeeded(t, e, 0, plainSifterSrc, state.ZBattlefield)
	abil := glavaAnnounce(t, e, 1)
	return e, cfg, abil, len(e.G.Zone(state.ZHand, 0))
}

func TestCantCopyAbilityIsNotCopied(t *testing.T) {
	t.Parallel()
	e, cfg, abil, handBefore := cantCopyBoard(t, 141, cantCopySifterSrc)
	if o := e.G.Obj(abil); o == nil || o.Zone != state.ZStack || o.Ability == nil {
		t.Fatalf("precondition: ability wrapper %d not on the stack", abil)
	}
	if e.SpellCopyAllowed(abil) {
		t.Fatal("precondition: CantCopy$ True ability reported copyable")
	}
	drainTriggerAsks(t, e, 40)
	if got := copyCount(e, abil); got != 0 {
		t.Fatalf("StackCopy of CantCopy ability = %d, want 0", got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("hand %d, want %d", got, handBefore+1)
	}
	replayCheck(t, e, cfg)
}

func TestCantCopyLeavesOtherAbilitiesCopyable(t *testing.T) {
	t.Parallel()
	e, cfg, abil, handBefore := cantCopyBoard(t, 141, changeXSifterSrc)
	if !e.SpellCopyAllowed(abil) {
		t.Fatal("precondition: plain ability reported uncopyable")
	}
	drainTriggerAsks(t, e, 40)
	if got := copyCount(e, abil); got != 1 {
		t.Fatalf("StackCopy = %d, want 1", got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+2 {
		t.Fatalf("hand %d, want %d", got, handBefore+2)
	}
	replayCheck(t, e, cfg)
}
