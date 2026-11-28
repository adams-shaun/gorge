package rules

// A spell that is granted convoke (Wand of the Worldsoul's "The next spell you
// cast this turn has convoke") has convoke when it is cast, so both
// SpellCast-triggers over a spell with convoke queue (CR 601.2i, 702.51a):
// Saint Traft and Rem Karolus's `Card.withConvoke` and Kasla, the Broken
// Halo's `Card.hasKeywordConvoke+Other`. The grant ends at the cast
// (ForgetOnCast$), after the triggers are read. All cards are authored.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	grantConvokeWandSrc = "Name:Fixture Wand\nManaCost:2\nTypes:Artifact\nOracle:x\n" +
		"A:AB$ Effect | Cost$ T | StaticAbilities$ GrantConvoke | ForgetOnCast$ Card.YouCtrl | SpellDescription$ The next spell you cast this turn has convoke.\n" +
		"SVar:GrantConvoke:Mode$ Continuous | Affected$ Card.YouCtrl | AffectedZone$ Stack | AddKeyword$ Convoke | Description$ The next spell you cast this turn has convoke.\n"
	grantConvokeWithSrc = "Name:Fixture With Watcher\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n" +
		"T:Mode$ SpellCast | ValidCard$ Card.withConvoke | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigGain | TriggerDescription$ Whenever you cast a spell that has convoke, gain 1 life.\n" +
		"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\n"
	grantConvokeHasSrc = "Name:Fixture Has Watcher\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n" +
		"K:Convoke\n" +
		"T:Mode$ SpellCast | ValidCard$ Card.hasKeywordConvoke+Other | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigGain | TriggerDescription$ Whenever you cast another spell that has convoke, gain 1 life.\n" +
		"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\n"
	grantConvokeSpellSrc = "Name:Fixture Spell\nManaCost:1 W\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
	grantConvokeBodySrc  = "Name:Fixture Body\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
)

func TestGrantedConvokeQueuesBothSpellCastTriggers(t *testing.T) {
	t.Parallel()
	wandC := card(t, grantConvokeWandSrc)
	withC := card(t, grantConvokeWithSrc)
	hasC := card(t, grantConvokeHasSrc)
	spellC := card(t, grantConvokeSpellSrc)
	bodyC := card(t, grantConvokeBodySrc)
	e, _ := tokenReplGame(t, 8811, wandC, withC, hasC, spellC, bodyC)
	toMain1(t, e)
	wand := onBoardCard(t, e, 0, wandC)
	with := onBoardCard(t, e, 0, withC)
	has := onBoardCard(t, e, 0, hasC)
	body := onBoardCard(t, e, 0, bodyC)
	spell := searchMoveByName(t, e, "Fixture Spell", state.ZHand)
	for _, id := range []state.ObjID{wand, with, has, body, spell} {
		if o := e.G.Obj(id); o == nil || o.Zone == state.ZLibrary {
			t.Fatalf("fixture object %d not placed: %+v", id, o)
		}
	}
	if o := e.G.Obj(spell); o.Face().HasKeyword("Convoke") {
		t.Fatal("precondition: the spell prints convoke")
	}
	opt, ok := findAbilityOptionByLabel(e, wand, "next spell")
	if !ok {
		t.Fatalf("the grant ability is not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	resolveTop(t, e)
	// The grant must be live before the cast (else the test proves nothing).
	castWithConvokeAt(t, e, spell, "W", body)
	if o := e.G.Obj(spell); o == nil || o.Zone != state.ZStack {
		t.Fatalf("precondition: the spell is not on the stack: %+v", o)
	}
	if b := e.G.Obj(body); b == nil || !b.Tapped {
		t.Fatal("precondition: the body creature did not convoke the spell (the grant was not live)")
	}
	got := map[state.ObjID]int{}
	for _, pt := range e.pendingTriggers {
		got[pt.Source]++
	}
	if got[with] != 1 {
		t.Errorf("Card.withConvoke trigger queued %d times for a spell granted convoke, want 1 (queued %v)", got[with], got)
	}
	if got[has] != 1 {
		t.Errorf("Card.hasKeywordConvoke+Other trigger queued %d times for a spell granted convoke, want 1 (queued %v)", got[has], got)
	}
}
