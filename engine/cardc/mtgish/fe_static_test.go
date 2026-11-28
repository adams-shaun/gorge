package mtgish

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func TestStaticSliceVariantsRegistered(t *testing.T) {
	checkFamilyCoverage(t, []string{"Cost", "ExilableCost", "RemoveCountersCost", "ReplacementActionWouldEnterCost",
		"CostReductionSymbol", "CostReductionSymbolX", "StaticLayerEffect", "SettableColor", "PermanentRule",
		"SpellsAndAbilities", "PlayerEffect", "CastEffect", "SpellEffect", "ReplacableEventWouldEnter",
		"ReplacableEventWouldPutCounters", "ReplacableEventWouldDealDamage",
		"ReplacableEventAnEffectWouldCreateAnyNumberOfTokens", "ReplacableEventWouldPutIntoGraveyard",
		"EventPreventDamage", "DamageRecipientsList", "ReplacementActionWouldEnter",
		"ReplacementActionWouldPutCounters", "ReplacementActionWouldDealDamage",
		"ReplacementActionAnEffectWouldCreateAnyNumberOfTokens", "ReplacementActionWouldPutIntoGraveyard",
		"ActionPreventDamage"},
		map[string]string{
			"Cost.ItsManaCost": "needs the permanent's mana cost reader",
			"Cost.Or":          "alternative costs",
			"Cost.PutTopCardOfLibraryOfTypeOnBattlefield":                                     "cost that puts a card onto the battlefield",
			"PlayerEffect.MayCastASpellOfEachPermanentFromAmongCardsInGraveyardsOnceEachTurn": "once-per-turn graveyard casting",
			"PlayerEffect.MayPlayALandFromAmongCardsInGraveyardsOnceEachTurn":                 "once-per-turn graveyard land play",
			"PlayerEffect.MayPlayExiledCardsAndMaySpendManaAsThoughAnyTypeToCast":             "mana-type spend permission",
			"ReplacementActionWouldEnter.ChooseACardName":                                     "needs CardsInOracle and the chosen-name readers",
			"SettableColor.Devoid": "devoid is a characteristic-defining keyword",
		})
}

// ssStubs adds the stubs these tests need on top of alStubs.
func ssStubs(t *testing.T) {
	t.Helper()
	alStubs(t, map[string]lowerFunc{
		"TestOnly.You":   func(*Ctx, node) (any, error) { return &ir.Ref{Kind: ir.RefYou}, nil },
		"TestOnly.Rules": func(*Ctx, node) (any, error) { return &ir.Ability{Kind: ir.AbSpell}, nil },
		"Spells.Stub": func(*Ctx, node) (any, error) {
			return &ir.Filter{Subject: ir.SubjSpell, Op: ir.FPred, Pred: ir.PAny}, nil
		},
		"DamageSources.Stub": func(*Ctx, node) (any, error) {
			return &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PAny}, nil
		},
		"Condition.CostWasPd": func(*Ctx, node) (any, error) { return &ir.Cond{Kind: ir.CCostPaid}, nil },
	})
}

type lowerCase struct {
	name string
	in   node
	want any
}

func runLowerCases(t *testing.T, cases []lowerCase) {
	t.Helper()
	for _, tc := range cases {
		got, err := (&Ctx{}).lower(tc.in)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if cs, ok := tc.want.(ir.ColorSet); ok {
			if got != cs {
				t.Errorf("%s: got %v want %v", tc.name, got, cs)
			}
			continue
		}
		if !ir.Equal(got, tc.want) {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, dump(got), dump(tc.want))
		}
	}
}

func dump(x any) string {
	if s, ok := x.(fmt.Stringer); ok {
		return s.String()
	}
	b, _ := json.Marshal(x)
	return string(b)
}

func gen(n int) node    { return tn("ManaSymbol", "ManaCostGeneric", float64(n)) }
func sym(l string) node { return tn("ManaSymbol", "ManaCost"+l) }

func TestCostLowering(t *testing.T) {
	ssStubs(t)
	you := &ir.Ref{Kind: ir.RefYou}
	self := &ir.Ref{Kind: ir.RefSelf}
	one, two := refConst1(), constV(2)
	hand := anyCardIn(ir.ZoneHand)
	allF := alFilter()
	p := func(k ir.CostKind, mod func(*ir.CostPart)) *ir.Cost {
		cp := &ir.CostPart{Kind: k}
		mod(cp)
		return &ir.Cost{Parts: []*ir.CostPart{cp}}
	}
	cost := func(v string, a ...any) node { return tn("Cost", v, a...) }
	manaW := []ir.ManaSym{{Kind: ir.ManaGeneric, N: 1}, {Kind: ir.ManaColored, Colors: ir.ColorW}}
	runLowerCases(t, []lowerCase{
		{"PayMana", cost("PayMana", []any{gen(1), sym("W")}), &ir.Cost{Mana: manaW, HasMana: true}},
		{"PayManaX", cost("PayManaX", []any{sym("X")}, tn("GameNumber", "ValueX")),
			&ir.Cost{Mana: []ir.ManaSym{{Kind: ir.ManaX}}, HasMana: true}},
		{"TapSelf", cost("TapPermanent", tRef), p(ir.CostTapSelf, func(*ir.CostPart) {})},
		{"TapOther", cost("TapPermanent", tn("TestOnly", "You")), p(ir.CostTapUntapped, func(c *ir.CostPart) { c.Obj = you })},
		{"TapNumber", cost("TapNumberPermanents", tN2, tFilt),
			p(ir.CostTapUntapped, func(c *ir.CostPart) { c.Obj = &ir.Ref{Kind: ir.RefChoose, Filter: allF, N: two} })},
		{"SacrificePermanent", cost("SacrificePermanent", tRef), p(ir.CostSacrifice, func(c *ir.CostPart) { c.Obj = self })},
		{"SacrificeA", cost("SacrificeAPermanent", tFilt),
			p(ir.CostSacrifice, func(c *ir.CostPart) { c.Obj = &ir.Ref{Kind: ir.RefChoose, Filter: allF, N: one} })},
		{"DiscardACard", cost("DiscardACard"),
			p(ir.CostDiscard, func(c *ir.CostPart) { c.Obj = &ir.Ref{Kind: ir.RefChoose, Filter: hand, N: one} })},
		{"DiscardHand", cost("DiscardHand"),
			p(ir.CostDiscard, func(c *ir.CostPart) { c.Obj = &ir.Ref{Kind: ir.RefAll, Filter: hand} })},
		{"PayLife", cost("PayLife", tN2), p(ir.CostPayLife, func(c *ir.CostPart) { c.N = two })},
		{"Loyalty -8", cost("Loyalty", -8.0), p(ir.CostLoyalty, func(c *ir.CostPart) { c.N = constV(-8) })},
		{"Loyalty +1", cost("Loyalty", 1.0), p(ir.CostLoyalty, func(c *ir.CostPart) { c.N = one })},
		{"Exile", cost("Exile", []any{
			tn("ExilableCost", "Permanent", tRef),
			tn("ExilableCost", "ACardOfTypeFromPlayersHand", tFilt, tn("TestOnly", "You")),
		}, []any{}), &ir.Cost{Parts: []*ir.CostPart{
			{Kind: ir.CostExile, Obj: self},
			{Kind: ir.CostExile, Obj: &ir.Ref{Kind: ir.RefChoose, Of: you, Filter: allF, N: one}},
		}}},
		{"ReturnToHand", cost("PutAPermanentIntoItsOwnersHand", tFilt),
			p(ir.CostReturnToHand, func(c *ir.CostPart) { c.Obj = &ir.Ref{Kind: ir.RefChoose, Filter: allF, N: one} })},
		{"RemoveCounters", cost("RemoveCounters", []any{
			tn("RemoveCountersCost", "ACounterOfTypeFromPermanent", tCounter, tRef),
			tn("RemoveCountersCost", "NumberCountersOfTypeFromPermanent", tN2, tCounter, tRef),
			tn("RemoveCountersCost", "NumberCountersFromAmongPermanents", tN2, tFilt),
		}), &ir.Cost{Parts: []*ir.CostPart{
			{Kind: ir.CostRemoveCounters, Obj: self, N: one, Word: "P1P1"},
			{Kind: ir.CostRemoveCounters, Obj: self, N: two, Word: "P1P1"},
			{Kind: ir.CostRemoveCounters, Obj: &ir.Ref{Kind: ir.RefAll, Filter: allF}, N: two},
		}}},
		{"RevealTop", cost("RevealTopCardOfLibrary"),
			p(ir.CostReveal, func(c *ir.CostPart) { c.Obj = &ir.Ref{Kind: ir.RefTopOfLibrary, N: one} })},
		// Fiend Artisan's shape: mana, tap, sacrifice.
		{"And", cost("And", []any{
			cost("PayMana", []any{gen(1), sym("W")}),
			cost("TapPermanent", tRef),
			cost("SacrificeAPermanent", tFilt),
		}), &ir.Cost{Mana: manaW, HasMana: true, Parts: []*ir.CostPart{
			{Kind: ir.CostTapSelf},
			{Kind: ir.CostSacrifice, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: allF, N: one}},
		}}},
		{"WouldEnterCost.PayLife", tn("ReplacementActionWouldEnterCost", "PayLife", tN2),
			p(ir.CostPayLife, func(c *ir.CostPart) { c.N = two })},
		{"CostReduceGeneric", tn("CostReductionSymbol", "CostReduceGeneric", 2.0), &ir.ManaSym{Kind: ir.ManaGeneric, N: 2}},
		{"CostReduceX", tn("CostReductionSymbolX", "CostReduceX"), &ir.ManaSym{Kind: ir.ManaX}},
	})

	refusals := []struct {
		name string
		in   node
		key  string
	}{
		{"PayManaX value", cost("PayManaX", []any{sym("X")}, tN2), "Cost.PayManaX:value"},
		{"Exile flag", cost("Exile", []any{tn("ExilableCost", "Permanent", tRef)}, []any{tn("ExileFlag", "Limited", tExp)}), "Cost.Exile:flag"},
		{"reduction shape", tn("PlayerEffect", "DecreaseSpellCost", tn("Spells", "Stub"), []any{tn("CostReductionSymbol", "CostReduceW")}), "CostReductionSymbol:shape"},
	}
	for _, tc := range refusals {
		_, err := (&Ctx{}).lower(tc.in)
		if re, ok := err.(*RefusalError); !ok || re.Key != tc.key {
			t.Errorf("%s: error = %v, want refusal %s", tc.name, err, tc.key)
		}
	}
}

func TestStaticLowering(t *testing.T) {
	ssStubs(t)
	self := &ir.Ref{Kind: ir.RefSelf}
	you := &ir.Ref{Kind: ir.RefYou}
	allF := &ir.Ref{Kind: ir.RefAll, Filter: alFilter()}
	allPl := &ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}}
	spells := &ir.Filter{Subject: ir.SubjSpell, Op: ir.FPred, Pred: ir.PAny}
	one, two := refConst1(), constV(2)
	hexproof := &ir.Effect{Kind: ir.EffHexproof}
	st := func(o *ir.Ref, es ...*ir.Effect) *ir.Program { return static(o, es) }
	ab := func(body *ir.Program) *ir.Ability { return &ir.Ability{Kind: ir.AbStatic, Body: body} }
	cda := func(e *ir.Effect) *ir.Ability {
		return &ir.Ability{Kind: ir.AbStatic, Zones: allZones, Body: st(self, e)}
	}
	rule := func(v string, a ...any) node { return tn("Rule", v, a...) }
	sle := func(v string, a ...any) node { return tn("StaticLayerEffect", v, a...) }
	pr := func(v string, a ...any) node { return tn("PermanentRule", v, a...) }
	pe := func(v string, a ...any) node { return tn("PlayerEffect", v, a...) }
	ce := func(v string, a ...any) node { return tn("CastEffect", v, a...) }
	sp := tn("Spells", "Stub")
	simple := func(c ...string) node {
		l := []any{}
		for _, s := range c {
			l = append(l, s)
		}
		return tn("SettableColor", "SimpleColorList", l)
	}
	genCost := &ir.Cost{Mana: []ir.ManaSym{{Kind: ir.ManaGeneric, N: 1}}, HasMana: true}
	reduce := func(n int) []any { return []any{tn("CostReductionSymbol", "CostReduceGeneric", float64(n))} }
	runLowerCases(t, []lowerCase{
		{"PermanentLayerEffect", rule("PermanentLayerEffect", tRef, []any{tEff}), ab(st(self, hexproof))},
		{"EachPermanentLayerEffect", rule("EachPermanentLayerEffect", tFilt, []any{tEff}), ab(st(allF, hexproof))},
		{"PermanentRuleEffect", rule("PermanentRuleEffect", tRef, []any{pr("CantBlock")}), ab(st(self, &ir.Effect{Kind: ir.EffCantBlock}))},
		{"PlayerEffect", rule("PlayerEffect", tn("TestOnly", "You"), []any{pe("Hexproof")}), ab(st(you, hexproof))},
		{"EachPlayerEffect", rule("EachPlayerEffect", tn("Players", "Stub"), []any{pe("Hexproof")}), ab(st(allPl, hexproof))},
		{"CastEffect", rule("CastEffect", ce("CantBeCastIf", tCond)),
			ab(st(self, &ir.Effect{Kind: ir.EffCantCast, Cond: &ir.Cond{Kind: ir.CKicked}}))},
		{"ThisSpellEffect", rule("ThisSpellEffect", []any{tn("SpellEffect", "CantBeCountered")}),
			&ir.Ability{Kind: ir.AbStatic, Zones: ir.MakeZoneSet(ir.ZoneStack), Body: st(self, &ir.Effect{Kind: ir.EffCantBeCountered})}},
		{"CDA_Power", rule("CDA_Power", tN2), cda(&ir.Effect{Kind: ir.EffCDAPower, P: two})},
		{"CDA_Toughness", rule("CDA_Toughness", tN2), cda(&ir.Effect{Kind: ir.EffCDAToughness, T: two})},
		{"CDA_Color", rule("CDA_Color", simple("Blue", "Red")), cda(&ir.Effect{Kind: ir.EffCDAColor, Colors: ir.ColorU | ir.ColorR})},

		// StaticLayerEffect.
		{"AddAbility", sle("AddAbility", []any{tRule}), &ir.Effect{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{Kind: ir.AbSpell}}}},
		{"AddCreatureType", sle("AddCreatureType", "Zombie"), &ir.Effect{Kind: ir.EffAddSubtype, Words: []string{"Zombie"}}},
		{"AdjustPT", sle("AdjustPT", 1.0, -1.0), &ir.Effect{Kind: ir.EffAdjustPT, P: constV(1), T: constV(-1)}},
		{"AdjustPTForEach", sle("AdjustPTForEach", 1.0, 2.0, tN2),
			&ir.Effect{Kind: ir.EffAdjustPT, P: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{constV(1), two}}, T: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{constV(2), two}}}},
		{"LosesAllAbilities", sle("LosesAllAbilities"), &ir.Effect{Kind: ir.EffLoseAllAbilities}},
		{"SetCardtype", sle("SetCardtype", "Artifact"), &ir.Effect{Kind: ir.EffSetCardType, Words: []string{"Artifact"}}},
		{"SetColor", sle("SetColor", simple("White")), &ir.Effect{Kind: ir.EffSetColor, Colors: ir.ColorW}},
		{"SetCreatureType", sle("SetCreatureType", "Elf"), &ir.Effect{Kind: ir.EffSetSubtype, Words: []string{"Elf"}}},
		{"SetName", sle("SetName", "Bob"), &ir.Effect{Kind: ir.EffSetName, Words: []string{"Bob"}}},
		{"SetPT", sle("SetPT", tn("PT", "PT", 4.0, 4.0)), &ir.Effect{Kind: ir.EffSetPT, P: constV(4), T: constV(4)}},
		{"Colorless", tn("SettableColor", "Colorless"), ir.ColorSet(0)},
		{"SimpleColorList", simple("White", "Green"), ir.ColorW | ir.ColorG},

		// PermanentRule.
		{"CantBeBlocked", pr("CantBeBlocked"), &ir.Effect{Kind: ir.EffCantBeBlocked}},
		{"CantBeBlockedByDefenders", pr("CantBeBlockedByDefenders", tFilt), &ir.Effect{Kind: ir.EffCantBeBlockedBy, Filter: alFilter()}},
		{"CantBeTheTarget", pr("CantBeTheTargetOfSpellsOrAbilities", tn("SpellsAndAbilities", "ControlledByAPlayer", tn("Players", "Stub"))),
			&ir.Effect{Kind: ir.EffCantBeTargeted, Filter: &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PControlledBy, Ref: allPl}}},
		{"CantBlock", pr("CantBlock"), &ir.Effect{Kind: ir.EffCantBlock}},
		{"DoesntUntap", pr("DoesntUntapDuringControllersUntap"), &ir.Effect{Kind: ir.EffNoUntap}},
		{"MustAttack", pr("MustAttack"), &ir.Effect{Kind: ir.EffMustAttack}},

		// PlayerEffect.
		{"CantCastSpells", pe("CantCastSpells", sp), &ir.Effect{Kind: ir.EffCantCast, Filter: spells}},
		{"CantLose", pe("CantLoseTheGame"), &ir.Effect{Kind: ir.EffCantLose}},
		{"CantWin", pe("CantWinTheGame"), &ir.Effect{Kind: ir.EffCantWin}},
		{"DecreaseSpellCost", pe("DecreaseSpellCost", sp, []any{tn("CostReductionSymbol", "CostReduceGeneric", 1.0), tn("CostReductionSymbol", "CostReduceGeneric", 1.0)}),
			&ir.Effect{Kind: ir.EffSpellsCostLess, Filter: spells, N: two}},
		{"IncreaseSpellCost", pe("IncreaseSpellCost", sp, tn("Cost", "PayMana", []any{gen(1)})),
			&ir.Effect{Kind: ir.EffSpellsCostMore, Filter: spells, Cost: genCost}},
		{"NoMaxHand", pe("HasNoMaximumHandSize"), &ir.Effect{Kind: ir.EffNoMaxHandSize}},
		{"Hexproof", pe("Hexproof"), hexproof},
		{"MayCastGraveyardCard", pe("MayCastGraveyardCard", tRef), &ir.Effect{Kind: ir.EffMayCast, Ref: self}},
		{"MayCastAddedCost", pe("MayCastCardFromGraveyardByPayingAddedCost", tRef, tn("Cost", "PayMana", []any{gen(1)})),
			&ir.Effect{Kind: ir.EffMayCast, Ref: self, Cost: genCost}},
		{"MayPlayExiledCard", pe("MayPlayExiledCard", tRef), &ir.Effect{Kind: ir.EffMayPlay, Ref: self}},
		{"Flash", pe("MayCastSpellsAsThoughTheyHadFlash", sp), &ir.Effect{Kind: ir.EffFlash, Filter: spells}},
		{"CastWithoutPaying", pe("MayCastSpellsFromHandWithoutPaying", sp), &ir.Effect{Kind: ir.EffCastWithoutPaying, Filter: spells}},
		{"ExtraLand", pe("MayPlayAnAdditionalLand"), &ir.Effect{Kind: ir.EffExtraLand, N: one}},

		// CastEffect.
		{"AdditionalCastingCost", ce("AdditionalCastingCost", tn("Cost", "PayMana", []any{gen(1)})), &ir.Effect{Kind: ir.EffAdditionalCost, Cost: genCost}},
		{"AlternateCastingCost", ce("AlternateCastingCost", tn("Cost", "PayMana", []any{gen(1)})), &ir.Effect{Kind: ir.EffAlternativeCost, Cost: genCost}},
		{"AlternateCastingCostIf", ce("AlternateCastingCostIf", tn("Cost", "PayMana", []any{gen(1)}), tCond),
			&ir.Effect{Kind: ir.EffAlternativeCost, Cost: genCost, Cond: &ir.Cond{Kind: ir.CKicked}}},
		{"CantBeCastIf", ce("CantBeCastIf", tCond), &ir.Effect{Kind: ir.EffCantCast, Cond: &ir.Cond{Kind: ir.CKicked}}},
		{"ReduceForEach", ce("ReduceCastingCostForEach", reduce(1), tN2),
			&ir.Effect{Kind: ir.EffSelfCostLess, N: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{one, two}}}},
		{"ReduceIf", ce("ReduceCastingCostIf", reduce(2), tCond), &ir.Effect{Kind: ir.EffSelfCostLess, N: two, Cond: &ir.Cond{Kind: ir.CKicked}}},
		{"ReduceIfTargets", ce("ReduceCastingCostIfItTargetsAPermanent", reduce(2), tFilt),
			&ir.Effect{Kind: ir.EffSelfCostLess, N: two, Filter: alFilter()}},
		{"ReduceX", ce("ReduceCastingCostX", []any{tn("CostReductionSymbolX", "CostReduceX")}, tn("TestOnly", "X")),
			&ir.Effect{Kind: ir.EffSelfCostLess, N: &ir.Value{Kind: ir.VX}}},
		{"CantBeCountered", tn("SpellEffect", "CantBeCountered"), &ir.Effect{Kind: ir.EffCantBeCountered}},
	})
	_, err := (&Ctx{}).lower(ce("ReduceCastingCostX", []any{tn("CostReductionSymbolX", "CostReduceX"), tn("CostReductionSymbolX", "CostReduceX")}, tn("TestOnly", "X")))
	if re, ok := err.(*RefusalError); !ok || re.Key != "CastEffect.ReduceCastingCostX:shape" {
		t.Errorf("ReduceCastingCostX shape: %v", err)
	}
}

func TestReplacementLowering(t *testing.T) {
	ssStubs(t)
	self := &ir.Ref{Kind: ir.RefSelf}
	allPl := &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}
	one, two := refConst1(), constV(2)
	rule := func(v string, a ...any) node { return tn("Rule", v, a...) }
	rw := func(v string, a ...any) node { return tn("ReplacementActionWouldEnter", v, a...) }
	paid := tn("Condition", "CostWasPd")
	payLife := &ir.Step{Verb: ir.VerbPayCost, Optional: true, Cost: &ir.Cost{Parts: []*ir.CostPart{{Kind: ir.CostPayLife, N: two}}}}
	tapped := &ir.Step{Verb: ir.VerbModifyEnter, Enter: &ir.Enter{Tapped: true}}
	prog := func(ss ...*ir.Step) *ir.Program { return &ir.Program{Steps: ss} }
	repl := func(tr *ir.Trigger, body *ir.Program) *ir.Ability {
		return &ir.Ability{Kind: ir.AbReplacement, Trigger: tr, Body: body}
	}
	dmgRecip := tn("DamageRecipientsList", "APermanent", tFilt)
	runLowerCases(t, []lowerCase{
		{"AsPermanentEnters", rule("AsPermanentEnters", tRef, []any{rw("EntersTapped")}),
			repl(&ir.Trigger{Event: ir.EvWouldEnter, Ref: self}, prog(tapped))},
		{"ReplaceWouldEnter", rule("ReplaceWouldEnter", tn("ReplacableEventWouldEnter", "APermanentWouldEnterTheBattlefield", tFilt), []any{rw("EntersTapped")}),
			repl(&ir.Trigger{Event: ir.EvWouldEnter, Subject: alFilter()}, prog(tapped))},
		{"ReplaceWouldPutCounters", rule("ReplaceWouldPutCounters", tn("ReplacableEventWouldPutCounters", "AnEffectWouldPutCountersOnAPermanent", tFilt),
			[]any{tn("ReplacementActionWouldPutCounters", "PutNewAmount", tN2)}),
			repl(&ir.Trigger{Event: ir.EvWouldPutCounters, Object: alFilter()}, prog(&ir.Step{Verb: ir.VerbSetAmount, N: two}))},
		{"ReplaceWouldDealDamage", rule("ReplaceWouldDealDamage", tn("ReplacableEventWouldDealDamage", "DamageWouldBeDealtByASourceToARecipient", tn("DamageSources", "Stub"), dmgRecip),
			[]any{tn("ReplacementActionWouldDealDamage", "DealDamageInstead", tN2)}),
			repl(&ir.Trigger{Event: ir.EvWouldDealDamage, Subject: &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PAny}, Object: alFilter()},
				prog(&ir.Step{Verb: ir.VerbSetAmount, N: two}))},
		{"ReplaceTokens", rule("ReplaceAnEffectWouldCreateAnyNumberOfTokens",
			tn("ReplacableEventAnEffectWouldCreateAnyNumberOfTokens", "AnEffectWouldCreateAnyNumberOfTokensUnderAPlayersControl", tFilt, tn("Players", "Stub")),
			[]any{tn("ReplacementActionAnEffectWouldCreateAnyNumberOfTokens", "CreateTokensInstead", []any{tn("CreatableToken", "FoodToken")})}),
			repl(&ir.Trigger{Event: ir.EvWouldCreateTokens, Subject: alFilter(), Object: allPl},
				prog(&ir.Step{Verb: ir.VerbDo, Then: prog(&ir.Step{Verb: ir.VerbCreateToken, N: one,
					Token: &ir.Face{Name: "Food", Types: []string{"Artifact"}, Subtypes: []string{"Food"}}})}))},
		{"PreventCombat", rule("PreventDamage", tn("EventPreventDamage", "CombatDamageWouldBeDealtToRecipient", tRef), []any{tn("ActionPreventDamage", "PreventThatDamage")}),
			repl(&ir.Trigger{Event: ir.EvWouldDealDamage, Ref: self, Word: "combat"}, prog(&ir.Step{Verb: ir.VerbPrevent}))},
		{"PreventNoncombat", rule("PreventDamage", tn("EventPreventDamage", "NoncombatDamageWouldBeDealtToARecipient",
			tn("DamageRecipientsList", "APlayerOrAPermanent", tn("Players", "Stub"), tFilt)), []any{tn("ActionPreventDamage", "PreventThatDamage")}),
			repl(&ir.Trigger{Event: ir.EvWouldDealDamage, Word: "noncombat",
				Object: &ir.Filter{Subject: ir.SubjObject, Op: ir.FAny, Args: []*ir.Filter{allPl, alFilter()}}}, prog(&ir.Step{Verb: ir.VerbPrevent}))},
		{"WouldDie", tn("ReplacableEventWouldPutIntoGraveyard", "APermanentWouldDie", tFilt), &ir.Trigger{Event: ir.EvWouldDie, Subject: alFilter()}},
		{"DamageRecipientsList.APermanent", dmgRecip, alFilter()},

		// Entering actions.
		{"EntersWithACounter", rw("EntersWithACounter", tCounter), &ir.Step{Verb: ir.VerbModifyEnter, Enter: &ir.Enter{Counter: "P1P1", N: one}}},
		{"EntersWithNumberCounters", rw("EntersWithNumberCounters", tN2, tCounter), &ir.Step{Verb: ir.VerbModifyEnter, Enter: &ir.Enter{Counter: "P1P1", N: two}}},
		{"ChooseAColor", rw("ChooseAColor", tn("ChoosableColor", "AnyColor")), &ir.Step{Verb: ir.VerbChooseColor, Colors: allFive}},
		{"ChooseACreatureType", rw("ChooseACreatureType"), &ir.Step{Verb: ir.VerbChooseCreatureType}},
		{"ChooseANumber", rw("ChooseANumberGreaterThanNumber", 0.0), &ir.Step{Verb: ir.VerbChooseNumber, N: constV(0)}},
		{"If", rw("If", tCond, []any{rw("EntersTapped")}), &ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CKicked}, Then: prog(tapped)}},
		{"Unless", rw("Unless", tCond, []any{rw("EntersTapped")}),
			&ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CNot, Args: []*ir.Cond{{Kind: ir.CKicked}}}, Then: prog(tapped)}},
		{"IfPassesFilter", rw("IfPassesFilter", tFilt, []any{rw("EntersTapped")}),
			&ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CRefMatches, Ref: &ir.Ref{Kind: ir.RefEventObject}, Filter: alFilter()}, Then: prog(tapped)}},
		{"MayCost", rw("MayCost", tn("ReplacementActionWouldEnterCost", "PayLife", tN2)), payLife},
		{"ExileItInstead", tn("ReplacementActionWouldPutIntoGraveyard", "ExileItInstead"),
			&ir.Step{Verb: ir.VerbMove, Obj: &ir.Ref{Kind: ir.RefEventObject}, To: ir.ZoneExile}},

		// Hallowed Fountain: pay 2 life, or enter tapped.
		{"Hallowed Fountain", rule("AsPermanentEnters", tn("Permanent", "ThisPermanent"), []any{
			rw("MayCost", tn("ReplacementActionWouldEnterCost", "PayLife", tN2)),
			rw("Unless", paid, []any{rw("EntersTapped")}),
		}), repl(&ir.Trigger{Event: ir.EvWouldEnter, Ref: self}, prog(payLife,
			&ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CNot, Args: []*ir.Cond{{Kind: ir.CCostPaid}}}, Then: prog(tapped)}))},
	})
}

// TestGrantedStaticAbilityFrame: an ability a static grants is lowered in its
// own target frame, and the outer frame is restored after it.
func TestGrantedStaticAbilityFrame(t *testing.T) {
	ssStubs(t)
	c := &Ctx{}
	act := tn("Rule", "Activated", tCost, tn("Actions", "Targeted", []any{targetPlayer()},
		tn("Actions", "ActionList", []any{tn("Action", "LookAtPlayersHand", tn("Player", "Ref_TargetPlayer"))})))
	withStubs(t, map[string]lowerFunc{
		"Rule.Activated": func(c *Ctx, n node) (any, error) {
			cost, err := c.Cost(argN(n, 0))
			if err != nil {
				return nil, err
			}
			body, err := c.Program(argN(n, 1))
			if err != nil {
				return nil, err
			}
			return &ir.Ability{Kind: ir.AbActivated, Cost: cost, Body: body}, nil
		},
	})
	ab, err := c.Ability(tn("Rule", "PermanentLayerEffect", tRef, []any{tn("StaticLayerEffect", "AddAbility", []any{act})}))
	if err != nil {
		t.Fatal(err)
	}
	granted := &ir.Ability{Kind: ir.AbActivated, Cost: &ir.Cost{HasMana: true}, Body: &ir.Program{
		Steps:   []*ir.Step{{Verb: ir.VerbLookAtHand, Obj: &ir.Ref{Kind: ir.RefTarget, Slot: 0}}},
		Targets: []*ir.Target{{Min: 1, Max: 1, Filter: &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}}}}}
	want := &ir.Ability{Kind: ir.AbStatic, Body: static(&ir.Ref{Kind: ir.RefSelf},
		[]*ir.Effect{{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{granted}}})}
	if !ir.Equal(ab, want) {
		t.Errorf("got %s\nwant %s", dump(ab), dump(want))
	}
	if _, ok := c.TargetSlot("Ref_TargetPlayer"); ok {
		t.Error("the granted ability's target leaked into the outer frame")
	}
	if len(c.targets) != 0 {
		t.Errorf("outer frame not restored: %v", c.targets)
	}
}

func TestStaticRuleKeysRegistered(t *testing.T) {
	for _, k := range []string{"PermanentLayerEffect", "EachPermanentLayerEffect", "PermanentRuleEffect", "PlayerEffect",
		"EachPlayerEffect", "CastEffect", "ThisSpellEffect", "CDA_Power", "CDA_Toughness", "CDA_Color",
		"AsPermanentEnters", "ReplaceWouldEnter", "ReplaceWouldPutCounters", "ReplaceWouldDealDamage",
		"ReplaceAnEffectWouldCreateAnyNumberOfTokens", "PreventDamage"} {
		if !Registered("Rule." + k) {
			t.Errorf("Rule.%s is not registered", k)
		}
	}
}
