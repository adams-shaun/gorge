package mtgish

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func TestActionMZSliceVariantsRegistered(t *testing.T) {
	actionCoverage(t, 'M', 'Z', map[string]string{
		"Action.PermanentDealsDistributedDamage": "needs Actions.TargetedDistributed",
		"Action.RepeatThisProcess":               "a loop construct",
		"Action.RepeatableActions":               "a loop construct",
		"Action.RestartTheGameLeavingEachExiledCardInExileThenPutThoseCardsOntoTheBattlefield": "restart",
		"Action.RevealCardFromOutsideGameAndPutInHand":                                         "outside the game",
		"Action.RevealCardFromOutsideGameAndPutInHandOrPutAnExiledCardInOwnersHand":            "outside the game",
		"Action.VoteForAPermanent": "voting",
	})
}

func TestActionMZSatellitesRegistered(t *testing.T) {
	checkFamilyCoverage(t, []string{"EnterFlag", "PutCountersAction", "RemoveCountersAction",
		"RevealTheTopNumberCardsOfLibraryAction", "SearchLibraryAction", "DamageToRecipients"}, map[string]string{})
}

func mzStubs(t *testing.T) {
	t.Helper()
	alStubs(t, map[string]lowerFunc{
		"Player.Stub": func(*Ctx, node) (any, error) { return &ir.Ref{Kind: ir.RefYou}, nil },
		"Target.Stub": func(*Ctx, node) (any, error) { return &ir.Ref{Kind: ir.RefSelf}, nil },
	})
	// The real EnterFlag lowerings replace alStubs' stand-ins.
	delete(overlay, "EnterFlag.EntersNormally")
	delete(overlay, "EnterFlag.EntersUnderOwnersControl")
	delete(overlay, "EnterFlag.WithACounter")
	delete(overlay, "EnterFlag.WithOtherCounter")
}

func TestActionLoweringMZ(t *testing.T) {
	mzStubs(t)
	self := &ir.Ref{Kind: ir.RefSelf}
	you := &ir.Ref{Kind: ir.RefYou}
	one, two := refConst1(), constV(2)
	allF := alFilter()
	allR := &ir.Ref{Kind: ir.RefAll, Filter: allF}
	chooseF := &ir.Ref{Kind: ir.RefChoose, Filter: allF, N: one}
	hand := &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: one}
	found := &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcFound}
	ref, filt := tn("TestOnly", "Ref"), tFilt
	pl := tn("Player", "Stub")
	step := func(s ir.Step) *ir.Step { return &s }
	draw := step(ir.Step{Verb: ir.VerbDraw, N: one})
	tapped := []any{tn("EnterFlag", "EntersTapped")}
	enTapped := &ir.Enter{Tapped: true}
	dmg := step(ir.Step{Verb: ir.VerbDealDamage, Source: self, N: two, Obj: self})
	ctr := tn("CounterType", "ChargeCounter")
	topOf := func(of *ir.Ref, n *ir.Value) *ir.Ref { return &ir.Ref{Kind: ir.RefTopOfLibrary, Of: of, N: n} }
	mv := func(obj *ir.Ref, to ir.Zone, word string, en *ir.Enter) *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, Obj: obj, To: to, Word: word, Enter: en}
	}
	ownedBy := func(r *ir.Ref) *ir.Filter {
		return &ir.Filter{Subject: ir.SubjCard, Op: ir.FPred, Pred: ir.POwnedBy, Ref: r}
	}
	pilfer := func(verb ir.Verb) *ir.Step {
		return &ir.Step{Verb: ir.VerbRevealHand, Then: &ir.Program{Steps: []*ir.Step{{Verb: verb,
			Obj: &ir.Ref{Kind: ir.RefChoose, Of: you, Filter: withZone(allF, ir.ZoneHand), N: one}}}}}
	}
	A := func(v string, a ...any) node { return tn("Action", v, a...) }

	cases := []struct {
		name string
		in   node
		want *ir.Step
	}{
		{"MayAction", A("MayAction", drawA()), step(ir.Step{Verb: ir.VerbDraw, N: one, Optional: true})},
		{"MayActions", A("MayActions", []any{drawA(), A("EndTheTurn")}),
			step(ir.Step{Verb: ir.VerbDo, Optional: true, Then: &ir.Program{Steps: []*ir.Step{draw, {Verb: ir.VerbEndTurn}}}})},
		{"MayCost", A("MayCost", tCost), step(ir.Step{Verb: ir.VerbPayCost, Optional: true, Cost: &ir.Cost{HasMana: true}})},
		{"MustCost", A("MustCost", tCost), step(ir.Step{Verb: ir.VerbPayCost, Cost: &ir.Cost{HasMana: true}})},
		{"Mill", A("MillNumberCards", tN2), step(ir.Step{Verb: ir.VerbMill, N: two})},
		{"PermDamage", A("PermanentDealsDamage", ref, tN2, ref), dmg},
		{"SpellDamage", A("SpellDealsDamage", ref, tN2, ref), dmg},
		{"PlayerActions", A("PlayerActions", pl, []any{drawA()}),
			step(ir.Step{Verb: ir.VerbDo, Who: you, Then: &ir.Program{Steps: []*ir.Step{draw}}})},
		{"PlayerMayAction", A("PlayerMayAction", pl, drawA()), step(ir.Step{Verb: ir.VerbDraw, N: one, Optional: true, Who: you})},
		{"PlayerMayCost", A("PlayerMayCost", pl, tCost), step(ir.Step{Verb: ir.VerbPayCost, Optional: true, Who: you, Cost: &ir.Cost{HasMana: true}})},
		{"PutACardFromGraveyardIntoHand", A("PutACardFromGraveyardIntoHand", filt),
			mv(&ir.Ref{Kind: ir.RefChoose, Filter: &ir.Filter{Subject: allF.Subject, Zones: allF.Zones, Op: ir.FAll, Args: []*ir.Filter{allF,
				{Subject: allF.Subject, Zones: allF.Zones, Op: ir.FPred, Pred: ir.POwnedBy, Ref: you}}}, N: one}, ir.ZoneHand, "", nil)},
		{"PutACardFromHandOnBattlefield", A("PutACardFromHandOnBattlefield", filt, tapped), mv(chooseF, ir.ZoneBattlefield, "", enTapped)},
		{"PutEachCardFromEachPlayersGraveyard", A("PutEachCardFromEachPlayersGraveyardOntoTheBattlefield", filt, tn("Players", "Stub"), tapped),
			mv(&ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{Op: ir.FAll, Args: []*ir.Filter{
				withZone(allF, ir.ZoneGraveyard),
				ownedBy(&ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}})}}},
				ir.ZoneBattlefield, "", enTapped)},
		{"PutEachGraveyardCardIntoHand", A("PutEachGraveyardCardIntoHand", filt), mv(allR, ir.ZoneHand, "", nil)},
		{"PutEachPermanentIntoItsOwnersHand", A("PutEachPermanentIntoItsOwnersHand", filt), mv(allR, ir.ZoneHand, "", nil)},
		{"PutEachPermanentOnBottom", A("PutEachPermanentOnBottomOfOwnersLibraryInOrderOfOwnersChoice", filt), mv(allR, ir.ZoneLibrary, "bottom-any-order", nil)},
		{"PutExiledCardOntoBattlefield", A("PutExiledCardOntoBattlefield", ref, tapped), mv(self, ir.ZoneBattlefield, "", enTapped)},
		{"PutGraveyardCardIntoHand", A("PutGraveyardCardIntoHand", ref), mv(self, ir.ZoneHand, "", nil)},
		{"PutGraveyardCardOntoBattlefield", A("PutGraveyardCardOntoBattlefield", ref, tapped), mv(self, ir.ZoneBattlefield, "", enTapped)},
		{"PutNumCardsFromHandOnTop", A("PutNumCardsFromHandOnTopOfLibraryAnyOrder", tN2),
			mv(&ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: two}, ir.ZoneLibrary, "top-any-order", nil)},
		{"PutPermanentIntoItsOwnersHand", A("PutPermanentIntoItsOwnersHand", ref), mv(self, ir.ZoneHand, "", nil)},
		{"PutPermanentOnBottom", A("PutPermanentOnBottomOfOwnersLibrary", ref), mv(self, ir.ZoneLibrary, "bottom", nil)},
		{"PutTheTopCardOnBottom", A("PutTheTopCardOfPlayersLibraryOnTheBottomOfTheirLibrary", pl), mv(topOf(you, one), ir.ZoneLibrary, "bottom", nil)},
		{"PutTopOfLibraryInHand", A("PutTopOfLibraryInHand"), mv(topOf(nil, one), ir.ZoneHand, "", nil)},
		{"Regenerate", A("RegeneratePermanent", ref), step(ir.Step{Verb: ir.VerbRegenerate, Obj: self})},
		{"ReturnDead", A("ReturnDeadGraveyardCardToBattlefield", tapped),
			step(ir.Step{Verb: ir.VerbMove, Obj: &ir.Ref{Kind: ir.RefEventObject}, From: ir.ZoneGraveyard, To: ir.ZoneBattlefield, Enter: enTapped})},
		{"ReturnEachCard", A("ReturnEachCardFromPlayersGraveyardToBattlefield", filt, pl, tapped),
			mv(&ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{Op: ir.FAll, Args: []*ir.Filter{allF, ownedBy(you)}}}, ir.ZoneBattlefield, "", enTapped)},
		{"ReturnGraveyardCardToHand", A("ReturnGraveyardCardToHand", ref), mv(self, ir.ZoneHand, "", nil)},
		{"ReturnPermanentToTopOrBottom", A("ReturnPermanentToTopOrBottomOfLibrary", ref), mv(self, ir.ZoneLibrary, "top-or-bottom", nil)},
		{"RevealACardFromHand", A("RevealACardFromHand"), step(ir.Step{Verb: ir.VerbReveal, Obj: hand})},
		{"RevealUntil", A("RevealCardsFromTheTopOfLibraryUntilACardOfTypeIsRevealed", filt, []any{tn("RevealTheTopNumberCardsOfLibraryAction", "PutTheRemainingCardsIntoGraveyard")}),
			step(ir.Step{Verb: ir.VerbRevealUntil, Filter: allF, Then: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbMove, To: ir.ZoneGraveyard}}}})},
		{"RevealHand", A("RevealHand"), step(ir.Step{Verb: ir.VerbRevealHand})},
		{"RevealHandDiscard", A("RevealHandAndPlayerChoosesACardToDiscard", pl, filt), pilfer(ir.VerbDiscard)},
		{"RevealHandExile", A("RevealHandAndPlayerChoosesACardToExile", pl, filt), pilfer(ir.VerbExile)},
		{"RevealTheTopNumber", A("RevealTheTopNumberCardsOfLibrary", tN2, []any{
			tn("RevealTheTopNumberCardsOfLibraryAction", "MayPutAnyNumberOfCardsOfTypeOntoTheBattlefield", filt, tapped),
			tn("RevealTheTopNumberCardsOfLibraryAction", "PutTheCardFoundThisWayIntoHand"),
			tn("RevealTheTopNumberCardsOfLibraryAction", "PutTheRemainingCardsIntoGraveyard"),
			tn("RevealTheTopNumberCardsOfLibraryAction", "PutTheRemainingCardsOnTheBottomOfLibraryInARandomOrder")}),
			step(ir.Step{Verb: ir.VerbReveal, Obj: topOf(nil, two), Then: &ir.Program{Steps: []*ir.Step{
				{Verb: ir.VerbMove, Optional: true, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: allF}, To: ir.ZoneBattlefield, Enter: enTapped},
				{Verb: ir.VerbMove, Obj: found, To: ir.ZoneHand},
				{Verb: ir.VerbMove, To: ir.ZoneGraveyard},
				{Verb: ir.VerbMove, To: ir.ZoneLibrary, Word: "bottom-random"}}}})},
		{"RevealTopCard", A("RevealTopCardOfLibrary"), step(ir.Step{Verb: ir.VerbReveal, Obj: topOf(nil, one)})},
		{"SacrificeAPermanent", A("SacrificeAPermanent", filt), step(ir.Step{Verb: ir.VerbSacrifice, Obj: chooseF})},
		{"SacrificeEachPermanent", A("SacrificeEachPermanent", filt), step(ir.Step{Verb: ir.VerbSacrifice, Obj: allR})},
		{"SacrificeNumberPermanents", A("SacrificeNumberPermanents", tN2, filt),
			step(ir.Step{Verb: ir.VerbSacrifice, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: allF, N: two}})},
		{"SacrificePermanent", A("SacrificePermanent", ref), step(ir.Step{Verb: ir.VerbSacrifice, Obj: self})},
		{"Shuffle", A("Shuffle"), step(ir.Step{Verb: ir.VerbShuffle})},
		{"ShuffleHandIntoLibrary", A("ShuffleHandIntoLibrary"),
			mv(&ir.Ref{Kind: ir.RefAll, Filter: anyCardIn(ir.ZoneHand)}, ir.ZoneLibrary, "shuffle", nil)},
		{"Surveil", A("Surveil", tN2), step(ir.Step{Verb: ir.VerbSurveil, N: two})},
		{"TakeAnExtraTurn", A("TakeAnExtraTurn"), step(ir.Step{Verb: ir.VerbExtraTurn})},
		{"TapPermanent", A("TapPermanent", ref), step(ir.Step{Verb: ir.VerbTap, Obj: self})},
		{"TransformPermanent", A("TransformPermanent", ref), step(ir.Step{Verb: ir.VerbTransform, Obj: self})},
		{"Unless", A("Unless", tCond, []any{drawA()}),
			step(ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CNot, Args: []*ir.Cond{{Kind: ir.CKicked}}}, Then: &ir.Program{Steps: []*ir.Step{draw}}})},
		{"UntapPermanent", A("UntapPermanent", ref), step(ir.Step{Verb: ir.VerbUntap, Obj: self})},
		{"SearchLibrary", A("SearchLibrary", []any{
			tn("SearchLibraryAction", "FindAGenericCard"),
			tn("SearchLibraryAction", "FindNumberCardsOfType", tN2, filt),
			tn("SearchLibraryAction", "PutFoundCardsOntoBattlefield", tapped),
			tn("SearchLibraryAction", "RevealFoundCards"),
			tn("SearchLibraryAction", "SetAsideFoundCards"),
			tn("SearchLibraryAction", "PutSetAsideCardsOnTopOfLibrary")}),
			step(ir.Step{Verb: ir.VerbSearch, Then: &ir.Program{Steps: []*ir.Step{
				{Verb: ir.VerbChoose, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneLibrary), N: one}},
				{Verb: ir.VerbChoose, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: allF, N: two, UpTo: true}},
				{Verb: ir.VerbMove, Obj: found, To: ir.ZoneBattlefield, Enter: enTapped},
				{Verb: ir.VerbReveal, Obj: found},
				{Verb: ir.VerbDo},
				{Verb: ir.VerbMove, Obj: found, To: ir.ZoneLibrary, Word: "top"}}}})},
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

	prog := func(ss ...*ir.Step) *ir.Program { return &ir.Program{Steps: ss} }
	progCases := []struct {
		name string
		in   node
		want *ir.Program
	}{
		{"MayAction block", A("MayAction", A("PutCounters", []any{
			tn("PutCountersAction", "ACounterOfTypeOnPermanent", ctr, ref),
			tn("PutCountersAction", "ACounterOfTypeOnPermanent", ctr, ref)})),
			prog(step(ir.Step{Verb: ir.VerbDo, Optional: true, Then: prog(
				step(ir.Step{Verb: ir.VerbPutCounters, Obj: self, N: one, Word: "CHARGE"}),
				step(ir.Step{Verb: ir.VerbPutCounters, Obj: self, N: one, Word: "CHARGE"}))}))},
		{"PlayerAction", A("PlayerAction", pl, drawA()), prog(step(ir.Step{Verb: ir.VerbDraw, N: one, Who: you}))},
		{"PermanentDealsMultipleDamage", A("PermanentDealsMultipleDamage", ref, []any{
			tn("DamageToRecipients", "DamageToRecipients", tN2, ref), tn("DamageToRecipients", "DamageToRecipients", tN1, ref)}),
			prog(dmg, step(ir.Step{Verb: ir.VerbDealDamage, Source: self, N: one, Obj: self}))},
		{"PutCounters", A("PutCounters", []any{
			tn("PutCountersAction", "ACounterOfTypeOnPermanent", ctr, ref),
			tn("PutCountersAction", "ACounterOfTypeOnEachPermanent", ctr, filt),
			tn("PutCountersAction", "NumberCountersOfTypeOnPermanent", tN2, ctr, ref),
			tn("PutCountersAction", "DoubleCountersOfTypeOnPermanent", ctr, ref),
			tn("PutCountersAction", "DoubleAllCountersOnEachPermanent", filt)}),
			prog(
				step(ir.Step{Verb: ir.VerbPutCounters, Obj: self, N: one, Word: "CHARGE"}),
				step(ir.Step{Verb: ir.VerbPutCounters, Obj: allR, N: one, Word: "CHARGE"}),
				step(ir.Step{Verb: ir.VerbPutCounters, Obj: self, N: two, Word: "CHARGE"}),
				step(ir.Step{Verb: ir.VerbDoubleCounters, Obj: self, Word: "CHARGE"}),
				step(ir.Step{Verb: ir.VerbDoubleCounters, Obj: allR}))},
		{"RemoveCounters", A("RemoveCounters", []any{tn("RemoveCountersAction", "AllCountersOfTypeFromEachPermanent", ctr, filt)}),
			prog(step(ir.Step{Verb: ir.VerbRemoveCounters, Obj: allR, Word: "CHARGE"}))},
		{"Reflexive", A("Reflexive_CreateTokens_WhenYouDo", []any{tn("CreatableToken", "FoodToken")}, tapped, tn("Actions", "ActionList", []any{drawA()})),
			prog(step(ir.Step{Verb: ir.VerbCreateToken, N: one, Enter: enTapped,
				Token: &ir.Face{Name: "Food", Types: []string{"Artifact"}, Subtypes: []string{"Food"}}}),
				step(ir.Step{Verb: ir.VerbDelayedTrigger, Abilities: []*ir.Ability{{Kind: ir.AbTriggered,
					Trigger: &ir.Trigger{Event: ir.EvReflexive}, Body: prog(draw)}}}))},
	}
	for _, tc := range progCases {
		p, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tc.in}))
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if !ir.Equal(p, tc.want) {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, p, tc.want)
		}
	}

	// EnterFlag.
	enCases := []struct {
		name string
		in   node
		want *ir.Enter
	}{
		{"Normally", tn("EnterFlag", "EntersNormally"), &ir.Enter{}},
		{"Tapped", tn("EnterFlag", "EntersTapped"), &ir.Enter{Tapped: true}},
		{"UnderOwner", tn("EnterFlag", "EntersUnderOwnersControl"), &ir.Enter{UnderOwner: true}},
		{"UnderPlayer", tn("EnterFlag", "EntersUnderPlayersControl", pl), &ir.Enter{Controller: you}},
		{"WithACounter", tn("EnterFlag", "EntersWithACounter", ctr), &ir.Enter{Counter: "CHARGE", N: one}},
		{"WithNumberCounters", tn("EnterFlag", "EntersWithNumberCounters", tN2, ctr), &ir.Enter{Counter: "CHARGE", N: two}},
		{"WithLayerEffect", tn("EnterFlag", "EntersWithLayerEffect", []any{tEff}), &ir.Enter{Effects: []*ir.Effect{{Kind: ir.EffHexproof}}}},
	}
	for _, tc := range enCases {
		got, err := (&Ctx{}).Enter(tc.in)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if !ir.Equal(got, tc.want) {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
}

func drawA() node { return tn("Action", "DrawACard") }

func TestPlayerActionSetsWho(t *testing.T) {
	mzStubs(t)
	p, err := (&Ctx{}).Program(tn("Actions", "Targeted", []any{targetPlayer()},
		tn("Actions", "ActionList", []any{tn("Action", "PlayerAction", tn("Player", "Ref_TargetPlayer"),
			tn("Action", "MayAction", tn("Action", "DiscardACard")))})))
	if err != nil {
		t.Fatal(err)
	}
	want := &ir.Program{Targets: p.Targets, Steps: []*ir.Step{{Verb: ir.VerbDiscard, Optional: true,
		Who: &ir.Ref{Kind: ir.RefTarget, Slot: 0}, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: refConst1()}}}}
	if len(p.Targets) != 1 || !ir.Equal(p, want) {
		t.Errorf("got %s\nwant %s", p, want)
	}
	// An inner step that already has Who keeps it.
	q, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tn("Action", "PlayerAction", tn("Player", "Stub"),
		tn("Action", "EachPlayerAction", tn("Players", "Stub"), tn("Action", "DrawACard")))}))
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Steps) != 1 || q.Steps[0].Who == nil || q.Steps[0].Who.Kind != ir.RefYou ||
		q.Steps[0].Then.Steps[0].Who.Kind != ir.RefIterPlayer {
		t.Errorf("got %s", q)
	}
}

func TestReflexiveTargetFrame(t *testing.T) {
	mzStubs(t)
	reflexive := tn("Action", "Reflexive_CreateTokens_WhenYouDo", []any{tn("CreatableToken", "FoodToken")}, []any{},
		tn("Actions", "Targeted", []any{targetPerm()}, tn("Actions", "ActionList", []any{
			tn("Action", "DestroyPermanent", tn("Permanent", "Ref_TargetPermanent"))})))
	p, err := (&Ctx{}).Program(tn("Actions", "Targeted", []any{targetPlayer()}, tn("Actions", "ActionList", []any{
		reflexive, tn("Action", "DrawNumberCards", tN1),
		tn("Action", "PlayerAction", tn("Player", "Ref_TargetPlayer"), tn("Action", "DrawACard"))})))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Targets) != 1 || len(p.Steps) != 4 {
		t.Fatalf("got %s", p)
	}
	body := p.Steps[1].Abilities[0].Body
	if len(body.Targets) != 1 || body.Steps[0].Obj.Kind != ir.RefTarget || body.Steps[0].Obj.Slot != 0 {
		t.Errorf("reflexive body: %s", body)
	}
	if w := p.Steps[3].Who; w == nil || w.Kind != ir.RefTarget || w.Slot != 0 {
		t.Errorf("outer Ref_TargetPlayer after the reflexive: %s", p)
	}
}

func TestSearchReceipts(t *testing.T) {
	mzStubs(t)
	p, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tn("Action", "SearchLibrary", []any{
		tn("SearchLibraryAction", "FindACardOfType", tFilt),
		tn("SearchLibraryAction", "PutFoundCardsIntoHand"),
		tn("SearchLibraryAction", "Shuffle")})}))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Steps[0]
	if s.Verb != ir.VerbSearch || len(s.Then.Steps) != 3 || s.Then.Steps[0].Verb != ir.VerbChoose ||
		s.Then.Steps[1].Obj.Kind != ir.RefReceipt || s.Then.Steps[1].Obj.Receipt != ir.RcFound {
		t.Errorf("got %s", p)
	}
}

// A search of the library for stated cards may find fewer, or none (CR 701.23b); a generic search may not.
func TestSearchFindsAreUpTo(t *testing.T) {
	mzStubs(t)
	for _, tc := range []struct {
		name string
		find node
		want bool
	}{
		{"FindACardOfType", tn("SearchLibraryAction", "FindACardOfType", tFilt), true},
		{"FindNumberCardsOfType", tn("SearchLibraryAction", "FindNumberCardsOfType", tN2, tFilt), true},
		{"FindUptoNumberCardsOfType", tn("SearchLibraryAction", "FindUptoNumberCardsOfType", tN2, tFilt), true},
		{"FindAGenericCard", tn("SearchLibraryAction", "FindAGenericCard"), false},
	} {
		p, err := (&Ctx{}).Program(tn("Actions", "ActionList", []any{tn("Action", "SearchLibrary", []any{tc.find})}))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := p.Steps[0].Then.Steps[0].Obj.UpTo; got != tc.want {
			t.Errorf("%s: UpTo = %v, want %v", tc.name, got, tc.want)
		}
	}
}
