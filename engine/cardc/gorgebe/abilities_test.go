package gorgebe

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

var bfZones = ir.MakeZoneSet(ir.ZoneBattlefield)

func abSelf() *ir.Filter {
	return &ir.Filter{Subject: ir.SubjPermanent, Zones: bfZones, Op: ir.FPred, Pred: ir.PIs, Ref: &ir.Ref{Kind: ir.RefSelf}}
}

func abYou() *ir.Filter {
	return &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PPlayerIs, Ref: &ir.Ref{Kind: ir.RefYou}}
}

func abTyped(subj ir.Subject, word string) *ir.Filter {
	return &ir.Filter{Subject: subj, Op: ir.FPred, Pred: ir.PCardType, Word: word}
}

func abCreatureYouCtrl() *ir.Filter {
	ctrl := &ir.Filter{Op: ir.FPred, Pred: ir.PControlledBy, Ref: &ir.Ref{Kind: ir.RefYou}}
	return &ir.Filter{Subject: ir.SubjPermanent, Zones: bfZones, Op: ir.FAll, Args: []*ir.Filter{abTyped(ir.SubjPermanent, "Creature"), ctrl}}
}

func abOppsRef() *ir.Ref { return &ir.Ref{Kind: ir.RefAll, Filter: ctOpp()} }

func abBody(steps ...*ir.Step) *ir.Program { return &ir.Program{Steps: steps} }

func abDraw() *ir.Program { return abBody(&ir.Step{Verb: ir.VerbDraw, N: vc(1)}) }

func abGenericCost(n int32) *ir.Cost {
	return &ir.Cost{HasMana: true, Mana: []ir.ManaSym{{Kind: ir.ManaGeneric, N: n}}}
}

// abRun prints a with a fresh printer.
func abRun(a *ir.Ability) (*pr, error) {
	p := &pr{}
	return p, abilityFns[a.Kind](p, a)
}

// abParams is the "K$ V" items of a script line, without its "T:" style prefix.
func abParams(line string) []string {
	return strings.Split(line[2:], " | ")
}

func abWhat(t *testing.T, name string, err error, what string) {
	t.Helper()
	var u *Unprintable
	if !errors.As(err, &u) || u.What != what {
		t.Errorf("%s: err %v, want unprintable %s", name, err, what)
	}
}

func abTrig(ev ir.EventKind, subj, obj *ir.Filter) *ir.Trigger {
	return &ir.Trigger{Event: ev, Subject: subj, Object: obj}
}

func TestActivated(t *testing.T) {
	green := &ir.Step{Verb: ir.VerbAddMana, Mana: []ir.ManaSym{{Kind: ir.ManaColored, Colors: ir.ColorG}}}
	tap := &ir.Cost{Parts: []*ir.CostPart{{Kind: ir.CostTapSelf}}}
	sac := &ir.Cost{Parts: []*ir.CostPart{{Kind: ir.CostSacrifice, Obj: &ir.Ref{Kind: ir.RefSelf}}}}
	dmg := &ir.Program{
		Steps:   []*ir.Step{{Verb: ir.VerbDealDamage, N: vc(2), Obj: &ir.Ref{Kind: ir.RefTarget}}},
		Targets: []*ir.Target{{Min: 1, Max: 1, Filter: abTyped(ir.SubjPermanent, "Creature")}},
	}
	raid := ctRef(ctYou(), ctPred(ir.PAttackedThisTurn))
	loyalty := &ir.Cost{Parts: []*ir.CostPart{{Kind: ir.CostLoyalty, N: vc(-2)}}}
	cases := []struct {
		name string
		a    *ir.Ability
		want []string
		what string
	}{
		{"mana", &ir.Ability{Kind: ir.AbActivated, Cost: tap, Body: abBody(green)}, []string{"Cost$ T", "Produced$ G"}, ""},
		{"sacrifice damage", &ir.Ability{Kind: ir.AbActivated, Cost: sac, Body: dmg},
			[]string{"Cost$ Sac<1/CARDNAME>", "NumDmg$ 2", "ValidTgts$ Creature"}, ""},
		{"sorcery once", &ir.Ability{Kind: ir.AbActivated, Cost: abGenericCost(2), Body: abDraw(), SorceryOnly: true, Once: true},
			[]string{"Cost$ 2", "NumCards$ 1", "SorcerySpeed$ True", "ActivationLimit$ 1"}, ""},
		{"graveyard", &ir.Ability{Kind: ir.AbActivated, Cost: abGenericCost(1), Body: abDraw(), Zones: ir.MakeZoneSet(ir.ZoneGraveyard, ir.ZoneHand)},
			[]string{"Cost$ 1", "NumCards$ 1", "ActivationZone$ Hand,Graveyard"}, ""},
		{"battlefield only", &ir.Ability{Kind: ir.AbActivated, Cost: abGenericCost(1), Body: abDraw(), Zones: bfZones},
			[]string{"Cost$ 1", "NumCards$ 1"}, ""},
		{"raid", &ir.Ability{Kind: ir.AbActivated, Cost: abGenericCost(1), Body: abDraw(), Cond: raid},
			[]string{"Cost$ 1", "NumCards$ 1", "CheckSVar$ N", "SVarCompare$ GE1"}, ""},
		{"loyalty", &ir.Ability{Kind: ir.AbActivated, Cost: loyalty, Body: abDraw(), SorceryOnly: true},
			[]string{"Cost$ SubCounter<2/LOYALTY>", "NumCards$ 1", "SorcerySpeed$ True", "Planeswalker$ True"}, ""},
		{"instant only", &ir.Ability{Kind: ir.AbActivated, Cost: tap, Body: abDraw(), InstantOnly: true}, nil, "activated:instant-only"},
		{"no cost", &ir.Ability{Kind: ir.AbActivated, Body: abDraw()}, nil, "activated:no-cost"},
		{"empty cost", &ir.Ability{Kind: ir.AbActivated, Cost: &ir.Cost{}, Body: abDraw()}, nil, "cost:empty"},
		{"empty body", &ir.Ability{Kind: ir.AbActivated, Cost: tap}, nil, "program:empty"},
	}
	for _, c := range cases {
		p, err := abRun(c.a)
		if c.what != "" {
			abWhat(t, c.name, err, c.what)
			continue
		}
		if err != nil || len(p.lines) != 1 || !strings.HasPrefix(p.lines[0], "A:AB$ ") {
			t.Errorf("%s: %v %q", c.name, err, p.lines)
			continue
		}
		got := abParams(p.lines[0])
		if got[0] = strings.TrimPrefix(got[0], "AB$ "); got[0] == "" {
			t.Errorf("%s: no API", c.name)
		}
		if !reflect.DeepEqual(got[1:], c.want) {
			t.Errorf("%s: params %q, want %q", c.name, got[1:], c.want)
		}
	}
}

func TestTriggerParams(t *testing.T) {
	cr := abCreatureYouCtrl()
	threshold := ctRef(ctYou(), ctCmpPred(ir.PCardsInGraveyard, ir.CmpGE, 7))
	present := ctRef(&ir.Ref{Kind: ir.RefSelf}, abTyped(ir.SubjPermanent, "Creature"))
	spellCreature := abTyped(ir.SubjSpell, "Creature")
	oppSpell := &ir.Filter{Subject: ir.SubjSpell, Op: ir.FPred, Pred: ir.PControlledBy, Ref: abOppsRef()}
	eq := func(op ir.CmpOp, n int32) *ir.Compare { return &ir.Compare{Op: op, V: vc(n)} }
	with := func(t *ir.Trigger, f func(*ir.Trigger)) *ir.Trigger { f(t); return t }
	ex := "Execute$ TrigDraw"
	cases := []struct {
		name string
		tr   *ir.Trigger
		a    func(*ir.Ability)
		want [][]string
		what string
	}{
		{"enters self", abTrig(ir.EvEnters, abSelf(), nil), nil,
			[][]string{{"Mode$ ChangesZone", "Origin$ Any", "Destination$ Battlefield", "ValidCard$ Card.Self", ex}}, ""},
		{"enters other", abTrig(ir.EvEnters, cr, nil), nil,
			[][]string{{"Mode$ ChangesZone", "Destination$ Battlefield", "ValidCard$ Creature.YouCtrl", "TriggerZones$ Battlefield", ex}}, ""},
		{"dies self", abTrig(ir.EvDies, abSelf(), nil), nil,
			[][]string{{"Mode$ ChangesZone", "Origin$ Battlefield", "Destination$ Graveyard", "ValidCard$ Card.Self", ex}}, ""},
		{"put into graveyard self", abTrig(ir.EvPutIntoGraveyard, abSelf(), nil), nil,
			[][]string{{"Mode$ ChangesZone", "Origin$ Battlefield", "Destination$ Graveyard", "ValidCard$ Card.Self", ex}}, ""},
		{"dies other", abTrig(ir.EvDies, cr, nil), nil,
			[][]string{{"Mode$ ChangesZone", "Origin$ Battlefield", "Destination$ Graveyard", "ValidCard$ Creature.YouCtrl", "TriggerZones$ Battlefield", ex}}, ""},
		{"leaves self", abTrig(ir.EvLeaves, abSelf(), nil), nil,
			[][]string{{"Mode$ ChangesZone", "Origin$ Battlefield", "Destination$ Any", "ValidCard$ Card.Self", ex}}, ""},
		{"leaves other", abTrig(ir.EvLeaves, cr, nil), nil, nil, "event:leaves"},
		{"attacks self", abTrig(ir.EvAttacks, abSelf(), nil), nil, [][]string{{"Mode$ Attacks", "ValidCard$ Card.Self", ex}}, ""},
		{"attacks other", abTrig(ir.EvAttacks, cr, nil), nil,
			[][]string{{"Mode$ Attacks", "ValidCard$ Creature.YouCtrl", "TriggerZones$ Battlefield", ex}}, ""},
		{"upkeep you", abTrig(ir.EvUpkeep, abYou(), nil), nil,
			[][]string{{"Mode$ Phase", "Phase$ Upkeep", "ValidPlayer$ You", "TriggerZones$ Battlefield", ex}}, ""},
		{"end step opponent", abTrig(ir.EvEndStep, ctOpp(), nil), nil,
			[][]string{{"Mode$ Phase", "Phase$ End of Turn", "ValidPlayer$ Opponent", "TriggerZones$ Battlefield", ex}}, ""},
		{"begin combat any player", abTrig(ir.EvBeginCombat, nil, nil), nil,
			[][]string{{"Mode$ Phase", "Phase$ BeginCombat", "TriggerZones$ Battlefield", ex}}, ""},
		{"draws you", abTrig(ir.EvDraws, abYou(), nil), nil,
			[][]string{{"Mode$ Drawn", "ValidCard$ Card.YouCtrl", "TriggerZones$ Battlefield", ex}}, ""},
		{"draws opponent second", with(abTrig(ir.EvDraws, ctOpp(), nil), func(x *ir.Trigger) { x.Cmp = eq(ir.CmpEQ, 2) }), nil,
			[][]string{{"Mode$ Drawn", "ValidCard$ Card.OppCtrl", "TriggerZones$ Battlefield", "Number$ 2", ex}}, ""},
		{"draws cmp", with(abTrig(ir.EvDraws, abYou(), nil), func(x *ir.Trigger) { x.Cmp = eq(ir.CmpGE, 2) }), nil, nil, "event:cmp"},
		{"gains life first", with(abTrig(ir.EvGainsLife, abYou(), nil), func(x *ir.Trigger) { x.Cmp = eq(ir.CmpEQ, 1) }), nil,
			[][]string{{"Mode$ LifeGained", "ValidPlayer$ You", "TriggerZones$ Battlefield", "FirstTime$ True", ex}}, ""},
		{"loses life", abTrig(ir.EvLosesLife, ctOpp(), nil), nil,
			[][]string{{"Mode$ LifeLost", "ValidPlayer$ Opponent", "TriggerZones$ Battlefield", ex}}, ""},
		{"gains life cmp", with(abTrig(ir.EvGainsLife, abYou(), nil), func(x *ir.Trigger) { x.Cmp = eq(ir.CmpEQ, 2) }), nil, nil, "event:cmp"},
		{"cast self", abTrig(ir.EvCast, nil, abSelf()), nil, [][]string{{"Mode$ SpellCast", "ValidCard$ Card.Self", ex}}, ""},
		{"cast other", abTrig(ir.EvCast, abYou(), spellCreature), nil,
			[][]string{{"Mode$ SpellCast", "ValidCard$ Creature", "ValidActivatingPlayer$ You", "TriggerZones$ Battlefield", ex}}, ""},
		{"combat damage self", abTrig(ir.EvDealsCombatDamageToPlayer, abSelf(), nil), nil,
			[][]string{{"Mode$ DamageDone", "ValidSource$ Card.Self", "ValidTarget$ Player", "CombatDamage$ True", ex}}, ""},
		{"noncombat damage other", abTrig(ir.EvDealsNoncombatDamageToPlayer, cr, ctOpp()), nil,
			[][]string{{"Mode$ DamageDone", "ValidSource$ Creature.YouCtrl", "ValidTarget$ Opponent", "CombatDamage$ False", "TriggerZones$ Battlefield", ex}}, ""},
		{"becomes target", abTrig(ir.EvBecomesTarget, abSelf(), oppSpell), nil,
			[][]string{{"Mode$ BecomesTarget", "ValidTarget$ Card.Self", "ValidSource$ Spell.OppCtrl", "TriggerZones$ Battlefield", ex}}, ""},
		{"counters put", with(abTrig(ir.EvCountersPut, abYou(), abSelf()), func(x *ir.Trigger) { x.Word = "P1P1" }), nil,
			[][]string{{"Mode$ CounterAddedOnce", "CounterType$ P1P1", "ValidCard$ Permanent.Self", "ValidSource$ You", "TriggerZones$ Battlefield", ex}}, ""},
		{"or", &ir.Trigger{Event: ir.EvOr, Args: []*ir.Trigger{abTrig(ir.EvEnters, abSelf(), nil), abTrig(ir.EvDies, abSelf(), nil)}}, nil,
			[][]string{
				{"Mode$ ChangesZone", "Origin$ Any", "Destination$ Battlefield", "ValidCard$ Card.Self", ex},
				{"Mode$ ChangesZone", "Origin$ Battlefield", "Destination$ Graveyard", "ValidCard$ Card.Self", "Secondary$ True", ex},
			}, ""},
		{"or mixed", &ir.Trigger{Event: ir.EvOr, Args: []*ir.Trigger{abTrig(ir.EvEnters, abSelf(), nil), abTrig(ir.EvAttacks, cr, nil)}}, nil, nil, "event:or-mixed"},
		{"or mixed enters", &ir.Trigger{Event: ir.EvOr, Args: []*ir.Trigger{abTrig(ir.EvEnters, abSelf(), nil), abTrig(ir.EvEnters, cr, nil)}}, nil,
			[][]string{{"Mode$ ChangesZone", "Origin$ Any", "Destination$ Battlefield", "ValidCard$ Card.Self,Creature.YouCtrl", ex}}, ""},
		{"or mixed dies", &ir.Trigger{Event: ir.EvOr, Args: []*ir.Trigger{abTrig(ir.EvDies, abSelf(), nil), abTrig(ir.EvDies, cr, nil)}}, nil,
			[][]string{{"Mode$ ChangesZone", "Origin$ Battlefield", "Destination$ Graveyard", "ValidCard$ Card.Self,Creature.YouCtrl", "TriggerZones$ Battlefield", ex}}, ""},
		{"combat damage other", abTrig(ir.EvDealsCombatDamage, cr, nil), nil,
			[][]string{{"Mode$ DamageDealtOnce", "CombatDamage$ True", "ValidSource$ Creature.YouCtrl", "TriggerZones$ Battlefield", ex}}, ""},
		{"combat damage self any target", abTrig(ir.EvDealsCombatDamage, abSelf(), nil), nil,
			[][]string{{"Mode$ DamageDealtOnce", "CombatDamage$ True", "ValidSource$ Card.Self", ex}}, ""},
		{"you attack", abTrig(ir.EvPlayerAttacks, abYou(), nil), nil,
			[][]string{{"Mode$ AttackersDeclared", "AttackingPlayer$ You", "TriggerZones$ Battlefield", ex}}, ""},
		{"you attack with three", with(abTrig(ir.EvPlayerAttacks, abYou(), cr), func(x *ir.Trigger) { x.Cmp = eq(ir.CmpGE, 3) }), nil,
			[][]string{{"Mode$ AttackersDeclared", "ValidAttackers$ Creature.YouCtrl", "ValidAttackersAmount$ GE3", "AttackingPlayer$ You", "TriggerZones$ Battlefield", ex}}, ""},
		{"you play another land", abTrig(ir.EvPlaysLand, abYou(), cr), nil,
			[][]string{{"Mode$ LandPlayed", "ValidCard$ Creature.YouCtrl", "TriggerZones$ Battlefield", ex}}, ""},
		{"trigger zones", abTrig(ir.EvEnters, cr, nil), func(a *ir.Ability) { a.Zones = ir.MakeZoneSet(ir.ZoneGraveyard) },
			[][]string{{"Mode$ ChangesZone", "Destination$ Battlefield", "ValidCard$ Creature.YouCtrl", "TriggerZones$ Graveyard", ex}}, ""},
		{"self trigger zones", abTrig(ir.EvEnters, abSelf(), nil), func(a *ir.Ability) { a.Zones = ir.MakeZoneSet(ir.ZoneGraveyard) },
			[][]string{{"Mode$ ChangesZone", "Origin$ Any", "Destination$ Battlefield", "ValidCard$ Card.Self", "TriggerZones$ Graveyard", ex}}, ""},
		{"once per turn", with(abTrig(ir.EvAttacks, abSelf(), nil), func(x *ir.Trigger) { x.OncePerTurn = true }), nil,
			[][]string{{"Mode$ Attacks", "ValidCard$ Card.Self", "ActivationLimit$ 1", ex}}, ""},
		{"threshold condition", abTrig(ir.EvAttacks, abSelf(), nil), func(a *ir.Ability) { a.Cond = threshold },
			[][]string{{"Mode$ Attacks", "ValidCard$ Card.Self", "Threshold$ True", ex}}, ""},
		{"trigger condition", with(abTrig(ir.EvAttacks, abSelf(), nil), func(x *ir.Trigger) { x.Cond = present }), nil,
			[][]string{{"Mode$ Attacks", "ValidCard$ Card.Self", "IsPresent$ Creature.Self", ex}}, ""},
		{"two conditions", with(abTrig(ir.EvAttacks, abSelf(), nil), func(x *ir.Trigger) { x.Cond = present }),
			func(a *ir.Ability) { a.Cond = threshold }, nil, "event:two-conds"},
		{"attacks cmp", with(abTrig(ir.EvAttacks, abSelf(), nil), func(x *ir.Trigger) { x.Cmp = eq(ir.CmpGE, 2) }), nil, nil, "event:cmp"},
		{"enters word", with(abTrig(ir.EvEnters, abSelf(), nil), func(x *ir.Trigger) { x.Word = "combat" }), nil, nil, "event:word"},
		{"untaps", abTrig(ir.EvUntaps, abSelf(), nil), nil, nil, "event:untaps"},
		{"next end step", &ir.Trigger{Event: ir.EvNextEndStep}, nil, nil, "event:next-end-step"},
	}
	for _, c := range cases {
		a := &ir.Ability{Kind: ir.AbTriggered, Trigger: c.tr, Body: abDraw()}
		if c.a != nil {
			c.a(a)
		}
		p, err := abRun(a)
		if c.what != "" {
			abWhat(t, c.name, err, c.what)
			continue
		}
		if err != nil || len(p.lines) != len(c.want) {
			t.Errorf("%s: %v %q", c.name, err, p.lines)
			continue
		}
		for i, l := range p.lines {
			if !strings.HasPrefix(l, "T:") || !reflect.DeepEqual(abParams(l), c.want[i]) {
				t.Errorf("%s line %d: %q, want %q", c.name, i, abParams(l), c.want[i])
			}
		}
		if len(p.svars) == 0 || p.svars[0].K != "TrigDraw" || p.ev != ir.EvNone || p.evSelf {
			t.Errorf("%s: svars %v, ev %v, evSelf %v", c.name, p.svars, p.ev, p.evSelf)
		}
	}
}

// abStatic is a static ability applying effs to obj for as long as it is active.
func abStatic(obj *ir.Ref, effs ...*ir.Effect) *ir.Ability {
	return &ir.Ability{Kind: ir.AbStatic, Body: abBody(&ir.Step{Verb: ir.VerbApplyEffect, Obj: obj, Dur: ir.DurWhileActive, Effects: effs})}
}

func TestStatics(t *testing.T) {
	self := &ir.Ref{Kind: ir.RefSelf}
	you := &ir.Ref{Kind: ir.RefYou}
	creatures := &ir.Ref{Kind: ir.RefAll, Filter: abCreatureYouCtrl()}
	pt := func(p, t int32) *ir.Effect { return &ir.Effect{Kind: ir.EffAdjustPT, P: vc(p), T: vc(t)} }
	grant := func(names ...string) *ir.Effect {
		e := &ir.Effect{Kind: ir.EffAddAbility}
		for _, n := range names {
			e.Abilities = append(e.Abilities, kw(n))
		}
		return e
	}
	kind := func(k ir.EffectKind) *ir.Effect { return &ir.Effect{Kind: k} }
	anySpell := &ir.Filter{Subject: ir.SubjSpell, Op: ir.FPred, Pred: ir.PAny}
	cost := func(k ir.EffectKind, f *ir.Filter) *ir.Effect {
		return &ir.Effect{Kind: k, Cost: abGenericCost(1), Filter: f}
	}
	withCond := func(a *ir.Ability, c *ir.Cond) *ir.Ability { a.Cond = c; return a }
	stack := func(a *ir.Ability) *ir.Ability { a.Zones = ir.MakeZoneSet(ir.ZoneStack); return a }
	threshold := ctRef(ctYou(), ctCmpPred(ir.PCardsInGraveyard, ir.CmpGE, 7))
	presentSelf := ctRef(&ir.Ref{Kind: ir.RefSelf}, abTyped(ir.SubjPermanent, "Creature"))
	cases := []struct {
		name string
		a    *ir.Ability
		line string // "S:" or "R:"
		want []string
		what string
	}{
		{"pump self", abStatic(self, pt(2, 0)), "S:", []string{"Mode$ Continuous", "Affected$ Card.Self", "AddPower$ 2"}, ""},
		{"anthem and keywords", abStatic(creatures, pt(1, 1), grant("Flying", "Lifelink")), "S:",
			[]string{"Mode$ Continuous", "Affected$ Creature.YouCtrl", "AddPower$ 1", "AddToughness$ 1", "AddKeyword$ Flying & Lifelink"}, ""},
		{"keywords under threshold", withCond(abStatic(self, grant("Flying")), threshold), "S:",
			[]string{"Mode$ Continuous", "Affected$ Card.Self", "AddKeyword$ Flying", "CheckSVar$ N", "SVarCompare$ GE7"}, ""},
		{"pump under present test", withCond(abStatic(self, pt(0, 2)), presentSelf), "S:",
			[]string{"Mode$ Continuous", "Affected$ Card.Self", "AddToughness$ 2", "IsPresent$ Creature.Self", "PresentCompare$ GE1"}, ""},
		{"cant block", abStatic(self, kind(ir.EffCantBlock)), "S:", []string{"Mode$ CantBlock", "ValidCard$ Card.Self"}, ""},
		{"cant be blocked", abStatic(self, kind(ir.EffCantBeBlocked)), "S:", []string{"Mode$ CantBlockBy", "ValidAttacker$ Card.Self"}, ""},
		{"no max hand size", abStatic(you, kind(ir.EffNoMaxHandSize)), "S:", []string{"Mode$ Continuous", "Affected$ You", "SetMaxHandSize$ Unlimited"}, ""},
		{"flash", abStatic(you, &ir.Effect{Kind: ir.EffFlash, Filter: abTyped(ir.SubjSpell, "Creature")}), "S:",
			[]string{"Mode$ CastWithFlash", "ValidCard$ Creature", "ValidSA$ Spell", "Caster$ You"}, ""},
		{"cast without paying", abStatic(you, &ir.Effect{Kind: ir.EffCastWithoutPaying, Filter: anySpell}), "S:",
			[]string{"Mode$ Continuous", "Affected$ Card.nonLand+YouOwn", "MayPlay$ True", "MayPlayWithoutManaCost$ True", "AffectedZone$ Hand"}, ""},
		{"spells cost more", abStatic(abOppsRef(), cost(ir.EffSpellsCostMore, anySpell)), "S:",
			[]string{"Mode$ RaiseCost", "Activator$ Opponent", "Type$ Spell", "Amount$ 1"}, ""},
		{"creature spells cost less", abStatic(&ir.Ref{Kind: ir.RefAll, Filter: abYou()}, cost(ir.EffSpellsCostLess, abTyped(ir.SubjSpell, "Creature"))), "S:",
			[]string{"Mode$ ReduceCost", "Activator$ You", "Type$ Spell", "Amount$ 1", "ValidCard$ Creature"}, ""},
		{"alternative cost", abStatic(self, &ir.Effect{Kind: ir.EffAlternativeCost, Cost: abGenericCost(3)}), "S:",
			[]string{"Mode$ AlternativeCost", "ValidSA$ Spell.Self", "EffectZone$ All", "Cost$ 3"}, ""},
		{"cant lose", abStatic(you, kind(ir.EffCantLose)), "R:",
			[]string{"Event$ GameLoss", "ActiveZones$ Battlefield", "ValidPlayer$ You", "Layer$ CantHappen"}, ""},
		{"cant win", abStatic(abOppsRef(), kind(ir.EffCantWin)), "R:",
			[]string{"Event$ GameWin", "ActiveZones$ Battlefield", "ValidPlayer$ Opponent", "Layer$ CantHappen"}, ""},
		{"cant be countered", stack(abStatic(self, kind(ir.EffCantBeCountered))), "R:",
			[]string{"Event$ Counter", "ValidCard$ Card.Self", "ValidSA$ Spell", "Layer$ CantHappen"}, ""},
		{"cant be countered off the stack", abStatic(self, kind(ir.EffCantBeCountered)), "", nil, "static:cant-be-countered"},
		{"cant lose of self", abStatic(self, kind(ir.EffCantLose)), "", nil, "static:cant-lose"},
		{"additional cost", abStatic(self, &ir.Effect{Kind: ir.EffAdditionalCost, Cost: abGenericCost(1)}), "", nil, "static:additional-cost"},
		{"cost more with a non-generic cost", abStatic(abOppsRef(), &ir.Effect{Kind: ir.EffSpellsCostMore, Cost: &ir.Cost{Parts: []*ir.CostPart{{Kind: ir.CostTapSelf}}}}), "", nil, "static:spells-cost-more"},
		{"permanent duration", &ir.Ability{Kind: ir.AbStatic, Body: abBody(&ir.Step{Verb: ir.VerbApplyEffect, Obj: self, Dur: ir.DurPermanent, Effects: []*ir.Effect{pt(1, 1)}})}, "", nil, "static:body"},
		{"two steps", &ir.Ability{Kind: ir.AbStatic, Body: abBody(&ir.Step{Verb: ir.VerbDraw}, &ir.Step{Verb: ir.VerbDraw})}, "", nil, "static:body"},
		{"mixed effects", abStatic(self, pt(1, 1), kind(ir.EffCantBlock)), "", nil, "static:body"},
		{"granted keyword with a cost", abStatic(self, &ir.Effect{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: "Ward", Cost: abGenericCost(1)}}}}), "", nil, "static:add-ability"},
	}
	for _, c := range cases {
		p, err := abRun(c.a)
		if c.what != "" {
			abWhat(t, c.name, err, c.what)
			continue
		}
		if err != nil || len(p.lines) != 1 || !strings.HasPrefix(p.lines[0], c.line) {
			t.Errorf("%s: %v %q", c.name, err, p.lines)
			continue
		}
		if got := abParams(p.lines[0]); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: params %q, want %q", c.name, got, c.want)
		}
	}
}

func TestReplacements(t *testing.T) {
	selfRef := &ir.Ref{Kind: ir.RefSelf}
	enter := func(e *ir.Enter) *ir.Program { return abBody(&ir.Step{Verb: ir.VerbModifyEnter, Enter: e}) }
	repl := func(ev ir.EventKind, body *ir.Program) *ir.Ability {
		return &ir.Ability{Kind: ir.AbReplacement, Trigger: &ir.Trigger{Event: ev, Ref: selfRef}, Body: body}
	}
	tapped := repl(ir.EvWouldEnter, enter(&ir.Enter{Tapped: true}))
	shock := repl(ir.EvWouldEnter, abBody(
		&ir.Step{Verb: ir.VerbPayCost, Optional: true, Cost: &ir.Cost{Parts: []*ir.CostPart{{Kind: ir.CostPayLife, N: vc(2)}}}},
		&ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CNot, Args: []*ir.Cond{{Kind: ir.CCostPaid}}}, Then: enter(&ir.Enter{Tapped: true})},
	))
	doubler := &ir.Ability{Kind: ir.AbReplacement, Trigger: &ir.Trigger{
		Event:   ir.EvWouldDealDamage,
		Subject: &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PControlledBy, Ref: &ir.Ref{Kind: ir.RefYou}},
		Object:  ctOpp(),
	}, Body: abBody(&ir.Step{Verb: ir.VerbSetAmount, N: vk(ir.VMul, vc(2), &ir.Value{Kind: ir.VEventAmount})})}

	check := func(name string, a *ir.Ability, lines []string, svars []kv) {
		t.Helper()
		p, err := abRun(a)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		if len(p.lines) != 1 {
			t.Errorf("%s: lines %q", name, p.lines)
			return
		}
		if lines != nil && !reflect.DeepEqual(abParams(p.lines[0]), lines) {
			t.Errorf("%s: params %q, want %q", name, abParams(p.lines[0]), lines)
		}
		if !reflect.DeepEqual(p.svars, svars) {
			t.Errorf("%s: svars %v, want %v", name, p.svars, svars)
		}
	}
	check("tapped", tapped, []string{"Event$ Moved", "ValidCard$ Card.Self", "Destination$ Battlefield", "ReplacementResult$ Updated", "ReplaceWith$ DBTap"},
		[]kv{{"DBTap", "DB$ Tap | Defined$ Self | ETB$ True"}})
	check("pay life or tapped", shock, []string{"Event$ Moved", "ValidCard$ Card.Self", "Destination$ Battlefield", "ReplacementResult$ Updated", "ReplaceWith$ DBTap"},
		[]kv{{"DBTap", "DB$ Tap | Defined$ Self | ETB$ True | UnlessCost$ PayLife<2> | UnlessPayer$ You"}})
	check("etb counter", repl(ir.EvWouldEnter, enter(&ir.Enter{Counter: "P1P1", N: vc(2)})), nil, nil)
	check("etb counter X", repl(ir.EvWouldEnter, enter(&ir.Enter{Counter: "P1P1", N: vX})), nil, []kv{{"X", "Count$xPaid"}})
	check("damage doubler", doubler,
		[]string{"Event$ DamageDone", "ActiveZones$ Battlefield", "ValidSource$ Card.YouCtrl", "ValidTarget$ Opponent", "ReplaceWith$ DBReplaceEffect"},
		[]kv{{"N", "ReplaceCount$DamageAmount/Times.2"}, {"DBReplaceEffect", "DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ N"}})

	p, _ := abRun(repl(ir.EvWouldEnter, enter(&ir.Enter{Counter: "P1P1", N: vc(2)})))
	if !reflect.DeepEqual(p.lines, []string{"K:etbCounter:P1P1:2"}) {
		t.Errorf("etb counter line: %q", p.lines)
	}
	p, _ = abRun(repl(ir.EvWouldEnter, enter(&ir.Enter{Counter: "P1P1", N: vX})))
	if !reflect.DeepEqual(p.lines, []string{"K:etbCounter:P1P1:X"}) {
		t.Errorf("etb counter X line: %q", p.lines)
	}

	raid := repl(ir.EvWouldEnter, abBody(&ir.Step{Verb: ir.VerbIf, Cond: ctRef(ctYou(), ctPred(ir.PAttackedThisTurn)), Then: enter(&ir.Enter{Counter: "P1P1", N: vc(1)})}))
	outside := &ir.Ability{Kind: ir.AbTriggered, Trigger: abTrig(ir.EvEnters, abSelf(), nil), Body: enter(&ir.Enter{Tapped: true})}
	setOutside := &ir.Ability{Kind: ir.AbTriggered, Trigger: abTrig(ir.EvEnters, abSelf(), nil), Body: abBody(&ir.Step{Verb: ir.VerbSetAmount, N: vc(1)})}
	setWrongEvent := repl(ir.EvWouldEnter, abBody(&ir.Step{Verb: ir.VerbSetAmount, N: vc(1)}))
	counterAndTapped := repl(ir.EvWouldEnter, enter(&ir.Enter{Counter: "P1P1", N: vc(1), Tapped: true}))
	notSelf := &ir.Ability{Kind: ir.AbReplacement, Trigger: &ir.Trigger{Event: ir.EvWouldEnter}, Body: enter(&ir.Enter{Tapped: true})}
	for _, c := range []struct {
		name string
		a    *ir.Ability
		what string
	}{
		{"counter under a condition", raid, "repl:etb-counter-cond"},
		{"would die", repl(ir.EvWouldDie, abDraw()), "repl:would-die"},
		{"would create tokens", repl(ir.EvWouldCreateTokens, abDraw()), "repl:would-create-tokens"},
		{"enter of another object", notSelf, "repl:would-enter"},
		{"modify-enter outside a replacement", outside, "verb:modify-enter-outside-replacement"},
		{"set-amount outside a replacement", setOutside, "verb:set-amount-outside-replacement"},
		{"set-amount under would-enter", setWrongEvent, "verb:set-amount"},
		{"counter and tapped", counterAndTapped, "verb:modify-enter"},
	} {
		_, err := abRun(c.a)
		abWhat(t, c.name, err, c.what)
	}
}

func TestKeywordArgs(t *testing.T) {
	cost := abGenericCost(1)
	cost.Mana = append(cost.Mana, ir.ManaSym{Kind: ir.ManaColored, Colors: ir.ColorR})
	life := &ir.Cost{Parts: []*ir.CostPart{{Kind: ir.CostPayLife, N: vc(3)}}}
	k := func(name string, f func(*ir.Keyword)) *ir.Ability {
		kwd := &ir.Keyword{Name: name}
		f(kwd)
		return &ir.Ability{Kind: ir.AbKeyword, Keyword: kwd}
	}
	colour := func(c ir.ColorSet) *ir.Filter { return &ir.Filter{Op: ir.FPred, Pred: ir.PColor, Colors: c} }
	cases := []struct {
		name string
		a    *ir.Ability
		want string
		what string
	}{
		{"kicker", k("Kicker", func(x *ir.Keyword) { x.Cost = cost }), "K:Kicker:1 R", ""},
		{"flashback", k("Flashback", func(x *ir.Keyword) { x.Cost = cost }), "K:Flashback:1 R", ""},
		{"miracle", k("Miracle", func(x *ir.Keyword) { x.Cost = cost }), "K:Miracle:1 R", ""},
		{"ward mana", k("Ward", func(x *ir.Keyword) { x.Cost = abGenericCost(2) }), "K:Ward:2", ""},
		{"ward life", k("Ward", func(x *ir.Keyword) { x.Cost = life }), "K:Ward:PayLife<3>", ""},
		{"protection", k("Protection", func(x *ir.Keyword) { x.Filter = colour(ir.ColorR) }), "K:Protection from red", ""},
		{"protection two colours", k("Protection", func(x *ir.Keyword) { x.Filter = colour(ir.ColorR | ir.ColorG) }), "", "keyword:Protection"},
		{"ward without a cost", k("Ward", func(x *ir.Keyword) { x.N = vc(1) }), "", "keyword:Ward"},
		{"other", k("Annihilator", func(x *ir.Keyword) { x.N = vc(2) }), "", "keyword:Annihilator"},
		{"empty cost", k("Flashback", func(x *ir.Keyword) { x.Cost = &ir.Cost{} }), "", "cost:empty"},
	}
	for _, c := range cases {
		p, err := abRun(c.a)
		if c.what != "" {
			abWhat(t, c.name, err, c.what)
			continue
		}
		if err != nil || !reflect.DeepEqual(p.lines, []string{c.want}) {
			t.Errorf("%s: %v %q, want %q", c.name, err, p.lines, c.want)
		}
	}

	kicked := &ir.Ability{Kind: ir.AbSpell, Keyword: &ir.Keyword{Name: "Kicker", Cost: cost}, Body: abDraw()}
	p, err := abRun(kicked)
	if err != nil || len(p.lines) != 2 || p.lines[0] != "K:Kicker:1 R" || !strings.HasPrefix(p.lines[1], "A:SP$ Draw") {
		t.Errorf("kicked spell: %v %q", err, p.lines)
	}
	other := &ir.Ability{Kind: ir.AbSpell, Keyword: &ir.Keyword{Name: "Flashback", Cost: cost}, Body: abDraw()}
	_, err = abRun(other)
	abWhat(t, "spell keyword", err, "spell:keyword")
}

// corpusLineParams reads every T:, S: and R: line of the Forge corpus and
// returns the Mode$ values of triggers and statics, the Event$ values of
// replacements, and the Phase$ values.
func corpusLineParams(t *testing.T) (trig, stat, repl, phase map[string]bool) {
	t.Helper()
	testutil.CorpusRegistry(t)
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	trig, stat, repl, phase = map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	dir := filepath.Join(root, ".cards", "cardsfolder")
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, l := range strings.Split(string(b), "\n") {
			if len(l) < 2 || (l[:2] != "T:" && l[:2] != "S:" && l[:2] != "R:") {
				continue
			}
			for _, x := range strings.Split(l[2:], "|") {
				k, v, ok := strings.Cut(strings.TrimSpace(x), "$")
				v = strings.TrimSpace(v)
				switch {
				case !ok:
				case k == "Mode" && l[0] == 'T':
					trig[v] = true
				case k == "Mode" && l[0] == 'S':
					stat[v] = true
				case k == "Event" && l[0] == 'R':
					repl[v] = true
				case k == "Phase":
					phase[v] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestModesAttested(t *testing.T) {
	trig, stat, repl, phase := corpusLineParams(t)
	for _, c := range []struct {
		kind string
		have map[string]bool
		want []string
	}{
		{"trigger Mode", trig, []string{"ChangesZone", "Attacks", "Phase", "Drawn", "LifeGained", "LifeLost", "SpellCast", "DamageDone", "BecomesTarget", "CounterAddedOnce"}},
		{"static Mode", stat, []string{"Continuous", "CantBlock", "CantBlockBy", "CastWithFlash", "RaiseCost", "ReduceCost", "AlternativeCost"}},
		{"replacement Event", repl, []string{"Moved", "DamageDone", "GameLoss", "GameWin", "Counter"}},
		{"Phase", phase, []string{"Upkeep", "End of Turn", "BeginCombat"}},
	} {
		for _, w := range c.want {
			if !c.have[w] {
				t.Errorf("%s %q occurs nowhere in the corpus", c.kind, w)
			}
		}
	}
}
