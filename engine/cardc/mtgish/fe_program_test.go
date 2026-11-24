package mtgish

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

var (
	tFilt  = tn("TestOnly", "Filter")
	tRule  = tn("TestOnly", "Rule")
	tN1    = tn("TestOnly", "N", 1)
	tN2    = tn("TestOnly", "N", 2)
	tCond  = tn("TestOnly", "Cond")
	tTrig  = tn("TestOnly", "Trigger")
	tCost  = tn("TestOnly", "Cost")
	tExp   = tn("Expiration", "UntilEndOfTurn")
	tEff   = tn("TestOnly", "Effect")
	tColor = tn("TestOnly", "Color")
)

func alFilter() *ir.Filter {
	return &ir.Filter{Subject: ir.SubjPermanent, Zones: zoneBit(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PAny}
}

// alStubs stands in for the lowerings other tickets own: a filter, a
// condition, a trigger, a cost, an effect, a color, a rule (an empty spell
// ability), and the enter flags.
func alStubs(t *testing.T, extra map[string]lowerFunc) {
	t.Helper()
	filtStubs(t)
	stubs := map[string]lowerFunc{
		"TestOnly.Filter": func(*Ctx, node) (any, error) { return alFilter(), nil },
		"TestOnly.Cond":   func(*Ctx, node) (any, error) { return &ir.Cond{Kind: ir.CKicked}, nil },
		"TestOnly.Trigger": func(*Ctx, node) (any, error) {
			return &ir.Trigger{Event: ir.EvDies}, nil
		},
		"TestOnly.Cost":   func(*Ctx, node) (any, error) { return &ir.Cost{HasMana: true}, nil },
		"TestOnly.Effect": func(*Ctx, node) (any, error) { return &ir.Effect{Kind: ir.EffHexproof}, nil },
		"TestOnly.Color":  func(*Ctx, node) (any, error) { return ir.ColorU, nil },
		"TestOnly.Rule": func(*Ctx, node) (any, error) {
			return &ir.Ability{Kind: ir.AbSpell}, nil
		},
		"Permanents.Stub": func(*Ctx, node) (any, error) { return alFilter(), nil },
		"Players.Stub": func(*Ctx, node) (any, error) {
			return &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}, nil
		},
		"EnterFlag.EntersNormally": func(*Ctx, node) (any, error) { return &ir.Enter{}, nil },
		"EnterFlag.EntersUnderOwnersControl": func(*Ctx, node) (any, error) {
			return &ir.Enter{UnderOwner: true}, nil
		},
		"EnterFlag.WithACounter": func(*Ctx, node) (any, error) {
			return &ir.Enter{Counter: "P1P1", N: refConst1()}, nil
		},
		"EnterFlag.WithOtherCounter": func(*Ctx, node) (any, error) {
			return &ir.Enter{Counter: "LOYALTY", N: refConst1()}, nil
		},
	}
	for k, f := range extra {
		stubs[k] = f
	}
	withStubs(t, stubs)
}

func targetPerm() node { return tn("Target", "TargetPermanent", tn("Permanents", "Stub")) }
func targetPlayer() node {
	return tn("Target", "TargetPlayer", tn("Players", "Stub"))
}

func destroyTarget() node {
	return tn("Action", "DestroyPermanent", tn("Permanent", "Ref_TargetPermanent"))
}

var destroySlot0 = &ir.Step{Verb: ir.VerbDestroy, Obj: &ir.Ref{Kind: ir.RefTarget, Slot: 0}}

func TestTargetFrames(t *testing.T) {
	innerSeenOuter := true
	alStubs(t, map[string]lowerFunc{
		// The nested rule declares its own player target in a fresh frame.
		"Rule.NestedStub": func(c *Ctx, n node) (any, error) {
			_, innerSeenOuter = c.TargetSlot("Ref_TargetPermanent")
			body, err := c.Program(tn("Actions", "Targeted", []any{targetPlayer()},
				tn("Actions", "ActionList", []any{
					tn("Action", "LookAtPlayersHand", tn("Player", "Ref_TargetPlayer"))})))
			if err != nil {
				return nil, err
			}
			return &ir.Ability{Kind: ir.AbSpell, Body: body}, nil
		},
	})
	nested := tn("Rule", "NestedStub")
	prog := tn("Actions", "Targeted", []any{targetPerm()}, tn("Actions", "ActionList", []any{
		destroyTarget(),
		tn("Action", "CreatePermanentLayerEffectUntil", tn("Permanent", "Ref_TargetPermanent"),
			[]any{tn("LayerEffect", "AddAbility", []any{nested})}, tExp),
		destroyTarget(),
	}))
	c := &Ctx{}
	p, err := c.Program(prog)
	if err != nil {
		t.Fatal(err)
	}
	inner := &ir.Program{
		Steps: []*ir.Step{{Verb: ir.VerbLookAtHand, Obj: &ir.Ref{Kind: ir.RefTarget, Slot: 0}}},
		Targets: []*ir.Target{{Min: 1, Max: 1,
			Filter: &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}}},
	}
	want := &ir.Program{
		Steps: []*ir.Step{
			destroySlot0,
			{Verb: ir.VerbApplyEffect, Obj: &ir.Ref{Kind: ir.RefTarget, Slot: 0}, Dur: ir.DurEndOfTurn,
				Effects: []*ir.Effect{{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{Kind: ir.AbSpell, Body: inner}}}}},
			destroySlot0,
		},
		Targets: []*ir.Target{{Min: 1, Max: 1, Filter: alFilter()}},
	}
	if !ir.Equal(p, want) {
		t.Errorf("Targeted:\n got %s\nwant %s", p, want)
	}
	if innerSeenOuter {
		t.Error("the nested rule saw the outer frame's Ref_TargetPermanent")
	}

	// After a nested ability the caller's declarations are exactly as before.
	c = &Ctx{}
	c.DeclareTarget("TargetPermanent")
	before := c.targets
	if _, err := c.NestedAbilities([]any{nested}); err != nil {
		t.Fatal(err)
	}
	if s, ok := c.TargetSlot("Ref_TargetPermanent"); !ok || s != 0 || len(c.targets) != 1 || &c.targets[0] != &before[0] {
		t.Errorf("caller frame changed: slot %d %v, %d declared", s, ok, len(c.targets))
	}
	// A failing frame restores too.
	c = &Ctx{}
	c.DeclareTarget("TargetPermanent")
	if _, err := c.NestedAbilities([]any{tn("Rule", "NoSuchRule")}); err == nil {
		t.Fatal("want a refusal")
	}
	if len(c.targets) != 1 {
		t.Error("failed nested frame did not restore the caller's targets")
	}

	// A nested target list is an error.
	bad := tn("Actions", "Targeted", []any{targetPerm()}, tn("Actions", "Targeted", []any{targetPerm()},
		tn("Actions", "ActionList", []any{})))
	if _, err := (&Ctx{}).Program(bad); err == nil || err.Error() != "Actions.Targeted: nested target list" {
		t.Errorf("nested Targeted: %v", err)
	}
}

func TestModesHaveOwnTargets(t *testing.T) {
	alStubs(t, nil)
	mode := func() node {
		return tn("Actions", "Targeted", []any{targetPerm()}, tn("Actions", "ActionList", []any{destroyTarget()}))
	}
	c := &Ctx{}
	p, err := c.Program(tn("Actions", "Modal_ChooseOne", []any{mode(), mode()}))
	if err != nil {
		t.Fatal(err)
	}
	modeProg := &ir.Program{Steps: []*ir.Step{destroySlot0}, Targets: []*ir.Target{{Min: 1, Max: 1, Filter: alFilter()}}}
	want := &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbChooseModes, N: refConst1(), Modes: []*ir.Program{modeProg, modeProg}}}}
	if !ir.Equal(p, want) {
		t.Errorf("Modal_ChooseOne:\n got %s\nwant %s", p, want)
	}
}

func TestStepsSplice(t *testing.T) {
	alStubs(t, nil)
	tok := func() node {
		return tn("CreatableToken", "FoodToken")
	}
	exil := func() node { return tn("Exilable", "Permanent", tn("TestOnly", "Ref")) }
	p, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{
		tn("Action", "DrawACard"),
		tn("Action", "CreateTokens", []any{tok(), tok()}, []any{}),
		tn("Action", "Exile", []any{exil(), exil()}, []any{}),
		tn("Action", "EndTheTurn"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	var verbs []ir.Verb
	for _, s := range p.Steps {
		verbs = append(verbs, s.Verb)
	}
	wantVerbs := []ir.Verb{ir.VerbDraw, ir.VerbCreateToken, ir.VerbCreateToken, ir.VerbExile, ir.VerbExile, ir.VerbEndTurn}
	if len(verbs) != len(wantVerbs) {
		t.Fatalf("verbs = %v, want %v", verbs, wantVerbs)
	}
	for i := range verbs {
		if verbs[i] != wantVerbs[i] {
			t.Fatalf("verbs = %v, want %v", verbs, wantVerbs)
		}
	}
	// A spliced program must not drop targets.
	withStubs(t, map[string]lowerFunc{"Action.Stub": func(*Ctx, node) (any, error) {
		return &ir.Program{Targets: []*ir.Target{{}}}, nil
	}})
	if _, err := (&Ctx{}).Steps(tn("Action", "Stub")); err == nil {
		t.Error("splicing a program with targets must fail")
	}
}

func TestEntersMerge(t *testing.T) {
	alStubs(t, nil)
	c := &Ctx{}
	got, err := c.Enters([]any{tn("TokenFlag", "EntersTapped"), tn("EnterFlag", "EntersUnderOwnersControl")})
	if err != nil {
		t.Fatal(err)
	}
	if !ir.Equal(got, &ir.Enter{Tapped: true, UnderOwner: true}) {
		t.Errorf("merge = %s", got)
	}
	if got, err := c.Enters(nil); got != nil || err != nil {
		t.Errorf("empty list: %v %v", got, err)
	}
	if got, err := c.Enters([]any{tn("EnterFlag", "EntersNormally")}); got != nil || err != nil {
		t.Errorf("EntersNormally: %v %v", got, err)
	}
	_, err = c.Enters([]any{tn("EnterFlag", "WithACounter"), tn("EnterFlag", "WithOtherCounter")})
	if err == nil {
		t.Error("two counter flags must conflict")
	} else if want := "EnterFlag.WithOtherCounter: conflicting enter flags"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}
