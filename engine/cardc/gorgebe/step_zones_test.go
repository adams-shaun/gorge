package gorgebe

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// --- builders (zt prefix) ---

func ztRef(k ir.RefKind) *ir.Ref { return &ir.Ref{Kind: k} }

func ztZoneFilter(z ir.Zone, types ...string) *ir.Filter {
	f := &ir.Filter{Subject: ir.SubjCard, Zones: ir.MakeZoneSet(z), Op: ir.FPred, Pred: ir.PAny}
	if len(types) > 0 {
		f.Pred, f.Word = ir.PCardType, types[0]
	}
	return f
}

func ztChoose(f *ir.Filter, n int32) *ir.Ref {
	return &ir.Ref{Kind: ir.RefChoose, Filter: f, N: vc(n)}
}

func ztAll(f *ir.Filter) *ir.Ref { return &ir.Ref{Kind: ir.RefAll, Filter: f} }

func ztTop(n int32) *ir.Ref { return &ir.Ref{Kind: ir.RefTopOfLibrary, N: vc(n)} }

func ztMv(obj *ir.Ref, to ir.Zone) *ir.Step { return &ir.Step{Verb: ir.VerbMove, Obj: obj, To: to} }

func ztProg(steps ...*ir.Step) *ir.Program { return &ir.Program{Steps: steps} }

func ztObjTarget(z ir.Zone) *ir.Target {
	return &ir.Target{Min: 1, Max: 1, Filter: &ir.Filter{Subject: ir.SubjPermanent, Zones: ir.MakeZoneSet(z), Op: ir.FPred, Pred: ir.PCardType, Word: "Creature"}}
}

// ztPrint prints steps as a program in a frame of targets.
func ztPrint(steps []*ir.Step, targets ...*ir.Target) (*node, error) {
	prog := &ir.Program{Steps: steps, Targets: targets}
	return (&pr{}).program(prog)
}

func ztWant(t *testing.T, steps []*ir.Step, targets []*ir.Target, api string, want []kv) {
	t.Helper()
	n, err := ztPrint(steps, targets...)
	if err != nil {
		t.Fatalf("unprintable: %v", err)
	}
	if n.API != api || !reflect.DeepEqual(n.Params, want) {
		t.Fatalf("got %s %v, want %s %v", n.API, n.Params, api, want)
	}
}

func ztRefuse(t *testing.T, steps []*ir.Step, targets []*ir.Target, what string) {
	t.Helper()
	_, err := ztPrint(steps, targets...)
	var u *Unprintable
	if !errors.As(err, &u) || u.What != what {
		t.Fatalf("got %v, want Unprintable %q", err, what)
	}
}

func TestMoveNodes(t *testing.T) {
	tgt := []*ir.Target{ztObjTarget(ir.ZoneGraveyard)}
	tr := &ir.Ref{Kind: ir.RefTarget}
	ztWant(t, []*ir.Step{ztMv(tr, ir.ZoneHand)}, tgt, "ChangeZone", []kv{
		{"Origin", "Graveyard"}, {"Destination", "Hand"}, {"ValidTgts", "Creature"}, {"TgtZone", "Graveyard"}})
	bf := []*ir.Target{ztObjTarget(ir.ZoneBattlefield)}
	ztWant(t, []*ir.Step{ztMv(tr, ir.ZoneHand)}, bf, "ChangeZone", []kv{
		{"Origin", "Battlefield"}, {"Destination", "Hand"}, {"ValidTgts", "Creature"}})
	s := ztMv(tr, ir.ZoneHand)
	s.From = ir.ZoneExile
	ztWant(t, []*ir.Step{s}, bf, "ChangeZone", []kv{
		{"Origin", "Exile"}, {"Destination", "Hand"}, {"ValidTgts", "Creature"}})
	// exile is a move to exile
	ztWant(t, []*ir.Step{{Verb: ir.VerbExile, Obj: tr}}, bf, "ChangeZone", []kv{
		{"Origin", "Battlefield"}, {"Destination", "Exile"}, {"ValidTgts", "Creature"}})
	// self
	ztWant(t, []*ir.Step{ztMv(ztRef(ir.RefSelf), ir.ZoneHand)}, nil, "ChangeZone", []kv{{"Defined", "Self"}, {"Destination", "Hand"}})
	s = ztMv(ztRef(ir.RefSelf), ir.ZoneLibrary)
	s.From, s.Word = ir.ZoneGraveyard, "bottom"
	ztWant(t, []*ir.Step{s}, nil, "ChangeZone", []kv{
		{"Origin", "Graveyard"}, {"Destination", "Library"}, {"LibraryPosition", "-1"}})
	// choose, all
	ztWant(t, []*ir.Step{ztMv(ztChoose(ztZoneFilter(ir.ZoneGraveyard, "Creature"), 2), ir.ZoneHand)}, nil, "ChangeZone", []kv{
		{"Origin", "Graveyard"}, {"Destination", "Hand"}, {"ChangeType", "Creature"}, {"ChangeNum", "2"}, {"Mandatory", "True"}})
	ztWant(t, []*ir.Step{ztMv(ztAll(ztZoneFilter(ir.ZoneGraveyard, "Creature")), ir.ZoneExile)}, nil, "ChangeZoneAll", []kv{
		{"ChangeType", "Creature"}, {"Origin", "Graveyard"}, {"Destination", "Exile"}})
	// mill-exile
	ztWant(t, []*ir.Step{{Verb: ir.VerbExile, Obj: ztTop(3), Who: ztRef(ir.RefYou)}}, nil, "Mill", []kv{
		{"NumCards", "3"}, {"Destination", "Exile"}, {"Defined", "You"}})
	of := ztTop(2)
	of.Of = ztRef(ir.RefYou)
	ztWant(t, []*ir.Step{{Verb: ir.VerbExile, Obj: of}}, nil, "Mill", []kv{
		{"NumCards", "2"}, {"Destination", "Exile"}, {"Defined", "You"}})
	// words
	for w, want := range map[string]kv{"top": {"LibraryPosition", "0"}, "bottom-any-order": {"LibraryPosition", "-1"}, "top-any-order": {"Reorder", "True"}} {
		s = ztMv(ztRef(ir.RefSelf), ir.ZoneLibrary)
		s.Word = w
		ztWant(t, []*ir.Step{s}, nil, "ChangeZone", []kv{{"Defined", "Self"}, {"Destination", "Library"}, want})
	}
	// enter
	s = ztMv(ztRef(ir.RefSelf), ir.ZoneBattlefield)
	s.Enter = &ir.Enter{Tapped: true, UnderOwner: true}
	ztWant(t, []*ir.Step{s}, nil, "ChangeZone", []kv{{"Defined", "Self"}, {"Destination", "Battlefield"}, {"Tapped", "True"}})
	s = ztMv(ztRef(ir.RefSelf), ir.ZoneBattlefield)
	s.Optional = true
	ztWant(t, []*ir.Step{s}, nil, "ChangeZone", []kv{{"Defined", "Self"}, {"Destination", "Battlefield"}, {"OptionalDecider", "You"}})
	// refusals
	s = ztMv(ztRef(ir.RefSelf), ir.ZoneLibrary)
	s.Word = "middle"
	ztRefuse(t, []*ir.Step{s}, nil, "verb:move-word")
	for _, e := range []*ir.Enter{{Counter: "P1P1"}, {Controller: ztRef(ir.RefYou)}, {Effects: []*ir.Effect{{Kind: ir.EffAdjustPT}}}} {
		s = ztMv(ztRef(ir.RefSelf), ir.ZoneBattlefield)
		s.Enter = e
		ztRefuse(t, []*ir.Step{s}, nil, "verb:move-enter")
	}
	ztRefuse(t, []*ir.Step{ztMv(&ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcFound}, ir.ZoneHand)}, nil, "verb:move")
	c := ztChoose(ztZoneFilter(ir.ZoneGraveyard), 1)
	c.Of = ztRef(ir.RefYou)
	c.Of = &ir.Ref{Kind: ir.RefEventPlayer}
	ztRefuse(t, []*ir.Step{ztMv(c, ir.ZoneHand)}, nil, "verb:move")
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbMove, Obj: ztTop(1), To: ir.ZoneHand}}, nil, "verb:move")
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbCopySpell}}, nil, "verb:copy-spell")
}

func ztSearch(to ir.Zone) *ir.Step {
	return &ir.Step{Verb: ir.VerbSearch, Then: ztProg(
		&ir.Step{Verb: ir.VerbChoose, Obj: ztChoose(ztZoneFilter(ir.ZoneLibrary, "Land"), 1)},
		&ir.Step{Verb: ir.VerbMove, Obj: &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcFound}, To: to},
		&ir.Step{Verb: ir.VerbShuffle})}
}

func TestDigNodes(t *testing.T) {
	ztWant(t, []*ir.Step{ztSearch(ir.ZoneHand)}, nil, "ChangeZone", []kv{
		{"Origin", "Library"}, {"Destination", "Hand"}, {"ChangeType", "Land"}, {"ChangeNum", "1"}, {"Mandatory", "True"}})
	s := ztSearch(ir.ZoneBattlefield)
	s.Then.Steps[1].Enter = &ir.Enter{Tapped: true}
	ztWant(t, []*ir.Step{s}, nil, "ChangeZone", []kv{
		{"Origin", "Library"}, {"Destination", "Battlefield"}, {"ChangeType", "Land"}, {"ChangeNum", "1"}, {"Mandatory", "True"}, {"Tapped", "True"}})
	s = ztSearch(ir.ZoneHand)
	s.Then.Steps[2].Optional = true
	ztRefuse(t, []*ir.Step{s}, nil, "verb:search")
	s = ztSearch(ir.ZoneHand)
	s.Then.Steps = s.Then.Steps[:2]
	ztRefuse(t, []*ir.Step{s}, nil, "verb:search")
	s = ztSearch(ir.ZoneHand)
	s.Then.Steps[0].Obj.Filter = ztZoneFilter(ir.ZoneGraveyard)
	ztRefuse(t, []*ir.Step{s}, nil, "verb:search")

	creature := ztZoneFilter(ir.ZoneLibrary, "Creature")
	// look top 4: reveal-to-hand, rest on the bottom
	m1 := ztMv(ztChoose(creature, 1), ir.ZoneHand)
	m1.Optional, m1.Word = true, "reveal"
	m2 := ztMv(nil, ir.ZoneLibrary)
	m2.Word = "bottom-any-order"
	ztWant(t, []*ir.Step{{Verb: ir.VerbLook, Obj: ztTop(4), Then: ztProg(m1, m2)}}, nil, "Dig", []kv{
		{"DigNum", "4"}, {"ChangeNum", "1"}, {"ChangeValid", "Creature"}, {"DestinationZone", "Hand"}, {"DestinationZone2", "Library"},
		{"Optional", "True"}, {"LibraryPosition2", "-1"}})
	// top or graveyard
	m1 = ztMv(ztChoose(nil, 1), ir.ZoneLibrary)
	m1.Word = "top"
	ztWant(t, []*ir.Step{{Verb: ir.VerbLook, Obj: ztTop(2), Then: ztProg(m1, ztMv(nil, ir.ZoneGraveyard))}}, nil, "Dig", []kv{
		{"DigNum", "2"}, {"ChangeNum", "1"}, {"ChangeValid", "Card"}, {"DestinationZone", "Library"}, {"DestinationZone2", "Graveyard"}, {"LibraryPosition", "0"}})
	// reveal any to battlefield
	m1 = ztMv(&ir.Ref{Kind: ir.RefChoose, Filter: ztZoneFilter(ir.ZoneLibrary, "Land")}, ir.ZoneBattlefield)
	m1.Optional = true
	ztWant(t, []*ir.Step{{Verb: ir.VerbReveal, Obj: ztTop(5), Then: ztProg(m1, ztMv(nil, ir.ZoneGraveyard))}}, nil, "Dig", []kv{
		{"DigNum", "5"}, {"ChangeNum", "Any"}, {"ChangeValid", "Land"}, {"DestinationZone", "Battlefield"}, {"DestinationZone2", "Graveyard"},
		{"Optional", "True"}, {"Reveal", "True"}})
	// rearrange, with and without the optional shuffle
	rm := ztMv(nil, ir.ZoneLibrary)
	rm.Word = "top-any-order"
	look := &ir.Step{Verb: ir.VerbLook, Obj: ztTop(3), Then: ztProg(rm)}
	ztWant(t, []*ir.Step{look}, nil, "RearrangeTopOfLibrary", []kv{{"NumCards", "3"}})
	n, err := ztPrint([]*ir.Step{look, {Verb: ir.VerbShuffle, Optional: true}})
	if err != nil || n.API != "RearrangeTopOfLibrary" || !reflect.DeepEqual(n.Params, []kv{{"NumCards", "3"}, {"MayShuffle", "True"}}) || n.Sub != nil {
		t.Fatalf("rearrange+shuffle: %v %+v", err, n)
	}
	n, err = ztPrint([]*ir.Step{look, {Verb: ir.VerbShuffle}})
	if err != nil || n.Sub == nil || n.Sub.API != "Shuffle" {
		t.Fatalf("plain shuffle after rearrange: %v %+v", err, n)
	}
	// refusals
	bad := ztMv(ztChoose(creature, 1), ir.ZoneHand)
	bad.Word = "middle"
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbLook, Obj: ztTop(2), Then: ztProg(bad, ztMv(nil, ir.ZoneGraveyard))}}, nil, "verb:dig-word")
	bad = ztMv(ztChoose(creature, 1), ir.ZoneHand)
	bad.Word = "top"
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbLook, Obj: ztTop(2), Then: ztProg(bad, ztMv(nil, ir.ZoneGraveyard))}}, nil, "verb:dig-word")
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbReveal, Obj: ztTop(2), Then: ztProg(rm)}}, nil, "verb:reveal")
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbLook, Obj: ztTop(2), Then: ztProg(ztMv(nil, ir.ZoneHand))}}, nil, "verb:look")
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbChoose, Obj: ztChoose(creature, 1)}}, nil, "verb:choose")
	// hands and shuffle
	tp := []*ir.Target{{Min: 1, Max: 1, Filter: &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}}}
	ztWant(t, []*ir.Step{{Verb: ir.VerbLookAtHand, Obj: &ir.Ref{Kind: ir.RefTarget}}}, tp, "RevealHand", []kv{{"Look", "True"}, {"ValidTgts", "Player"}})
	ztWant(t, []*ir.Step{{Verb: ir.VerbRevealHand, Obj: ztRef(ir.RefYou)}}, nil, "RevealHand", []kv{{"Defined", "You"}})
	ztWant(t, []*ir.Step{{Verb: ir.VerbShuffle, Who: ztRef(ir.RefYou)}}, nil, "Shuffle", []kv{{"Defined", "You"}})
	ztWant(t, []*ir.Step{{Verb: ir.VerbShuffle}}, nil, "Shuffle", nil)
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbShuffle, Optional: true}}, nil, "verb:shuffle-optional")
}

func TestPermanentNodes(t *testing.T) {
	creatures := &ir.Filter{Subject: ir.SubjPermanent, Zones: ir.MakeZoneSet(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PCardType, Word: "Creature"}
	ztWant(t, []*ir.Step{{Verb: ir.VerbTap, Obj: ztAll(creatures)}}, nil, "TapAll", []kv{{"ValidCards", "Creature"}})
	ztWant(t, []*ir.Step{{Verb: ir.VerbUntap, Obj: ztAll(creatures)}}, nil, "UntapAll", []kv{{"ValidCards", "Creature"}})
	tgt := []*ir.Target{ztObjTarget(ir.ZoneBattlefield)}
	ztWant(t, []*ir.Step{{Verb: ir.VerbTap, Obj: &ir.Ref{Kind: ir.RefTarget}}}, tgt, "Tap", []kv{{"ValidTgts", "Creature"}})
	ztWant(t, []*ir.Step{{Verb: ir.VerbUntap, Obj: ztRef(ir.RefSelf)}}, nil, "Untap", []kv{{"Defined", "Self"}})
	ztWant(t, []*ir.Step{{Verb: ir.VerbSacrifice, Obj: ztRef(ir.RefSelf)}}, nil, "Sacrifice", []kv{{"Defined", "Self"}})
	ztWant(t, []*ir.Step{{Verb: ir.VerbSacrifice, Obj: ztChoose(creatures, 2), Who: ztRef(ir.RefYou)}}, nil, "Sacrifice", []kv{
		{"SacValid", "Creature"}, {"Amount", "2"}, {"Defined", "You"}})
	ztWant(t, []*ir.Step{{Verb: ir.VerbSacrifice, Obj: ztChoose(creatures, 1)}}, nil, "Sacrifice", []kv{{"SacValid", "Creature"}, {"Amount", "1"}})
	c := ztChoose(creatures, 1)
	c.Of = &ir.Ref{Kind: ir.RefEventPlayer}
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbSacrifice, Obj: c}}, nil, "verb:sacrifice")
	ztRefuse(t, []*ir.Step{{Verb: ir.VerbSacrifice, Obj: ztAll(creatures)}}, nil, "verb:sacrifice")
}

func ztKW(names ...string) []*ir.Ability {
	var out []*ir.Ability
	for _, n := range names {
		out = append(out, &ir.Ability{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: n}})
	}
	return out
}

func ztFx(dur ir.Duration, obj *ir.Ref, effs ...*ir.Effect) []*ir.Step {
	return []*ir.Step{{Verb: ir.VerbApplyEffect, Obj: obj, Dur: dur, Effects: effs}}
}

func TestEffectNodes(t *testing.T) {
	self := ztRef(ir.RefSelf)
	ztWant(t, ztFx(ir.DurEndOfTurn, self, &ir.Effect{Kind: ir.EffAdjustPT, P: vc(1), T: vc(0)}), nil, "Pump", []kv{{"NumAtt", "+1"}, {"Defined", "Self"}})
	ztWant(t, ztFx(ir.DurEndOfTurn, self, &ir.Effect{Kind: ir.EffAdjustPT, P: vc(2), T: vc(-1)}, &ir.Effect{Kind: ir.EffAddAbility, Abilities: ztKW("Flying", "Trample")}),
		nil, "Pump", []kv{{"NumAtt", "+2"}, {"NumDef", "-1"}, {"KW", "Flying & Trample"}, {"Defined", "Self"}})
	ztWant(t, ztFx(ir.DurEndOfTurn, self, &ir.Effect{Kind: ir.EffAdjustPT, P: &ir.Value{Kind: ir.VX}, T: vc(0)}), nil, "Pump", []kv{{"NumAtt", "+X"}, {"Defined", "Self"}})
	tgt := []*ir.Target{ztObjTarget(ir.ZoneBattlefield)}
	ztWant(t, ztFx(ir.DurEndOfTurn, &ir.Ref{Kind: ir.RefTarget}, &ir.Effect{Kind: ir.EffAddAbility, Abilities: ztKW("Haste")}), tgt, "Pump", []kv{{"KW", "Haste"}, {"ValidTgts", "Creature"}})
	// animate
	ztWant(t, ztFx(ir.DurPermanent, self,
		&ir.Effect{Kind: ir.EffSetPT, P: vc(4), T: vc(4)},
		&ir.Effect{Kind: ir.EffAddCardType, Words: []string{"Creature"}},
		&ir.Effect{Kind: ir.EffAddSubtype, Words: []string{"Elemental"}},
		&ir.Effect{Kind: ir.EffSetColor, Colors: ir.ColorR | ir.ColorW},
		&ir.Effect{Kind: ir.EffAddAbility, Abilities: ztKW("Haste")},
		&ir.Effect{Kind: ir.EffAllCreatureTypes}), nil, "Animate", []kv{
		{"Power", "4"}, {"Toughness", "4"}, {"Types", "Creature,Elemental"}, {"Colors", "White,Red"}, {"OverwriteColors", "True"},
		{"Keywords", "Haste"}, {"AddAllCreatureTypes", "True"}, {"Defined", "Self"}, {"Duration", "Permanent"}})
	ztWant(t, ztFx(ir.DurEndOfTurn, self, &ir.Effect{Kind: ir.EffSetSubtype, Words: []string{"AssemblyWorker"}}), nil, "Animate", []kv{
		{"Types", "Assembly-Worker"}, {"RemoveCreatureTypes", "True"}, {"Defined", "Self"}})
	for d, want := range map[ir.Duration]string{ir.DurUntilYourNextTurn: "UntilYourNextTurn", ir.DurEndOfCombat: "UntilEndOfCombat"} {
		ztWant(t, ztFx(d, self, &ir.Effect{Kind: ir.EffAdjustPT, P: vc(1), T: vc(1)}), nil, "Pump", []kv{{"NumAtt", "+1"}, {"NumDef", "+1"}, {"Defined", "Self"}, {"Duration", want}})
	}
	// refusals
	pump := &ir.Effect{Kind: ir.EffAdjustPT, P: vc(1), T: vc(1)}
	ztRefuse(t, ztFx(ir.DurWhileActive, self, pump), nil, "effect:dur-while-active")
	ztRefuse(t, ztFx(ir.DurEndOfTurn, ztRef(ir.RefYou), pump), nil, "effect:obj")
	ztRefuse(t, ztFx(ir.DurEndOfTurn, self, pump, &ir.Effect{Kind: ir.EffNoUntap}), nil, "effect:no-untap")
	ztRefuse(t, ztFx(ir.DurEndOfTurn, self, pump, &ir.Effect{Kind: ir.EffSetPT, P: vc(1), T: vc(1)}), nil, "effect:adjust-pt")
	ztRefuse(t, ztFx(ir.DurEndOfTurn, self, &ir.Effect{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: "Ward", N: vc(1)}}}}), nil, "effect:add-ability")
}

func ztCounterUnless(who *ir.Ref, c *ir.Cond, then ...*ir.Step) []*ir.Step {
	return []*ir.Step{
		{Verb: ir.VerbPayCost, Optional: true, Who: who, Cost: &ir.Cost{HasMana: true, Mana: []ir.ManaSym{{Kind: ir.ManaGeneric, N: 3}}}},
		{Verb: ir.VerbIf, Cond: c, Then: ztProg(then...)},
	}
}

func ztNotPaid() *ir.Cond {
	return &ir.Cond{Kind: ir.CNot, Args: []*ir.Cond{{Kind: ir.CCostPaid}}}
}

func TestUnless(t *testing.T) {
	spell := []*ir.Target{ztObjTarget(ir.ZoneBattlefield)}
	ctl := &ir.Ref{Kind: ir.RefControllerOf, Of: &ir.Ref{Kind: ir.RefTarget}}
	tapT := &ir.Step{Verb: ir.VerbTap, Obj: &ir.Ref{Kind: ir.RefTarget}}
	ztWant(t, ztCounterUnless(ctl, ztNotPaid(), tapT), spell, "Tap", []kv{
		{"ValidTgts", "Creature"}, {"UnlessCost", "3"}, {"UnlessPayer", "TargetedController"}})
	ztWant(t, ztCounterUnless(nil, ztNotPaid(), &ir.Step{Verb: ir.VerbSacrifice, Obj: ztRef(ir.RefSelf)}), nil, "Sacrifice", []kv{
		{"Defined", "Self"}, {"UnlessCost", "3"}, {"UnlessPayer", "You"}})
	ztWant(t, ztCounterUnless(nil, &ir.Cond{Kind: ir.CCostPaid}, &ir.Step{Verb: ir.VerbUntap, Obj: ztRef(ir.RefSelf)}), nil, "Untap", []kv{
		{"Defined", "Self"}, {"UnlessCost", "3"}, {"UnlessPayer", "You"}, {"UnlessSwitched", "True"}})
	two := ztCounterUnless(nil, ztNotPaid(), &ir.Step{Verb: ir.VerbTap, Obj: ztRef(ir.RefSelf)}, &ir.Step{Verb: ir.VerbUntap, Obj: ztRef(ir.RefSelf)})
	ztRefuse(t, two, nil, "verb:pay-cost-chain")
	ztRefuse(t, two[:1], nil, "verb:pay-cost")
	els := ztCounterUnless(nil, ztNotPaid(), tapT)
	els[1].Else = ztProg(tapT)
	ztRefuse(t, els, spell, "verb:pay-cost")
	notOpt := ztCounterUnless(nil, ztNotPaid(), tapT)
	notOpt[0].Optional = false
	ztRefuse(t, notOpt, spell, "verb:pay-cost")
}

func TestCostText(t *testing.T) {
	self := ztRef(ir.RefSelf)
	hand := ztZoneFilter(ir.ZoneHand, "Creature")
	grave := ztZoneFilter(ir.ZoneGraveyard, "Creature")
	part := func(k ir.CostKind, o *ir.Ref, n *ir.Value, w string) *ir.CostPart {
		return &ir.CostPart{Kind: k, Obj: o, N: n, Word: w}
	}
	cases := []struct {
		p    *ir.CostPart
		want string
	}{
		{part(ir.CostTapSelf, nil, nil, ""), "T"},
		{part(ir.CostUntapSelf, nil, nil, ""), "Q"},
		{part(ir.CostSacrifice, self, nil, ""), "Sac<1/CARDNAME>"},
		{part(ir.CostSacrifice, ztChoose(ztZoneFilter(ir.ZoneBattlefield, "Creature"), 2), nil, ""), "Sac<2/Creature>"},
		{part(ir.CostDiscard, ztChoose(hand, 1), nil, ""), "Discard<1/Creature>"},
		{part(ir.CostDiscard, self, nil, ""), "Discard<1/CARDNAME>"},
		{part(ir.CostPayLife, nil, vc(3), ""), "PayLife<3>"},
		{part(ir.CostPayLife, nil, &ir.Value{Kind: ir.VX}, ""), "PayLife<X>"},
		{part(ir.CostExile, self, nil, ""), "Exile<1/CARDNAME>"},
		{part(ir.CostExile, ztChoose(hand, 1), nil, ""), "ExileFromHand<1/Creature>"},
		{part(ir.CostExile, ztChoose(grave, 1), nil, ""), "ExileFromGrave<1/Creature>"},
		{part(ir.CostRemoveCounters, self, vc(2), "P1P1"), "SubCounter<2/P1P1>"},
		{part(ir.CostReturnToHand, self, nil, ""), "Return<1/CARDNAME>"},
		{part(ir.CostReturnToHand, ztChoose(ztZoneFilter(ir.ZoneBattlefield, "Creature"), 1), nil, ""), "Return<1/Creature>"},
		{part(ir.CostDiscard, ztAll(ztZoneFilter(ir.ZoneHand)), nil, ""), "Discard<0/Hand>"},
		{part(ir.CostRemoveCounters, ztAll(ztZoneFilter(ir.ZoneBattlefield, "Creature")), vc(6), ""), "RemoveAnyCounter<6/Any/Creature>"},
		{part(ir.CostTapUntapped, ztChoose(ztZoneFilter(ir.ZoneBattlefield, "Creature"), 1), nil, ""), "tapXType<1/Creature>"},
		{part(ir.CostReveal, ztChoose(hand, 1), nil, ""), "Reveal<1/Creature>"},
		{part(ir.CostMill, nil, vc(2), ""), "Mill<2>"},
		{part(ir.CostLoyalty, nil, vc(2), ""), "AddCounter<2/LOYALTY>"},
		{part(ir.CostLoyalty, nil, vc(0), ""), "AddCounter<0/LOYALTY>"},
		{part(ir.CostLoyalty, nil, vc(-3), ""), "SubCounter<3/LOYALTY>"},
	}
	for _, c := range cases {
		got, err := (&pr{}).costText(&ir.Cost{Parts: []*ir.CostPart{c.p}})
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v, want %q", c.p.Kind, got, err, c.want)
		}
	}
	mixed := &ir.Cost{HasMana: true, Mana: []ir.ManaSym{{Kind: ir.ManaGeneric, N: 1}}, Parts: []*ir.CostPart{part(ir.CostTapSelf, nil, nil, ""), part(ir.CostSacrifice, self, nil, "")}}
	if got, err := (&pr{}).costText(mixed); err != nil || got != "1 T Sac<1/CARDNAME>" {
		t.Errorf("mixed: %q, %v", got, err)
	}
	refuse := map[string]*ir.Cost{
		"cost:empty":           {},
		"cost:loyalty":         {Parts: []*ir.CostPart{part(ir.CostLoyalty, nil, &ir.Value{Kind: ir.VX}, "")}},
		"cost:discard":         {Parts: []*ir.CostPart{part(ir.CostDiscard, ztAll(hand), nil, "")}},
		"cost:remove-counters": {Parts: []*ir.CostPart{part(ir.CostRemoveCounters, ztChoose(hand, 1), vc(1), "P1P1")}},
		"cost:return-to-hand":  {Parts: []*ir.CostPart{part(ir.CostReturnToHand, ztAll(hand), nil, "")}},
		"cost:none":            {Parts: []*ir.CostPart{{}}},
	}
	for what, c := range refuse {
		_, err := (&pr{}).costText(c)
		var u *Unprintable
		if !errors.As(err, &u) || u.What != what {
			t.Errorf("%s: got %v", what, err)
		}
	}
}

func TestCostsAttested(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	heads := map[string]bool{}
	apis := map[string]bool{}
	var sa func(a *cards.SA)
	sa = func(a *cards.SA) {
		for ; a != nil; a = a.Sub {
			for _, tok := range strings.Fields(a.Params["Cost"]) {
				if h, _, ok := strings.Cut(tok, "<"); ok {
					heads[h] = true
				}
			}
		}
	}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, a := range f.Abilities {
				sa(a)
			}
		}
	}
	for _, h := range []string{"Sac", "Discard", "PayLife", "Exile", "ExileFromHand", "ExileFromGrave", "SubCounter", "Return", "tapXType", "Reveal", "Mill", "AddCounter"} {
		if !heads[h] {
			t.Errorf("cost head %s is in no corpus ability", h)
		}
	}
	sup := effects.Supported()
	for _, api := range []string{"ChangeZone", "ChangeZoneAll", "Mill", "Dig", "RearrangeTopOfLibrary", "RevealHand", "Shuffle",
		"TapAll", "UntapAll", "Tap", "Untap", "Sacrifice", "Pump", "Animate"} {
		apis[api] = true
		if !sup["api:"+api] {
			t.Errorf("api %s is not supported by the engine", api)
		}
	}
}
