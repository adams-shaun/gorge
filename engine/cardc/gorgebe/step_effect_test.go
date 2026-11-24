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
