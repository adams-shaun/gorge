package rules

// cr-g243: an Animate staticAbilities$ grant of a combat restriction or
// requirement mode (MustAttack, CantAttack, CantBlock, CanAttackDefender)
// registers as the animated object's own static (CR 611.3a) and binds the
// declaration sites the Effect-registered walk already consults (CR 508.1d,
// 509.1c). Nettling Imp's MustAttack and Traumatic Prank's CantBlock are the
// real corpus carriers; no Forge script text is inlined.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// animateBoardObj finds seat p's battlefield object with the given face name.
func animateBoardObj(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("seat %d has no %s on the battlefield", p, name)
	return 0
}

// animateImpOnLion resolves Nettling Imp's real Animate activation during
// p1's begin-combat step (the active player is p1; seat 0's Imp targets p1's
// Silvercoat Lion), and asserts the grant registered rather than falling
// through to the unimplemented Note.
func animateImpOnLion(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	imp, ok := reg.Lookup("Nettling Imp")
	if !ok {
		t.Fatal("corpus fixture: Nettling Imp missing")
	}
	lion, ok := reg.Lookup("Silvercoat Lion")
	if !ok {
		t.Fatal("corpus fixture: Silvercoat Lion missing")
	}
	e, cfg := restrictionGame(t, 901,
		[][]*cards.Card{{}, {}},
		[][]*cards.Card{{imp}, {lion}})
	impID := animateBoardObj(t, e, 0, "Nettling Imp")
	lionID := animateBoardObj(t, e, 1, "Silvercoat Lion")

	driveToStepAll(t, e, 3, 1, state.StepBeginCombat)
	offered := false
	var opt decision.Option
	for i := 0; i < 24; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("non-priority decision while waiting for seat 0's Imp activation: %+v", d)
		}
		if d.Player == 0 {
			if o, ok := findAbilityOption(e, impID, 0); ok {
				opt, offered = o, true
				break
			}
		}
		passOnce(t, e)
	}
	if !offered {
		t.Fatal("Nettling Imp's ability was never offered to seat 0 during p1's begin-combat step")
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target decision after activating Nettling Imp: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == lionID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Silvercoat Lion not offered as Nettling Imp's target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 200)

	if n := animGrantNotes(e, "Animate staticAbilities$ MustAttack mode MustAttack is not implemented"); n != 0 {
		t.Fatalf("the MustAttack grant emitted %d unimplemented Notes", n)
	}
	if !e.attackRequirements(lionID).Any() {
		t.Fatal("precondition: the animated Lion carries no MustAttack requirement")
	}
	if o := e.G.Obj(lionID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: the Lion is not p1's battlefield permanent: %+v", o)
	}
	return e, cfg, impID, lionID
}

func TestAnimateGrantedMustAttackForcesAttack(t *testing.T) {
	t.Parallel()
	e, cfg, _, lionID := animateImpOnLion(t)

	driveToStepAll(t, e, 3, 1, state.StepDeclareAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	found := false
	var lionOpt decision.Option
	for _, o := range d.Options {
		if o.Obj == lionID {
			found, lionOpt = true, o
		}
	}
	if !found {
		t.Fatalf("precondition: the must-attack Lion is not offered: %+v", d.Options)
	}
	if !lionOpt.Required {
		t.Fatalf("the Lion's attack pair is not marked Required: %+v", lionOpt)
	}
	if q := d.RequiredQuota(); q != 1 {
		t.Fatalf("RequiredQuota = %d, want 1 (the Lion must attack)", q)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: nil}); err == nil {
		t.Fatal("the empty declaration was accepted although the animated Lion must attack")
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("the rejected declaration consumed the attackers decision: %+v", d)
	}
	submitAttackers(t, e, lionID)
	replayCheck(t, e, cfg)
}

func TestAnimateGrantedMustAttackEndsWithAnimation(t *testing.T) {
	t.Parallel()
	e, cfg, _, lionID := animateImpOnLion(t)

	driveToStepAll(t, e, 3, 1, state.StepDeclareAttackers)
	submitAttackers(t, e, lionID)

	driveToStepAll(t, e, 5, 1, state.StepDeclareAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision on the next p1 turn, got %+v", d)
	}
	if q := d.RequiredQuota(); q != 0 {
		t.Fatalf("RequiredQuota = %d on the next turn, want 0 (the animation expired)", q)
	}
	if e.attackRequirements(lionID).Any() {
		t.Fatal("the MustAttack requirement outlived the until-end-of-turn animation")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: nil}); err != nil {
		t.Fatalf("the next turn's empty declaration was rejected although the animation expired: %v", err)
	}
	replayCheck(t, e, cfg)
}

func TestAnimateGrantedCantBlock(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	prank, ok := reg.Lookup("Traumatic Prank")
	if !ok {
		t.Fatal("corpus fixture: Traumatic Prank missing")
	}
	lion, ok := reg.Lookup("Silvercoat Lion")
	if !ok {
		t.Fatal("corpus fixture: Silvercoat Lion missing")
	}
	bears, ok := reg.Lookup("Grizzly Bears")
	if !ok {
		t.Fatal("corpus fixture: Grizzly Bears missing")
	}
	blocker := card(t, "Name:Animate Test Blocker\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	e, cfg := restrictionGame(t, 902,
		[][]*cards.Card{{prank}, {}},
		[][]*cards.Card{{bears}, {lion, blocker}})
	lionID := animateBoardObj(t, e, 1, "Silvercoat Lion")
	bearsID := animateBoardObj(t, e, 0, "Grizzly Bears")
	blockerID := animateBoardObj(t, e, 1, "Animate Test Blocker")

	castCorpusSpell(t, e, "Traumatic Prank", "CCR", lionID)
	if n := animGrantNotes(e, "Animate staticAbilities$ CantBlock mode CantBlock is not implemented"); n != 0 {
		t.Fatalf("the CantBlock grant emitted %d unimplemented Notes", n)
	}
	if !e.blockRestricted(lionID, bearsID) {
		t.Fatal("precondition: the animated Lion is not forbidden from blocking")
	}
	if e.blockRestricted(blockerID, bearsID) {
		t.Fatal("precondition: the ordinary blocker is already forbidden")
	}

	// The Prank's control change ends with turn 2, so the Lion is p1's again
	// on p0's next turn and would be a legal blocker but for the perpetual
	// CantBlock the Animate granted.
	driveToStepAll(t, e, 4, 0, state.StepDeclareAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	submitAttackers(t, e, bearsID)

	d = e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	sawLion, sawBlocker := false, false
	for _, o := range d.Options {
		switch o.Obj {
		case lionID:
			sawLion = true
		case blockerID:
			sawBlocker = true
		}
	}
	if !sawBlocker {
		t.Fatalf("precondition: the ordinary blocker is not offered: %+v", d.Options)
	}
	if sawLion {
		t.Fatalf("the animate-granted CantBlock Lion was offered as a blocker: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: nil}); err != nil {
		t.Fatalf("submit empty blockers: %v", err)
	}
	replayCheck(t, e, cfg)
}
