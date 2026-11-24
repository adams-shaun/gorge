package mtgish

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// actionCoverage checks the proof slice's Action.<V> keys whose first letter
// lies in [lo, hi]: each must be registered or deferred. CC-15b reuses it.
func actionCoverage(t *testing.T, lo, hi byte, deferred map[string]string) {
	t.Helper()
	var census []VariantCount
	for _, vc := range Census(proofSlice(t)) {
		enum, v, _ := cutKey(vc.Key)
		if enum == "Action" && v != "" && lo <= v[0] && v[0] <= hi {
			census = append(census, vc)
		}
	}
	for _, p := range familyCoverageProblems(census, []string{"Action"}, deferred, Registered) {
		t.Error(p)
	}
}

func cutKey(k string) (enum, variant string, ok bool) {
	for i := 0; i < len(k); i++ {
		if k[i] == '.' {
			return k[:i], k[i+1:], true
		}
	}
	return k, "", false
}

func TestActionALSliceVariantsRegistered(t *testing.T) {
	actionCoverage(t, 'A', 'L', map[string]string{
		"Action.AddManaWithModifiers":                         "spend restrictions",
		"Action.BecomeTheMonarch":                             "monarch",
		"Action.BeginGameWithCardOnBattlefield":               "pregame actions",
		"Action.CastAnyNumberOfSpellsFromExileWithoutPaying":  "free casts from exile",
		"Action.ChooseACardName":                              "needs CardsInOracle and the chosen-name readers",
		"Action.ChooseAPermanentOfEachPermanentTypeAvailable": "per-type choice",
		"Action.EachPlayerMayCostOrFallback":                  "per-player optional costs",
		"Action.EachPlayerStartingWithAction":                 "turn-order iteration",
	})
}

func TestProgramSliceVariantsRegistered(t *testing.T) {
	checkFamilyCoverage(t, []string{"Actions", "ManaProduce", "ChoosableColor", "Expiration", "FutureTrigger",
		"LayerEffect", "GraveyardCardEffect", "ModX", "PT", "CreatableToken", "TokenColorList", "TokenSubtypes",
		"TokenFlag", "TokenCopyEffects", "TokenCopyEffect", "ExileFlag", "LookAtTopOfLibraryAction"},
		map[string]string{
			"Actions.TargetedDistributed":                                    "dividing damage",
			"ManaProduce.ManaOfTheChosenColor":                               "chosen-color mana",
			"LookAtTopOfLibraryAction.APlayerChoosesAPileTopPutIntoHand":     "piles",
			"LookAtTopOfLibraryAction.SeparateIntoFaceUpFileAndFaceDownPile": "piles",
			"Expiration.UntilAPlayerBecomesTheMonarch":                       "monarch",
		})
}

func TestActionLoweringAL(t *testing.T) {
	alStubs(t, map[string]lowerFunc{
		"Player.Stub":        func(*Ctx, node) (any, error) { return &ir.Ref{Kind: ir.RefYou}, nil },
		"Permanents.Stub":    func(*Ctx, node) (any, error) { return alFilter(), nil },
		"SettableColor.Stub": func(*Ctx, node) (any, error) { return ir.ColorR, nil },
		"PermanentRule.Stub": func(*Ctx, node) (any, error) { return &ir.Effect{Kind: ir.EffCantBlock}, nil },
		"PlayerEffect.Stub":  func(*Ctx, node) (any, error) { return &ir.Effect{Kind: ir.EffNoMaxHandSize}, nil },
	})
	self := &ir.Ref{Kind: ir.RefSelf}
	you := &ir.Ref{Kind: ir.RefYou}
	one, two := refConst1(), constV(2)
	allF := &ir.Ref{Kind: ir.RefAll, Filter: alFilter()}
	hand := &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: one}
	wuC := func(cs ir.ColorSet) []ir.ManaSym { return []ir.ManaSym{{Kind: ir.ManaColored, Colors: cs}} }
	effHex := &ir.Effect{Kind: ir.EffHexproof}
	trDies := &ir.Trigger{Event: ir.EvDies}
	ref := tn("TestOnly", "Ref")
	filt := tFilt
	mp := func(v string) node { return tn("ManaProduce", v) }
	layer := func(v string, a ...any) node { return tn("LayerEffect", v, a...) }
	step := func(s ir.Step) *ir.Step { return &s }
	adjPT := func(p, q int32) *ir.Effect {
		return &ir.Effect{Kind: ir.EffAdjustPT, P: constV(p), T: constV(q)}
	}
	food := &ir.Face{Name: "Food", Types: []string{"Artifact"}, Subtypes: []string{"Food"}}
	token := &ir.Face{ColorIndicator: ir.ColorW, Types: []string{"Creature"}, Subtypes: []string{"Human"},
		Power: &ir.PT{N: 1}, Toughness: &ir.PT{N: 1}}
	tokNode := func() node {
		return tn("CreatableToken", "TokenWithPT", tn("PT", "PT", 1.0, 1.0),
			tn("TokenColorList", "Colors", []any{"White"}), []any{}, []any{"Creature"},
			tn("TokenSubtypes", "SubtypesList", []any{"Human"}), []any{})
	}
	pt := func(p, q any) node { return tn("PT", "PT", p, q) }

	cases := []struct {
		name string
		in   node
		want *ir.Step
	}{
		{"AddMana", tn("Action", "AddMana", mp("ManaProduceW")), step(ir.Step{Verb: ir.VerbAddMana, Mana: wuC(ir.ColorW)})},
		{"AddMana And", tn("Action", "AddMana", tn("ManaProduce", "And", []any{mp("ManaProduceC"), mp("ManaProduceG")})),
			step(ir.Step{Verb: ir.VerbAddMana, Mana: []ir.ManaSym{{Kind: ir.ManaColorless}, {Kind: ir.ManaColored, Colors: ir.ColorG}}})},
		{"AddMana Or", tn("Action", "AddMana", tn("ManaProduce", "Or", []any{mp("ManaProduceU"), mp("ManaProduceB")})),
			step(ir.Step{Verb: ir.VerbAddMana, Mana: wuC(ir.ColorU | ir.ColorB)})},
		{"AddMana any", tn("Action", "AddMana", mp("AnyManaColor")),
			step(ir.Step{Verb: ir.VerbAddMana, Mana: wuC(allFive)})},
		{"AddMana each colour", tn("Action", "AddMana", tn("ManaProduce", "EachManaColorAmongPermanents", tn("Permanents", "Stub"))),
			step(ir.Step{Verb: ir.VerbAddMana, Word: "each-color-among", Filter: alFilter()})},
		{"AddManaRepeated", tn("Action", "AddManaRepeated", mp("ManaProduceR"), tN2),
			step(ir.Step{Verb: ir.VerbAddMana, Mana: wuC(ir.ColorR), N: two})},
		{"Attach", tn("Action", "AttachPermanentToPermanent", ref, ref), step(ir.Step{Verb: ir.VerbAttach, Obj: self, Other: self})},
		{"ChooseAColor", tn("Action", "ChooseAColor", tn("ChoosableColor", "AnyColor")), step(ir.Step{Verb: ir.VerbChooseColor, Colors: allFive})},
		{"ChooseAnExiledCard", tn("Action", "ChooseAnExiledCard", filt),
			step(ir.Step{Verb: ir.VerbChoose, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: alFilter(), N: one}})},
		{"CopySpell", tn("Action", "CopySpellAndMayChooseNewTargets", ref), step(ir.Step{Verb: ir.VerbCopySpell, Obj: self})},
		{"CopySpellForEach", tn("Action", "CopySpellForEachAndMayChooseNewTargets", ref, tN2), step(ir.Step{Verb: ir.VerbCopySpell, Obj: self, N: two})},
		{"CounterSpell", tn("Action", "CounterSpell", ref), step(ir.Step{Verb: ir.VerbCounter, Obj: self})},
		{"EachPermLayerUntil", tn("Action", "CreateEachPermanentLayerEffectUntil", filt, []any{tEff}, tExp),
			step(ir.Step{Verb: ir.VerbApplyEffect, Obj: allF, Effects: []*ir.Effect{effHex}, Dur: ir.DurEndOfTurn})},
		{"GraveyardEffectUntil", tn("Action", "CreateGraveyardCardEffectUntil", ref, []any{tEff}, tExp),
			step(ir.Step{Verb: ir.VerbApplyEffect, Obj: self, Effects: []*ir.Effect{effHex}, Dur: ir.DurEndOfTurn})},
		{"PermLayer", tn("Action", "CreatePermanentLayerEffect", ref, []any{tEff}),
			step(ir.Step{Verb: ir.VerbApplyEffect, Obj: self, Effects: []*ir.Effect{effHex}, Dur: ir.DurPermanent})},
		{"PermLayerUntil", tn("Action", "CreatePermanentLayerEffectUntil", ref, []any{tEff}, tExp),
			step(ir.Step{Verb: ir.VerbApplyEffect, Obj: self, Effects: []*ir.Effect{effHex}, Dur: ir.DurEndOfTurn})},
		{"PermRuleUntil", tn("Action", "CreatePermanentRuleEffectUntil", ref, []any{tn("PermanentRule", "Stub")}, tExp),
			step(ir.Step{Verb: ir.VerbApplyEffect, Obj: self, Effects: []*ir.Effect{{Kind: ir.EffCantBlock}}, Dur: ir.DurEndOfTurn})},
		{"PlayerEffectUntil", tn("Action", "CreatePlayerEffectUntil", tn("Player", "Stub"), []any{tn("PlayerEffect", "Stub")}, tExp),
			step(ir.Step{Verb: ir.VerbApplyEffect, Obj: you, Effects: []*ir.Effect{{Kind: ir.EffNoMaxHandSize}}, Dur: ir.DurEndOfTurn})},
		{"FutureTrigger", tn("Action", "CreateFutureTrigger", tTrig, tn("Actions", "ActionList", []any{tn("Action", "DrawACard")})),
			step(ir.Step{Verb: ir.VerbDelayedTrigger, Abilities: []*ir.Ability{{Kind: ir.AbTriggered, Trigger: trDies,
				Body: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbDraw, N: one}}}}}})},
		{"FutureTrigger next end step", tn("Action", "CreateFutureTrigger", tn("FutureTrigger", "AtTheBeginningOfTheNextEndStep"), tn("Actions", "ActionList", []any{})),
			step(ir.Step{Verb: ir.VerbDelayedTrigger, Abilities: []*ir.Ability{{Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvNextEndStep}, Body: &ir.Program{}}}})},
		{"PreventDamageUntil", tn("Action", "CreatePreventDamageUntil", tTrig, []any{tn("Action", "DrawACard")}, tExp),
			step(ir.Step{Verb: ir.VerbCreateReplacement, Dur: ir.DurEndOfTurn, Abilities: []*ir.Ability{{Kind: ir.AbReplacement, Trigger: trDies,
				Body: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbDraw, N: one}}}}}})},
		{"ReplaceWouldPutIntoGraveyardUntil", tn("Action", "CreateReplaceWouldPutIntoGraveyardUntil", tTrig, []any{}, tExp),
			step(ir.Step{Verb: ir.VerbCreateReplacement, Dur: ir.DurEndOfTurn, Abilities: []*ir.Ability{{Kind: ir.AbReplacement, Trigger: trDies, Body: &ir.Program{}}}})},
		{"DestroyEach", tn("Action", "DestroyEachPermanent", filt), step(ir.Step{Verb: ir.VerbDestroy, Obj: allF})},
		{"DestroyPermanent", tn("Action", "DestroyPermanent", ref), step(ir.Step{Verb: ir.VerbDestroy, Obj: self})},
		{"DiscardACard", tn("Action", "DiscardACard"), step(ir.Step{Verb: ir.VerbDiscard, Obj: hand})},
		{"DiscardCards", tn("Action", "DiscardCards", filt), step(ir.Step{Verb: ir.VerbDiscard, Obj: allF})},
		{"DiscardNumberCards", tn("Action", "DiscardNumberCards", tN2),
			step(ir.Step{Verb: ir.VerbDiscard, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: two}})},
		{"DoublePower", tn("Action", "DoubleCreaturesPowerUntilEndOfTurn", ref),
			step(ir.Step{Verb: ir.VerbApplyEffect, Obj: self, Dur: ir.DurEndOfTurn,
				Effects: []*ir.Effect{{Kind: ir.EffAdjustPT, P: &ir.Value{Kind: ir.VPower, Ref: self}, T: constV(0)}}})},
		{"DrawACard", tn("Action", "DrawACard"), step(ir.Step{Verb: ir.VerbDraw, N: one})},
		{"DrawNumberCards", tn("Action", "DrawNumberCards", tN2), step(ir.Step{Verb: ir.VerbDraw, N: two})},
		{"EachPlayerAction", tn("Action", "EachPlayerAction", tn("Players", "Stub"), tn("Action", "DrawACard")),
			step(ir.Step{Verb: ir.VerbForEachPlayer,
				Obj:  &ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}},
				Then: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbDraw, N: one, Who: &ir.Ref{Kind: ir.RefIterPlayer}}}}})},
		{"EndTheTurn", tn("Action", "EndTheTurn"), step(ir.Step{Verb: ir.VerbEndTurn})},
		{"Fight", tn("Action", "Fight", ref, ref), step(ir.Step{Verb: ir.VerbFight, Source: self, Obj: self})},
		{"GainControlUntil", tn("Action", "GainControlOfPermanentUntil", ref, tExp), step(ir.Step{Verb: ir.VerbGainControl, Obj: self, Dur: ir.DurEndOfTurn})},
		{"GainLife", tn("Action", "GainLife", tN2), step(ir.Step{Verb: ir.VerbGainLife, N: two})},
		{"GainLifeForEach", tn("Action", "GainLifeForEach", tN2, tN1),
			step(ir.Step{Verb: ir.VerbGainLife, N: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{two, one}}})},
		{"GetAnEmblem", tn("Action", "GetAnEmblem", []any{tRule}), step(ir.Step{Verb: ir.VerbEmblem, Abilities: []*ir.Ability{{Kind: ir.AbSpell}}})},
		{"If", tn("Action", "If", tCond, []any{tn("Action", "DrawACard")}),
			step(ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CKicked}, Then: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbDraw, N: one}}}})},
		{"IfElse", tn("Action", "IfElse", tCond, []any{tn("Action", "DrawACard")}, []any{tn("Action", "EndTheTurn")}),
			step(ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CKicked},
				Then: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbDraw, N: one}}},
				Else: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbEndTurn}}}})},
		{"LookAtHand", tn("Action", "LookAtPlayersHand", ref), step(ir.Step{Verb: ir.VerbLookAtHand, Obj: self})},
		{"LookAtTopCardOfPlayers", tn("Action", "LookAtTheTopCardOfPlayersLibrary", ref),
			step(ir.Step{Verb: ir.VerbLook, Obj: &ir.Ref{Kind: ir.RefTopOfLibrary, Of: self, N: one}})},
		{"LookAtTopOfLibrary", tn("Action", "LookAtTopOfLibrary"), step(ir.Step{Verb: ir.VerbLook, Obj: &ir.Ref{Kind: ir.RefTopOfLibrary, N: one}})},
		{"LookAtTheTopNumber", tn("Action", "LookAtTheTopNumberCardsOfLibrary", tN2, []any{
			tn("LookAtTopOfLibraryAction", "MayRevealACardOfTypeAndPutIntoHand", filt),
			tn("LookAtTopOfLibraryAction", "MayRevealAndPutACardOfTypeOntoTheBattlefield", filt, []any{tn("TokenFlag", "EntersTapped")}),
			tn("LookAtTopOfLibraryAction", "MayPutAGenericCardOnTopOfLibrary"),
			tn("LookAtTopOfLibraryAction", "PutTheRemainingCardsIntoGraveyard"),
			tn("LookAtTopOfLibraryAction", "PutTheRemainingCardsOnTheBottomOfLibraryInARandomOrder"),
			tn("LookAtTopOfLibraryAction", "PutTheRemainingCardsOnTheBottomOfLibraryInAnyOrder"),
			tn("LookAtTopOfLibraryAction", "PutTheRemainingCardsOnTopOfLibraryInAnyOrder"),
		}), step(ir.Step{Verb: ir.VerbLook, Obj: &ir.Ref{Kind: ir.RefTopOfLibrary, N: two}, Then: &ir.Program{Steps: []*ir.Step{
			{Verb: ir.VerbMove, Optional: true, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: alFilter(), N: one}, To: ir.ZoneHand, Word: "reveal"},
			{Verb: ir.VerbMove, Optional: true, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: alFilter(), N: one}, To: ir.ZoneBattlefield, Word: "reveal", Enter: &ir.Enter{Tapped: true}},
			{Verb: ir.VerbMove, Optional: true, Obj: &ir.Ref{Kind: ir.RefChoose, N: one}, To: ir.ZoneLibrary, Word: "top"},
			{Verb: ir.VerbMove, To: ir.ZoneGraveyard},
			{Verb: ir.VerbMove, To: ir.ZoneLibrary, Word: "bottom-random"},
			{Verb: ir.VerbMove, To: ir.ZoneLibrary, Word: "bottom-any-order"},
			{Verb: ir.VerbMove, To: ir.ZoneLibrary, Word: "top-any-order"},
		}}})},
		{"LoseLife", tn("Action", "LoseLife", tN2), step(ir.Step{Verb: ir.VerbLoseLife, N: two})},

		// Expirations.
		{"Exp UntilPlayersNextTurn", tn("Action", "GainControlOfPermanentUntil", ref, tn("Expiration", "UntilPlayersNextTurn", tn("Player", "Stub"))),
			step(ir.Step{Verb: ir.VerbGainControl, Obj: self, Dur: ir.DurUntilYourNextTurn})},
		{"Exp UntilTheEndOfPlayersNextTurn", tn("Action", "GainControlOfPermanentUntil", ref, tn("Expiration", "UntilTheEndOfPlayersNextTurn", tn("Player", "Stub"))),
			step(ir.Step{Verb: ir.VerbGainControl, Obj: self, Dur: ir.DurUntilEndOfYourNextTurn})},
		{"Exp UntilPermanentLeavesBattlefield", tn("Action", "GainControlOfPermanentUntil", ref, tn("Expiration", "UntilPermanentLeavesBattlefield", ref)),
			step(ir.Step{Verb: ir.VerbGainControl, Obj: self, Dur: ir.DurWhileSourceOnBattlefield})},

		// Exile and its flags.
		{"Exile", tn("Action", "Exile", []any{tn("Exilable", "Permanent", ref)}, []any{}),
			step(ir.Step{Verb: ir.VerbExile, Obj: self})},
	}
	for _, tc := range cases {
		p, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tc.in}))
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if len(p.Steps) != 1 || !ir.Equal(p.Steps[0], tc.want) {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, p, tc.want)
		}
	}

	// Splicing rows.
	progCases := []struct {
		name string
		in   node
		want []*ir.Step
	}{
		{"Exile flags", tn("Action", "Exile", []any{tn("Exilable", "Permanent", ref)}, []any{
			tn("ExileFlag", "ExilesWithACounter", tCounter), tn("ExileFlag", "Limited", tExp)}),
			[]*ir.Step{{Verb: ir.VerbExile, Obj: self, Word: "P1P1", Dur: ir.DurEndOfTurn}}},
		{"CreateTokens", tn("Action", "CreateTokens", []any{
			tn("CreatableToken", "NumberTokens", tN2, tokNode()),
			tn("CreatableToken", "FoodToken"), tn("CreatableToken", "TreasureToken"),
			tn("CreatableToken", "NamedTokenWithPT", "Soldier", pt(2.0, 3.0), tn("TokenColorList", "Colorless"), []any{"Legendary"}, []any{"Artifact"},
				tn("TokenSubtypes", "SubtypesList", []any{"Golem"}), []any{tRule}),
			tn("CreatableToken", "TokenCopyOfPermanent", ref, tn("TokenCopyEffects", "NoTokenCopyEffects")),
			tn("CreatableToken", "TokenCopyOfGraveyardCard", ref, tn("TokenCopyEffects", "TokenCopyEffects", []any{
				tn("TokenCopyEffect", "AddAbility", []any{tRule}), tn("TokenCopyEffect", "AddCreatureTypes", []any{"Zombie"})})),
		}, []any{tn("TokenFlag", "EntersTapped")}),
			[]*ir.Step{
				{Verb: ir.VerbCreateToken, N: two, Token: token, Enter: &ir.Enter{Tapped: true}},
				{Verb: ir.VerbCreateToken, N: one, Token: food, Enter: &ir.Enter{Tapped: true}},
				{Verb: ir.VerbCreateToken, N: one, Token: &ir.Face{Name: "Treasure", Types: []string{"Artifact"}, Subtypes: []string{"Treasure"}}, Enter: &ir.Enter{Tapped: true}},
				{Verb: ir.VerbCreateToken, N: one, Enter: &ir.Enter{Tapped: true}, Token: &ir.Face{Name: "Soldier", Supertypes: []string{"Legendary"}, Types: []string{"Artifact"},
					Subtypes: []string{"Golem"}, Power: &ir.PT{N: 2}, Toughness: &ir.PT{N: 3}, Abilities: []*ir.Ability{{Kind: ir.AbSpell}}}},
				{Verb: ir.VerbCreateTokenCopy, N: one, Obj: self, Enter: &ir.Enter{Tapped: true}},
				{Verb: ir.VerbCreateTokenCopy, N: one, Obj: self, Enter: &ir.Enter{Tapped: true}, Effects: []*ir.Effect{
					{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{Kind: ir.AbSpell}}}, {Kind: ir.EffAddSubtype, Words: []string{"Zombie"}}}},
			}},
	}
	for _, tc := range progCases {
		p, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tc.in}))
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if !ir.Equal(p, &ir.Program{Steps: tc.want}) {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, p, &ir.Program{Steps: tc.want})
		}
	}

	// Layer effects.
	layerCases := []struct {
		name string
		in   node
		want *ir.Effect
	}{
		{"AddAbility", layer("AddAbility", []any{tRule}), &ir.Effect{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{Kind: ir.AbSpell}}}},
		{"AddCardtype", layer("AddCardtype", "Artifact"), &ir.Effect{Kind: ir.EffAddCardType, Words: []string{"Artifact"}}},
		{"AddCardtypes", layer("AddCardtypes", []any{"Artifact", "Creature"}), &ir.Effect{Kind: ir.EffAddCardType, Words: []string{"Artifact", "Creature"}}},
		{"AddCreatureType", layer("AddCreatureType", "Zombie"), &ir.Effect{Kind: ir.EffAddSubtype, Words: []string{"Zombie"}}},
		{"AdjustPT", layer("AdjustPT", 2.0, -1.0), adjPT(2, -1)},
		{"AdjustPTForEach", layer("AdjustPTForEach", 1.0, 2.0, tN2),
			&ir.Effect{Kind: ir.EffAdjustPT, P: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{constV(1), two}}, T: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{constV(2), two}}}},
		{"AdjustPTX", layer("AdjustPTX", tn("ModX", "PlusX"), tn("ModX", "PlusX"), tn("TestOnly", "X")),
			&ir.Effect{Kind: ir.EffAdjustPT, P: &ir.Value{Kind: ir.VX}, T: &ir.Value{Kind: ir.VX}}},
		{"HasAllCreatureTypes", layer("HasAllCreatureTypes"), &ir.Effect{Kind: ir.EffAllCreatureTypes}},
		{"SetColor", layer("SetColor", tn("SettableColor", "Stub")), &ir.Effect{Kind: ir.EffSetColor, Colors: ir.ColorR}},
		{"SetCreatureType", layer("SetCreatureType", "Elf"), &ir.Effect{Kind: ir.EffSetSubtype, Words: []string{"Elf"}}},
		{"SetCreatureTypes", layer("SetCreatureTypes", []any{"Elf", "Druid"}), &ir.Effect{Kind: ir.EffSetSubtype, Words: []string{"Elf", "Druid"}}},
		{"SetPT", layer("SetPT", pt(4.0, 4.0)), &ir.Effect{Kind: ir.EffSetPT, P: constV(4), T: constV(4)}},
		{"SetPT manual", layer("SetPT", tn("PT", "ManualPT", tN2, tN1)), &ir.Effect{Kind: ir.EffSetPT, P: two, T: one}},
		{"SetPowerAndToughnessBoth", layer("SetPowerAndToughnessBoth", tN2), &ir.Effect{Kind: ir.EffSetPT, P: two, T: two}},
		{"GraveyardCardEffect.AddAbility", tn("GraveyardCardEffect", "AddAbility", []any{tRule}), &ir.Effect{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{Kind: ir.AbSpell}}}},
	}
	for _, tc := range layerCases {
		got, err := (&Ctx{}).Effect(tc.in)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if !ir.Equal(got, tc.want) {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}

	// Refusals.
	refusals := []struct {
		name string
		in   node
		key  string
	}{
		{"Or shape", tn("Action", "AddMana", tn("ManaProduce", "Or", []any{mp("ManaProduceC"), mp("ManaProduceG")})), "ManaProduce.Or:shape"},
		{"UntilPlayersNextTurn player", tn("Action", "GainControlOfPermanentUntil", ref, tn("Expiration", "UntilPlayersNextTurn", ref)), "Expiration.UntilPlayersNextTurn:player"},
		{"UntilTheEndOfPlayersNextTurn player", tn("Action", "GainControlOfPermanentUntil", ref, tn("Expiration", "UntilTheEndOfPlayersNextTurn", ref)), "Expiration.UntilTheEndOfPlayersNextTurn:player"},
		{"AdjustPTX mod", tn("Action", "CreatePermanentLayerEffect", ref, []any{layer("AdjustPTX", tn("ModX", "MinusX"), tn("ModX", "PlusX"), tn("TestOnly", "X"))}), "LayerEffect.AdjustPTX:mod"},
		{"manual PT", tn("Action", "CreateTokens", []any{tn("CreatableToken", "TokenWithPT", tn("PT", "ManualPT", tN2, tN1),
			tn("TokenColorList", "Colorless"), []any{}, []any{"Creature"}, tn("TokenSubtypes", "SubtypesList", []any{}), []any{})}, []any{}),
			"CreatableToken.TokenWithPT:manual-pt"},
	}
	for _, tc := range refusals {
		_, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tc.in}))
		re, ok := err.(*RefusalError)
		if !ok || re.Key != tc.key {
			t.Errorf("%s: error = %v, want refusal %s", tc.name, err, tc.key)
		}
	}
	if _, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tn("Action", "GainControlOfPermanentUntil", ref,
		tn("Expiration", "UntilPermanentLeavesBattlefield", tn("Permanent", "Trigger_ThatPermanent")))})); err == nil {
		t.Error("UntilPermanentLeavesBattlefield on a non-self permanent must refuse")
	}
	// Two exile flags setting one field.
	_, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tn("Action", "Exile", []any{tn("Exilable", "Permanent", ref)},
		[]any{tn("ExileFlag", "Limited", tExp), tn("ExileFlag", "Limited", tExp)})}))
	if err == nil {
		t.Error("two Limited flags must conflict")
	}
}
