package mtgish

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

var (
	tActions = tn("Actions", "ActionList", []any{})
	tPlayers = tn("Players", "Stub")
	tPerms   = tn("Permanents", "Stub")
	tCmpN    = tn("TestOnly", "Cmp")
)

func playersFilter() *ir.Filter {
	return &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}
}

func ruleStubs(t *testing.T, extra map[string]lowerFunc) {
	t.Helper()
	m := map[string]lowerFunc{
		"Protectable.TestOnly": func(*Ctx, node) (any, error) { return alFilter(), nil },
	}
	for k, f := range extra {
		m[k] = f
	}
	alStubs(t, m)
}

func lowerAb(t *testing.T, x any) *ir.Ability {
	t.Helper()
	ab, err := (&Ctx{}).Ability(x)
	if err != nil {
		t.Fatal(err)
	}
	return ab
}

func costMana() *ir.Cost { return &ir.Cost{HasMana: true} }

func TestRuleLowering(t *testing.T) {
	ruleStubs(t, nil)
	empty := &ir.Program{}
	kw := func(name string, cost *ir.Cost, f *ir.Filter) *ir.Ability {
		return &ir.Ability{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: name, Cost: cost, Filter: f}}
	}
	syms := []any{tn("ManaSymbol", "ManaCostX"), tn("ManaSymbol", "ManaCostW")}
	symsWant := &ir.Cost{HasMana: true, Mana: []ir.ManaSym{{Kind: ir.ManaX}, {Kind: ir.ManaColored, Colors: ir.ColorW}}}
	trig := &ir.Trigger{Event: ir.EvDies}
	condK := &ir.Cond{Kind: ir.CKicked}
	cases := []struct {
		name string
		in   node
		want *ir.Ability
	}{
		{"SpellActions", tn("Rule", "SpellActions", tActions), &ir.Ability{Kind: ir.AbSpell, Body: empty}},
		{"Kicker spell", tn("Rule", "SpellActions_Kicker", tCost, tActions, tActions), &ir.Ability{Kind: ir.AbSpell,
			Keyword: &ir.Keyword{Name: "Kicker", Cost: costMana()},
			Body:    &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbIf, Cond: condK, Then: empty, Else: empty}}}}},
		{"Activated", tn("Rule", "Activated", tCost, tActions), &ir.Ability{Kind: ir.AbActivated, Cost: costMana(), Body: empty}},
		{"Mod sorcery", tn("Rule", "ActivatedWithModifiers", tCost, tActions, tn("ActivateModifier", "ActivateOnlyAsASorcery")),
			&ir.Ability{Kind: ir.AbActivated, Cost: costMana(), Body: empty, SorceryOnly: true}},
		{"Mod instant", tn("Rule", "ActivatedWithModifiers", tCost, tActions, tn("ActivateModifier", "ActivateOnlyAsAnInstant")),
			&ir.Ability{Kind: ir.AbActivated, Cost: costMana(), Body: empty, InstantOnly: true}},
		{"Mod once", tn("Rule", "ActivatedWithModifiers", tCost, tActions, tn("ActivateModifier", "ActivateOnlyOnce")),
			&ir.Ability{Kind: ir.AbActivated, Cost: costMana(), Body: empty, Once: true}},
		{"Mod if", tn("Rule", "ActivatedWithModifiers", tCost, tActions, tn("ActivateModifier", "ActivateOnlyIf", tCond)),
			&ir.Ability{Kind: ir.AbActivated, Cost: costMana(), Body: empty, Cond: condK}},
		{"TriggerA", tn("Rule", "TriggerA", tTrig, tActions), &ir.Ability{Kind: ir.AbTriggered, Trigger: trig, Body: empty}},
		{"TriggerI", tn("Rule", "TriggerI", tTrig, tCond, tActions), &ir.Ability{Kind: ir.AbTriggered, Trigger: trig, Cond: condK, Body: empty}},
		{"TriggerOnce", tn("Rule", "TriggerOnceEachTurn", tTrig, tActions),
			&ir.Ability{Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvDies, OncePerTurn: true}, Body: empty}},
		{"If over spell", tn("Rule", "If", tCond, []any{tn("Rule", "SpellActions", tActions)}),
			&ir.Ability{Kind: ir.AbSpell, Body: empty, Cond: condK}},
		{"If over TriggerI", tn("Rule", "If", tCond, []any{tn("Rule", "TriggerI", tTrig, tCond, tActions)}),
			&ir.Ability{Kind: ir.AbTriggered, Trigger: trig, Body: empty, Cond: &ir.Cond{Kind: ir.CAll, Args: []*ir.Cond{condK, condK}}}},
		{"FromGraveyard", tn("Rule", "FromGraveyard", tn("Rule", "SpellActions", tActions)),
			&ir.Ability{Kind: ir.AbSpell, Body: empty, Zones: ir.MakeZoneSet(ir.ZoneGraveyard)}},
		{"FromStack", tn("Rule", "FromStack", tn("Rule", "SpellActions", tActions)),
			&ir.Ability{Kind: ir.AbSpell, Body: empty, Zones: ir.MakeZoneSet(ir.ZoneStack)}},
		{"Ward", tn("Rule", "Ward", tCost), kw("Ward", costMana(), nil)},
		{"Equip", tn("Rule", "Equip", tPerms, tCost), kw("Equip", costMana(), alFilter())},
		{"Flashback", tn("Rule", "Flashback", tCost), kw("Flashback", costMana(), nil)},
		{"Kicker", tn("Rule", "Kicker", tCost), kw("Kicker", costMana(), nil)},
		{"Miracle", tn("Rule", "Miracle", tCost), kw("Miracle", costMana(), nil)},
		{"MiracleX", tn("Rule", "MiracleX", syms), kw("Miracle", symsWant, nil)},
		{"Surge", tn("Rule", "Surge", syms), kw("Surge", symsWant, nil)},
		{"Affinity", tn("Rule", "Affinity", tPerms), kw("Affinity", nil, alFilter())},
		{"Enchant", tn("Rule", "EnchantPermanent", tPerms), kw("Enchant", nil, alFilter())},
		{"Protection", tn("Rule", "Protection", tn("Protectable", "FromEverything")),
			kw("Protection", nil, &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PAny})},
		{"HexproofFrom", tn("Rule", "HexproofFrom", tn("Protectable", "FromColor", tn("ProtectableColor", "Colors", []any{tColor, tColor}))),
			kw("Hexproof", nil, &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PColor, Colors: ir.ColorU})},
		{"FromTypes", tn("Rule", "Protection", tn("Protectable", "FromTypes", tPerms)), kw("Protection", nil, alFilter())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lowerAb(t, tc.in)
			if !ir.Equal(got, tc.want) {
				t.Errorf("got %s\nwant %s", got, tc.want)
			}
		})
	}

	t.Run("If multi", func(t *testing.T) {
		_, err := (&Ctx{}).Ability(tn("Rule", "If", tCond, []any{tn("Rule", "SpellActions", tActions), tn("Rule", "SpellActions", tActions)}))
		re, ok := err.(*RefusalError)
		if !ok || re.Key != "Rule.If:multi" {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("damage sources", func(t *testing.T) {
		v, err := (&Ctx{}).lower(tn("DamageSources", "ControlledByAPlayer", tPlayers))
		if err != nil {
			t.Fatal(err)
		}
		want := &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PControlledBy,
			Ref: &ir.Ref{Kind: ir.RefAll, Filter: playersFilter()}}
		if !ir.Equal(v, want) {
			t.Errorf("got %s want %s", v, want)
		}
	})
}

func TestTriggerLowering(t *testing.T) {
	ruleStubs(t, map[string]lowerFunc{
		"CounterType.Stub": func(*Ctx, node) (any, error) { return "P1P1", nil },
	})
	pl, pm := playersFilter(), alFilter()
	cmp := &ir.Compare{Op: ir.CmpGE, V: constV(3)}
	ctr := tn("CounterType", "Stub")
	P, M := tPlayers, tPerms
	cases := []struct {
		name string
		in   node
		want *ir.Trigger
	}{
		{"EndStep", tn("Trigger", "AtTheBeginningOfAPlayersEndStep", P), &ir.Trigger{Event: ir.EvEndStep, Subject: pl}},
		{"Upkeep", tn("Trigger", "AtTheBeginningOfAPlayersUpkeep", P), &ir.Trigger{Event: ir.EvUpkeep, Subject: pl}},
		{"BeginCombat", tn("Trigger", "AtTheBeginningOfCombatDuringAPlayersTurn", P), &ir.Trigger{Event: ir.EvBeginCombat, Subject: pl}},
		{"If", tn("Trigger", "If", tCond, tn("Trigger", "WhenAPermanentDies", M)),
			&ir.Trigger{Event: ir.EvDies, Subject: pm, Cond: &ir.Cond{Kind: ir.CKicked}}},
		{"Or", tn("Trigger", "Or", []any{tn("Trigger", "WhenAPermanentDies", M), tn("Trigger", "WhenAPlayerAttacks", P)}),
			&ir.Trigger{Event: ir.EvOr, Args: []*ir.Trigger{{Event: ir.EvDies, Subject: pm}, {Event: ir.EvPlayerAttacks, Subject: pl}}}},
		{"Attacks", tn("Trigger", "WhenACreatureAttacks", M), &ir.Trigger{Event: ir.EvAttacks, Subject: pm}},
		{"CombatDamage", tn("Trigger", "WhenACreatureDealsCombatDamage", M), &ir.Trigger{Event: ir.EvDealsCombatDamage, Subject: pm}},
		{"CombatDamageToPlayer", tn("Trigger", "WhenACreatureDealsCombatDamageToAPlayer", M, P),
			&ir.Trigger{Event: ir.EvDealsCombatDamageToPlayer, Subject: pm, Object: pl}},
		{"NoncombatDamage", tn("Trigger", "WhenASourceDealsNoncombatDamageToAPlayer", M, P),
			&ir.Trigger{Event: ir.EvDealsNoncombatDamageToPlayer, Subject: pm, Object: pl}},
		{"BecomesTarget", tn("Trigger", "WhenAPermanentBecomesTheTargetOfASpell", M, M),
			&ir.Trigger{Event: ir.EvBecomesTarget, Subject: pm, Object: pm}},
		{"Untapped", tn("Trigger", "WhenAPermanentBecomesUntapped", M), &ir.Trigger{Event: ir.EvUntaps, Subject: pm}},
		{"Dies", tn("Trigger", "WhenAPermanentDies", M), &ir.Trigger{Event: ir.EvDies, Subject: pm}},
		{"Enters", tn("Trigger", "WhenAPermanentEntersTheBattlefield", M), &ir.Trigger{Event: ir.EvEnters, Subject: pm}},
		{"EntersUnder", tn("Trigger", "WhenAPermanentEntersTheBattlefieldUnderAPlayersControl", M, P),
			&ir.Trigger{Event: ir.EvEnters, Subject: pm, Object: pl}},
		{"ToGraveyard", tn("Trigger", "WhenAPermanentIsPutIntoAPlayersGraveyard", M, P),
			&ir.Trigger{Event: ir.EvPutIntoGraveyard, Subject: pm, Object: pl}},
		{"Leaves", tn("Trigger", "WhenAPermanentLeavesTheBattlefield", M), &ir.Trigger{Event: ir.EvLeaves, Subject: pm}},
		{"PlayerAttacks", tn("Trigger", "WhenAPlayerAttacks", P), &ir.Trigger{Event: ir.EvPlayerAttacks, Subject: pl}},
		{"AttacksWithN", tn("Trigger", "WhenAPlayerAttacksWithANumberOfCreatures", P, tCmpN, M),
			&ir.Trigger{Event: ir.EvPlayerAttacks, Subject: pl, Cmp: cmp, Object: pm}},
		{"Cast", tn("Trigger", "WhenAPlayerCastsASpell", P, M), &ir.Trigger{Event: ir.EvCast, Subject: pl, Object: pm}},
		{"Discards", tn("Trigger", "WhenAPlayerDiscardsACard", P, M), &ir.Trigger{Event: ir.EvDiscards, Subject: pl, Object: pm}},
		{"Draws", tn("Trigger", "WhenAPlayerDrawsACard", P), &ir.Trigger{Event: ir.EvDraws, Subject: pl}},
		{"DrawsNth", tn("Trigger", "WhenAPlayerDrawsTheirNthCardEachTurn", P, tCmpN), &ir.Trigger{Event: ir.EvDraws, Subject: pl, Cmp: cmp}},
		{"GainsLife", tn("Trigger", "WhenAPlayerGainsLife", P), &ir.Trigger{Event: ir.EvGainsLife, Subject: pl}},
		{"GainsLifeFirst", tn("Trigger", "WhenAPlayerGainsLifeForTheFirstTimeEachTurn", P),
			&ir.Trigger{Event: ir.EvGainsLife, Subject: pl, Cmp: &ir.Compare{Op: ir.CmpEQ, V: constV(1)}}},
		{"LosesLife", tn("Trigger", "WhenAPlayerLosesLife", P), &ir.Trigger{Event: ir.EvLosesLife, Subject: pl}},
		{"PlaysLand", tn("Trigger", "WhenAPlayerPlaysALand", P, M), &ir.Trigger{Event: ir.EvPlaysLand, Subject: pl, Object: pm}},
		{"PutsCounters", tn("Trigger", "WhenAPlayerPutsAnyNumberOfCountersOfTypeOnAPermanent", P, ctr, M),
			&ir.Trigger{Event: ir.EvCountersPut, Subject: pl, Word: "P1P1", Object: pm}},
		{"CountersPut", tn("Trigger", "WhenAnyNumberOfCountersOfTypeArePutOnAPermanent", ctr, M),
			&ir.Trigger{Event: ir.EvCountersPut, Word: "P1P1", Object: pm}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (&Ctx{}).Trigger(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if !ir.Equal(got, tc.want) {
				t.Errorf("got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func nestedRule() node {
	return tn("Rule", "TriggerA", tTrig,
		tn("Actions", "Targeted", []any{targetPerm()}, tn("Actions", "ActionList", []any{
			tn("Action", "CreatePermanentLayerEffectUntil", tn("Permanent", "Ref_TargetPermanent"),
				[]any{tn("LayerEffect", "AddAbility", []any{
					tn("Rule", "Activated", tCost,
						tn("Actions", "Targeted", []any{targetPlayer()}, tn("Actions", "ActionList", []any{
							tn("Action", "PlayerAction", tn("Player", "Ref_TargetPlayer"), tn("Action", "DrawACard"))})))})},
				tExp),
			destroyTarget(),
		})))
}

func TestNestedAbilityTargets(t *testing.T) {
	ruleStubs(t, nil)
	slot0 := &ir.Ref{Kind: ir.RefTarget, Slot: 0}
	inner := &ir.Ability{Kind: ir.AbActivated, Cost: costMana(), Body: &ir.Program{
		Steps:   []*ir.Step{{Verb: ir.VerbDraw, Who: slot0, N: refConst1()}},
		Targets: []*ir.Target{{Min: 1, Max: 1, Filter: playersFilter()}}}}
	apply := &ir.Step{Verb: ir.VerbApplyEffect, Obj: slot0, Dur: ir.DurEndOfTurn,
		Effects: []*ir.Effect{{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{inner}}}}
	want := &ir.Ability{Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvDies}, Body: &ir.Program{
		Steps:   []*ir.Step{apply, destroySlot0},
		Targets: []*ir.Target{{Min: 1, Max: 1, Filter: alFilter()}}}}
	got := lowerAb(t, nestedRule())
	if !ir.Equal(got, want) {
		t.Errorf("got %s\nwant %s", got, want)
	}

	// Destroy first: the same slots.
	r := nestedRule()
	acts := argN(argN(r, 1), 1)
	l := list(rawArgs(acts))
	l[0], l[1] = l[1], l[0]
	want.Body.Steps = []*ir.Step{destroySlot0, apply}
	got = lowerAb(t, r)
	if !ir.Equal(got, want) {
		t.Errorf("reordered: got %s\nwant %s", got, want)
	}
}

func TestWrapperIsNotNested(t *testing.T) {
	ruleStubs(t, nil)
	r := tn("Rule", "FromGraveyard", tn("Rule", "Activated", tCost,
		tn("Actions", "Targeted", []any{targetPerm()}, tn("Actions", "ActionList", []any{destroyTarget()}))))
	got := lowerAb(t, r)
	want := &ir.Ability{Kind: ir.AbActivated, Cost: costMana(), Zones: ir.MakeZoneSet(ir.ZoneGraveyard),
		Body: &ir.Program{Steps: []*ir.Step{destroySlot0}, Targets: []*ir.Target{{Min: 1, Max: 1, Filter: alFilter()}}}}
	if !ir.Equal(got, want) {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestRuleKeysRegistered(t *testing.T) {
	for _, v := range []string{"SpellActions", "SpellActions_Kicker", "Activated", "ActivatedWithModifiers", "TriggerA", "TriggerI",
		"TriggerOnceEachTurn", "If", "FromGraveyard", "FromStack", "Ward", "Equip", "Flashback", "Kicker", "Miracle", "MiracleX",
		"Surge", "Affinity", "EnchantPermanent", "Protection", "HexproofFrom"} {
		if !Registered("Rule." + v) {
			t.Errorf("Rule.%s is not registered", v)
		}
	}
}

func TestTriggerSliceVariantsRegistered(t *testing.T) {
	checkFamilyCoverage(t, []string{"Trigger", "ActivateModifier", "DamageSources", "Protectable", "ProtectableColor"},
		map[string]string{
			"ProtectableColor.TheChosenColor": "chosen colour, as CC-12's Color.TheChosenColor",
		})
}
