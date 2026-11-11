// clone_addabilities_copiable_test.go pins CR 707.9b/707.3 for a Clone's
// AddAbilities$ rider: "except it has <ability>" is part of the copy's
// copiable values, so a Clone of that copy has the ability too (it was a
// layer-6 grant bound to the one object, lost to any later copy).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// offeredAbilityOption returns the priority option index of obj's printed
// ability with API api, or -1.
func offeredAbilityOption(e *Engine, obj state.ObjID, api string) int {
	e.priorityRound()
	d := e.Pending()
	if d == nil {
		return -1
	}
	o := e.G.Obj(obj)
	for _, op := range d.Options {
		if op.Kind == "ability" && op.Obj == obj && op.SVar == "" &&
			op.Ability >= 0 && op.Ability < len(o.Face().Abilities) && o.Face().Abilities[op.Ability].API == api {
			return op.Index
		}
	}
	return -1
}

// TestCloneOfMercurialPretenderCopyHasTheReturnAbility: Mercurial Pretender
// enters as a Grizzly Bears with "{2}{U}{U}: Return this creature to its
// owner's hand"; a Clone copying that Pretender is a Grizzly Bears with the
// same ability (CR 707.3), and the Pretender's own copy still has exactly one
// (the grant is not also registered as a layer-6 effect).
func TestCloneOfMercurialPretenderCopyHasTheReturnAbility(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1431, []string{"Mercurial Pretender", "Clone", "Grizzly Bears"}, nil, nil)
	bears := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	mp := findCardObj(t, e, 0, "Mercurial Pretender", state.ZHand)
	clone := findCardObj(t, e, 0, "Clone", state.ZHand)
	if n := len(e.G.Obj(clone).Card.Faces[0].Abilities); n != 0 {
		t.Fatalf("precondition: Clone's printed card has %d abilities, want none", n)
	}
	addMana(t, e, 0, "CCCCU")
	castCopyCard(t, e, mp, bears, 0)
	po := e.G.Obj(mp)
	if po.Zone != state.ZBattlefield || po.Face().Name != "Grizzly Bears" || changeZoneAbilities(po) != 1 {
		t.Fatalf("precondition: pretender zone %v name %q with %d return abilities, want a Grizzly Bears with one", po.Zone, po.Face().Name, changeZoneAbilities(po))
	}

	addMana(t, e, 0, "CCCU")
	castCopyCard(t, e, clone, mp, 0)
	co := e.G.Obj(clone)
	if co.Zone != state.ZBattlefield || co.Face().Name != "Grizzly Bears" {
		t.Fatalf("Clone is %q in zone %v, want a Grizzly Bears on the battlefield", co.Face().Name, co.Zone)
	}
	if n := changeZoneAbilities(co); n != 1 {
		t.Fatalf("Clone of the Pretender has %d return-to-hand abilities, want 1 (CR 707.3)", n)
	}
	if n := changeZoneAbilities(e.G.Obj(mp)); n != 1 {
		t.Fatalf("the Pretender has %d return-to-hand abilities, want exactly 1", n)
	}
	if n := changeZoneAbilities(e.G.Obj(bears)); n != 0 {
		t.Fatalf("the template bear has %d return-to-hand abilities, want none", n)
	}
	if n := len(e.grantedAbilities(0, clone)) + len(e.grantedAbilities(0, mp)); n != 0 {
		t.Fatalf("%d layer-6 granted abilities, want none (the grant is copiable)", n)
	}

	addMana(t, e, 0, "CCUU")
	opt := offeredAbilityOption(e, clone, "ChangeZone")
	if opt < 0 {
		t.Fatal("the Clone's return-to-hand ability is not offered at priority")
	}
	submitChoices(t, e, opt)
	passUntilStackEmpty(t, e, 8)
	if e.G.Obj(clone).Zone != state.ZHand {
		t.Fatalf("the Clone is in zone %v, want its owner's hand", e.G.Obj(clone).Zone)
	}
	if e.G.Obj(mp).Zone != state.ZBattlefield || e.G.Obj(bears).Zone != state.ZBattlefield {
		t.Fatal("the activation moved the Pretender or the bear, want only the Clone")
	}
	replayCheck(t, e, cfg)
}

// TestCloneOfSakashimaCopyHasTheDelayedReturn: Sakashima the Impostor's rider
// is an ability whose Execute$ names a second grant (AddSVars$), so the Clone
// of the Sakashima copy needs both: the DelayedTrigger ability and the SVar
// body it resolves.
func TestCloneOfSakashimaCopyHasTheDelayedReturn(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1432, []string{"Sakashima the Impostor", "Clone", "Grizzly Bears"}, nil, nil)
	bears := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	sk := findCardObj(t, e, 0, "Sakashima the Impostor", state.ZHand)
	clone := findCardObj(t, e, 0, "Clone", state.ZHand)
	addMana(t, e, 0, "CCUU")
	castCopyCard(t, e, sk, bears, 0)
	so := e.G.Obj(sk)
	if so.Zone != state.ZBattlefield || so.Face().Name != "Sakashima the Impostor" || so.Face().SVars["TrigReturnSak"] == "" {
		t.Fatalf("precondition: Sakashima is %q in zone %v with SVars %v", so.Face().Name, so.Zone, so.Face().SVars)
	}
	if _, has := e.G.Obj(clone).Card.Faces[0].SVars["TrigReturnSak"]; has {
		t.Fatal("precondition: Clone's printed card already has the SVar")
	}
	addMana(t, e, 0, "CCCU")
	castCopyCard(t, e, clone, sk, 0)
	co := e.G.Obj(clone)
	if co.Zone != state.ZBattlefield {
		t.Fatalf("Clone is in zone %v, want the battlefield", co.Zone)
	}
	var delayed int
	for _, sa := range co.Face().Abilities {
		if sa.API == "DelayedTrigger" {
			delayed++
		}
	}
	if delayed != 1 || co.Face().SVars["TrigReturnSak"] == "" {
		t.Fatalf("Clone of Sakashima has %d DelayedTrigger abilities and SVars %v, want 1 and TrigReturnSak", delayed, co.Face().SVars)
	}
	replayCheck(t, e, cfg)
}

// TestMercurialPretenderCopyingAPretenderCopyHasOneReturnAbility: the second
// Pretender copies the first's copy, whose basis face already carries the
// "{2}{U}{U}: Return this creature" ability; its own exception adds the same
// ability, which the copy has once (the fold skips a line the face has), and
// priority offers it once, marked as a copy's grant, not a printed line.
func TestMercurialPretenderCopyingAPretenderCopyHasOneReturnAbility(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1433, []string{"Mercurial Pretender", "Mercurial Pretender", "Grizzly Bears"}, nil, nil)
	bears := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	mp1 := findCardObj(t, e, 0, "Mercurial Pretender", state.ZHand)
	addMana(t, e, 0, "CCCCU")
	castCopyCard(t, e, mp1, bears, 0)
	if o := e.G.Obj(mp1); o.Zone != state.ZBattlefield || changeZoneAbilities(o) != 1 {
		t.Fatalf("precondition: the first Pretender is %q in zone %v with %d return abilities, want a copy with one", o.Face().Name, o.Zone, changeZoneAbilities(o))
	}
	mp2 := findCardObj(t, e, 0, "Mercurial Pretender", state.ZHand)
	if mp2 == mp1 {
		t.Fatal("precondition: both Pretenders are the same object")
	}
	addMana(t, e, 0, "CCCCU")
	castCopyCard(t, e, mp2, mp1, 0)
	o2 := e.G.Obj(mp2)
	if o2.Zone != state.ZBattlefield || o2.Face().Name != "Grizzly Bears" {
		t.Fatalf("the second Pretender is %q in zone %v, want a Grizzly Bears on the battlefield", o2.Face().Name, o2.Zone)
	}
	if n := changeZoneAbilities(o2); n != 1 {
		t.Fatalf("the second Pretender's copy has %d return-to-hand abilities, want 1", n)
	}
	addMana(t, e, 0, "CCUU")
	e.priorityRound()
	n := 0
	for _, op := range e.Pending().Options {
		if op.Kind == "ability" && op.Obj == mp2 {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d ability options offered for the second Pretender, want 1", n)
	}
	replayCheck(t, e, cfg)
}
