package gorgebe

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func TestOneShotEffects(t *testing.T) {
	creatures := ztAll(&ir.Filter{Subject: ir.SubjPermanent, Zones: ir.MakeZoneSet(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PCardType, Word: "Creature"})
	pump := &ir.Effect{Kind: ir.EffAdjustPT, P: vc(1), T: vc(1)}
	kw := &ir.Effect{Kind: ir.EffAddAbility, Abilities: ztKW("Trample")}
	// each permanent a filter selects: PumpAll
	ztWant(t, ztFx(ir.DurEndOfTurn, creatures, pump, kw), nil, "PumpAll", []kv{{"NumAtt", "+1"}, {"NumDef", "+1"}, {"KW", "Trample"}, {"ValidCards", "Creature"}})
	ztRefuse(t, ztFx(ir.DurEndOfTurn, creatures, &ir.Effect{Kind: ir.EffSetPT, P: vc(1), T: vc(1)}), nil, "effect:all-set-pt")
	// can't block: a hidden keyword
	tgt := []*ir.Target{ztObjTarget(ir.ZoneBattlefield)}
	ztWant(t, ztFx(ir.DurEndOfTurn, &ir.Ref{Kind: ir.RefTarget}, &ir.Effect{Kind: ir.EffCantBlock}), tgt, "Pump",
		[]kv{{"KW", cantBlockKW}, {"ValidTgts", "Creature"}})
	// an event object that is the source itself
	p := &pr{ev: ir.EvAttacks, evSelf: true}
	n, err := p.program(ztProg(ztFx(ir.DurEndOfTurn, ztRef(ir.RefEventObject), pump)...))
	if err != nil || n.API != "Pump" || n.Params[len(n.Params)-1] != (kv{"Defined", "Self"}) {
		t.Fatalf("event object: %v %v", n, err)
	}
	// can't be blocked: an Effect with a static of the remembered object
	p = &pr{}
	n, err = p.program(&ir.Program{Steps: ztFx(ir.DurEndOfTurn, ztRef(ir.RefSelf), &ir.Effect{Kind: ir.EffCantBeBlocked}), Targets: nil})
	if err != nil || n.API != "Effect" || len(p.svars) != 1 || !strings.HasPrefix(p.svars[0].V, "Mode$ CantBlockBy") {
		t.Fatalf("unblockable: %v %v %v", n, p.svars, err)
	}
	if v, _ := n.get("RememberObjects"); v != "Self" {
		t.Fatalf("RememberObjects %q", v)
	}
	ztRefuse(t, ztFx(ir.DurUntilYourNextTurn, ztRef(ir.RefSelf), &ir.Effect{Kind: ir.EffCantBeBlocked}), nil, "effect:cant-be-blocked-dur")
}

func TestEmblem(t *testing.T) {
	creatures := ztAll(&ir.Filter{Subject: ir.SubjPermanent, Zones: ir.MakeZoneSet(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PCardType, Word: "Creature"})
	static := &ir.Ability{Kind: ir.AbStatic, Body: ztProg(&ir.Step{Verb: ir.VerbApplyEffect, Obj: creatures, Dur: ir.DurWhileActive,
		Effects: []*ir.Effect{{Kind: ir.EffAdjustPT, P: vc(2), T: vc(2)}}})}
	p := &pr{face: &ir.Face{Name: "Test Walker, The Third"}}
	n, err := p.program(ztProg(&ir.Step{Verb: ir.VerbEmblem, Abilities: []*ir.Ability{static}}))
	if err != nil {
		t.Fatal(err)
	}
	if n.API != "Effect" || n.Params[1] != (kv{"Image", "emblem_test_walker_the_third"}) || n.Params[2] != (kv{"Duration", "Permanent"}) {
		t.Fatalf("emblem node %v", n)
	}
	if len(p.svars) != 1 || !strings.HasSuffix(p.svars[0].V, "AffectedZone$ Battlefield") {
		t.Fatalf("emblem static %v", p.svars)
	}
	if _, err := (&pr{}).program(ztProg(&ir.Step{Verb: ir.VerbEmblem, Abilities: []*ir.Ability{static}})); err == nil {
		t.Fatal("emblem without a face printed")
	}
}

func TestKickedTargetCount(t *testing.T) {
	obj := &ir.Ref{Kind: ir.RefTarget}
	add := &ir.Effect{Kind: ir.EffAddAbility, Abilities: ztKW("Indestructible")}
	many := &ir.Target{Min: 0, Max: -1, Filter: ztObjTarget(ir.ZoneBattlefield).Filter}
	one := ztObjTarget(ir.ZoneBattlefield)
	st := &ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CKicked},
		Then: &ir.Program{Steps: ztFx(ir.DurEndOfTurn, obj, add), Targets: []*ir.Target{many}},
		Else: &ir.Program{Steps: ztFx(ir.DurEndOfTurn, obj, add), Targets: []*ir.Target{one}}}
	p := &pr{}
	n, err := p.program(ztProg(st))
	if err != nil || n.Sub != nil {
		t.Fatalf("got %v, %v", n, err)
	}
	if mn, _ := n.get("TargetMin"); mn != "X" {
		t.Fatalf("TargetMin %q", mn)
	}
	var got []string
	for _, s := range p.svars {
		got = append(got, s.K+"="+s.V)
	}
	if strings.Join(got, ";") != "X=Count$Kicked.0.1;Z=Count$Valid Creature;Y=Count$Kicked.Z.1" {
		t.Fatalf("svars %v", got)
	}
}

func TestControlAttachRiders(t *testing.T) {
	tr := &ir.Ref{Kind: ir.RefTarget}
	tgt := []*ir.Target{ztObjTarget(ir.ZoneBattlefield)}
	// gain control with the untap and the haste folded in
	gc := []*ir.Step{
		{Verb: ir.VerbGainControl, Obj: tr, Dur: ir.DurEndOfTurn},
		{Verb: ir.VerbUntap, Obj: tr},
		{Verb: ir.VerbApplyEffect, Obj: tr, Dur: ir.DurEndOfTurn, Effects: []*ir.Effect{{Kind: ir.EffAddAbility, Abilities: ztKW("Haste")}}},
	}
	ztWant(t, gc, tgt, "GainControl", []kv{{"ValidTgts", "Creature"}, {"Untap", "True"}, {"AddKWs", "Haste"}, {"LoseControl", "EOT"}})
	ztWant(t, gc[:1], tgt, "GainControl", []kv{{"ValidTgts", "Creature"}, {"LoseControl", "EOT"}})
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbGainControl, Obj: tr, Dur: ir.DurEndOfCombat}}, tgt, "gain-control:dur-end-of-combat")
	// attach the source to a target
	p := &pr{ev: ir.EvEnters, evSelf: true}
	n, err := p.program(&ir.Program{Targets: tgt, Steps: []*ir.Step{{Verb: ir.VerbAttach, Obj: ztRef(ir.RefEventObject), Other: tr}}})
	if err != nil || n.API != "Attach" || n.Params[0] != (kv{"ValidTgts", "Creature"}) {
		t.Fatalf("attach: %v %v", n, err)
	}
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbAttach, Obj: tr, Other: tr}}, tgt, "verb:attach")
	// flying and the combat-damage prevention ride one Pump
	repl := &ir.Step{Verb: ir.VerbCreateReplacement, Dur: ir.DurEndOfTurn, Abilities: []*ir.Ability{{Kind: ir.AbReplacement,
		Trigger: &ir.Trigger{Event: ir.EvWouldDealDamage, Ref: tr, Word: "combat"}, Body: ztProg(&ir.Step{Verb: ir.VerbPrevent})}}}
	fly := ztFx(ir.DurEndOfTurn, tr, &ir.Effect{Kind: ir.EffAddAbility, Abilities: ztKW("Flying")})
	nn, err := ztPrint(append(fly, repl), tgt...)
	if err != nil || nn.API != "Pump" || nn.Sub != nil {
		t.Fatalf("rider: %v %v", nn, err)
	}
	if kw, _ := nn.get("KW"); kw != "Flying & "+preventCombatKW {
		t.Fatalf("KW %q", kw)
	}
}

func TestMayPlayEffects(t *testing.T) {
	you := ztRef(ir.RefYou)
	exiled := &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcExiled}
	play := &ir.Step{Verb: ir.VerbApplyEffect, Obj: you, Dur: ir.DurEndOfTurn, Effects: []*ir.Effect{{Kind: ir.EffMayPlay, Ref: exiled}}}
	exile := &ir.Step{Verb: ir.VerbExile, Obj: ztTop(1)}
	n, err := ztPrint([]*ir.Step{exile, play})
	if err != nil || n.API != "Dig" || n.Sub == nil || n.Sub.API != "Effect" || n.Sub.Sub == nil || n.Sub.Sub.API != "Cleanup" {
		t.Fatalf("exile then play: %v %v", n, err)
	}
	if v, _ := n.get("RememberChanged"); v != "True" {
		t.Fatalf("dig %v", n.Params)
	}
	// without a following use, the exile stays a Mill
	n, err = ztPrint([]*ir.Step{exile})
	if err != nil || n.API != "Mill" {
		t.Fatalf("lone exile: %v %v", n, err)
	}
	// a choice among the exiled cards, then play the chosen one until your next turn ends
	pick := &ir.Step{Verb: ir.VerbChoose, Obj: &ir.Ref{Kind: ir.RefChoose, N: vc(1), Filter: &ir.Filter{Subject: ir.SubjCard,
		Zones: ir.MakeZoneSet(ir.ZoneExile), Op: ir.FPred, Pred: ir.PIs, Ref: exiled}}}
	chosen := &ir.Step{Verb: ir.VerbApplyEffect, Obj: you, Dur: ir.DurUntilEndOfYourNextTurn,
		Effects: []*ir.Effect{{Kind: ir.EffMayPlay, Ref: &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcChosen}}}}
	n, err = ztPrint([]*ir.Step{exile, pick, chosen})
	if err != nil || n.Sub == nil || n.Sub.API != "ChooseCard" || n.Sub.Sub == nil {
		t.Fatalf("choose: %v %v", n, err)
	}
	if v, _ := n.Sub.Sub.get("Duration"); v != "UntilTheEndOfYourNextTurn" {
		t.Fatalf("duration %q", v)
	}
	// a choice with nothing exiled before it is refused
	ztRefuse(t, []*ir.Step{pick}, nil, "verb:choose")
}

func TestPayCostChain(t *testing.T) {
	sac := &ir.Cost{Parts: []*ir.CostPart{{Kind: ir.CostSacrifice, Obj: &ir.Ref{Kind: ir.RefSelf}}}}
	body := ztProg(&ir.Step{Verb: ir.VerbDraw, N: vc(1)}, &ir.Step{Verb: ir.VerbApplyEffect, Obj: ztRef(ir.RefSelf), Dur: ir.DurEndOfTurn,
		Effects: []*ir.Effect{{Kind: ir.EffCantBeBlocked}}})
	steps := []*ir.Step{{Verb: ir.VerbPayCost, Optional: true, Cost: sac}, {Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CCostPaid}, Then: body}}
	p := &pr{}
	n, err := p.program(ztProg(steps...))
	if err != nil || !n.Ability || n.Params[0].K != "Cost" || n.Sub == nil {
		t.Fatalf("chain: %v %v", n, err)
	}
	if got := p.render("A", n); !strings.HasPrefix(got, "AB$ Draw | Cost$ ") {
		t.Fatalf("render %q", got)
	}
}

func TestTriggerGrant(t *testing.T) {
	dies := &ir.Ability{Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvDies, Subject: &ir.Filter{Subject: ir.SubjPermanent,
		Zones: ir.MakeZoneSet(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PIs, Ref: ztRef(ir.RefSelf)}},
		Body: ztProg(&ir.Step{Verb: ir.VerbDraw, N: vc(1)})}
	tr := &ir.Ref{Kind: ir.RefTarget}
	tgt := []*ir.Target{ztObjTarget(ir.ZoneBattlefield)}
	p := &pr{}
	n, err := p.program(&ir.Program{Targets: tgt, Steps: ztFx(ir.DurEndOfTurn, tr,
		&ir.Effect{Kind: ir.EffAdjustPT, P: vc(2), T: vc(0)}, &ir.Effect{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{dies}})})
	if err != nil || n.API != "Pump" || n.Sub == nil || n.Sub.API != "Animate" {
		t.Fatalf("grant: %v %v", n, err)
	}
	if d, _ := n.Sub.get("Defined"); d != "ParentTarget" {
		t.Fatalf("Defined %q", d)
	}
	if tg, _ := n.Sub.get("Triggers"); tg == "" || !p.hasSVar(tg) {
		t.Fatalf("Triggers %q in %v", tg, p.svars)
	}
}
