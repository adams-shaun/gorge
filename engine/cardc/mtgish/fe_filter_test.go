package mtgish

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// tn builds a tagged node: tn("Permanents", "IsToken") or with args.
func tn(enum, variant string, args ...any) node {
	m := node{"_" + enum: variant}
	switch len(args) {
	case 0:
	case 1:
		m["args"] = args[0]
	default:
		m["args"] = args
	}
	return m
}

// filtStubs stands in for the other tickets' lowerings: a comparison (>= 3),
// a reference (self), a counter word and a game number (N = args).
func filtStubs(t *testing.T) {
	t.Helper()
	withStubs(t, map[string]lowerFunc{
		"TestOnly.Cmp": func(*Ctx, node) (any, error) {
			return &ir.Compare{Op: ir.CmpGE, V: &ir.Value{Kind: ir.VConst, N: 3}}, nil
		},
		"TestOnly.Ref":     func(*Ctx, node) (any, error) { return &ir.Ref{Kind: ir.RefSelf}, nil },
		"TestOnly.Counter": func(*Ctx, node) (any, error) { return "P1P1", nil },
		"TestOnly.N": func(c *Ctx, n node) (any, error) {
			x, _ := intOf(rawArgs(n))
			return &ir.Value{Kind: ir.VConst, N: int32(x)}, nil
		},
		"TestOnly.X": func(*Ctx, node) (any, error) { return &ir.Value{Kind: ir.VX}, nil },
	})
}

var (
	tCmp     = tn("TestOnly", "Cmp")
	tRef     = tn("TestOnly", "Ref")
	tCounter = tn("TestOnly", "Counter")
)

func filtWant(e *filtEnum, p ir.Pred, mod func(*ir.Filter)) *ir.Filter {
	f := e.pred(p)
	if mod != nil {
		mod(f)
	}
	return f
}

func TestFilterLowering(t *testing.T) {
	filtStubs(t)
	one := &ir.Value{Kind: ir.VConst, N: 1}
	three := &ir.Value{Kind: ir.VConst, N: 3}
	selfRef := &ir.Ref{Kind: ir.RefSelf}
	anyPlayers := &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}
	cases := []struct {
		variant string
		args    []any
		want    func(e *filtEnum) *ir.Filter
	}{
		{"IsCardtype", []any{tn("CardType", "Creature")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PCardType, func(f *ir.Filter) { f.Word = "Creature" })
		}},
		{"IsNonCardtype", []any{tn("CardType", "Land")}, func(e *filtEnum) *ir.Filter {
			return e.not(filtWant(e, ir.PCardType, func(f *ir.Filter) { f.Word = "Land" }))
		}},
		{"IsCreatureType", []any{tn("CreatureType", "Elf")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PSubtype, func(f *ir.Filter) { f.Word = "Elf" })
		}},
		{"IsLandType", []any{tn("LandType", "Forest")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PSubtype, func(f *ir.Filter) { f.Word = "Forest" })
		}},
		{"IsArtifactType", []any{tn("ArtifactType", "Equipment")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PSubtype, func(f *ir.Filter) { f.Word = "Equipment" })
		}},
		{"IsNonCreatureType", []any{tn("CreatureType", "Elf")}, func(e *filtEnum) *ir.Filter {
			return e.not(filtWant(e, ir.PSubtype, func(f *ir.Filter) { f.Word = "Elf" }))
		}},
		{"IsNonEnchantmentType", []any{tn("EnchantmentType", "Aura")}, func(e *filtEnum) *ir.Filter {
			return e.not(filtWant(e, ir.PSubtype, func(f *ir.Filter) { f.Word = "Aura" }))
		}},
		{"IsSupertype", []any{tn("SuperType", "Legendary")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PSupertype, func(f *ir.Filter) { f.Word = "Legendary" })
		}},
		{"IsNonSupertype", []any{tn("SuperType", "Basic")}, func(e *filtEnum) *ir.Filter {
			return e.not(filtWant(e, ir.PSupertype, func(f *ir.Filter) { f.Word = "Basic" }))
		}},
		{"IsColor", []any{tn("Color", "Red")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PColor, func(f *ir.Filter) { f.Colors = ir.ColorR })
		}},
		{"IsNonColor", []any{tn("Color", "Blue")}, func(e *filtEnum) *ir.Filter {
			return e.not(filtWant(e, ir.PColor, func(f *ir.Filter) { f.Colors = ir.ColorU }))
		}},
		{"IsColorless", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PColorless) }},
		{"IsPermanent", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PPermanentCard) }},
		{"ManaValueIs", []any{tCmp}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PManaValue, func(f *ir.Filter) { f.Cmp, f.Value = ir.CmpGE, three })
		}},
		{"PowerIs", []any{tCmp}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PPower, func(f *ir.Filter) { f.Cmp, f.Value = ir.CmpGE, three })
		}},
		{"ToughnessIs", []any{tCmp}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PToughness, func(f *ir.Filter) { f.Cmp, f.Value = ir.CmpGE, three })
		}},
		{"ControlledByAPlayer", []any{tn("Players", "AnyPlayer")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PControlledBy, func(f *ir.Filter) { f.Ref = &ir.Ref{Kind: ir.RefAll, Filter: anyPlayers} })
		}},
		{"OwnedByAPlayer", []any{tn("Players", "AnyPlayer")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.POwnedBy, func(f *ir.Filter) { f.Ref = &ir.Ref{Kind: ir.RefAll, Filter: anyPlayers} })
		}},
		{"InAPlayersGraveyard", []any{tn("Players", "AnyPlayer")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.POwnedBy, func(f *ir.Filter) { f.Ref = &ir.Ref{Kind: ir.RefAll, Filter: anyPlayers} })
		}},
		{"HasAbility", []any{tn("CheckHasable", "FirstStrike")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PHasKeyword, func(f *ir.Filter) { f.Word = "First Strike" })
		}},
		{"DoesntHaveAbility", []any{tn("CheckHasable", "Flying")}, func(e *filtEnum) *ir.Filter {
			return e.not(filtWant(e, ir.PHasKeyword, func(f *ir.Filter) { f.Word = "Flying" }))
		}},
		{"IsToken", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PToken) }},
		{"IsNonToken", nil, func(e *filtEnum) *ir.Filter { return e.not(e.pred(ir.PToken)) }},
		{"IsTapped", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PTapped) }},
		{"IsUntapped", nil, func(e *filtEnum) *ir.Filter { return e.not(e.pred(ir.PTapped)) }},
		{"IsAttacking", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PAttacking) }},
		{"IsBlocking", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PBlocking) }},
		{"Other", []any{tRef}, func(e *filtEnum) *ir.Filter {
			return e.not(filtWant(e, ir.PIs, func(f *ir.Filter) { f.Ref = selfRef }))
		}},
		{"SinglePermanent", []any{tRef}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PIs, func(f *ir.Filter) { f.Ref = selfRef })
		}},
		{"SingleSpell", []any{tRef}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PIs, func(f *ir.Filter) { f.Ref = selfRef })
		}},
		{"AnyPermanent", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PAny) }},
		{"AnySpell", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PAny) }},
		{"AnyCard", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PAny) }},
		{"AnyCardInAnyGraveyard", nil, func(e *filtEnum) *ir.Filter { return e.pred(ir.PAny) }},
		{"IsNamed", []any{tn("NameFilter", "NamedCard", "Zz Synthetic")}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PName, func(f *ir.Filter) { f.Word = "Zz Synthetic" })
		}},
		{"HasACounterOfType", []any{tCounter}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PCounters, func(f *ir.Filter) { f.Word, f.Cmp, f.Value = "P1P1", ir.CmpGE, one })
		}},
		{"HasNoCountersOfType", []any{tCounter}, func(e *filtEnum) *ir.Filter {
			return e.not(filtWant(e, ir.PCounters, func(f *ir.Filter) { f.Word, f.Cmp, f.Value = "P1P1", ir.CmpGE, one }))
		}},
		{"HasNumberCountersOfType", []any{tCmp, tCounter}, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PCounters, func(f *ir.Filter) { f.Word, f.Cmp, f.Value = "P1P1", ir.CmpGE, three })
		}},
		{"TheCardsExiledThisWay", nil, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PIs, func(f *ir.Filter) { f.Ref = &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcExiled} })
		}},
		{"TheTokensCreatedThisWay", nil, func(e *filtEnum) *ir.Filter {
			return filtWant(e, ir.PIs, func(f *ir.Filter) { f.Ref = &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcCreated} })
		}},
	}
	checked := map[string]bool{}
	for i := range filtEnums {
		e := &filtEnums[i]
		if e.player() {
			continue // TestPlayerFilterLowering
		}
		for _, tc := range cases {
			if !e.has(tc.variant) {
				continue
			}
			checked[tc.variant] = true
			c := &Ctx{}
			got, err := c.Filter(tn(e.name, tc.variant, tc.args...))
			if err != nil {
				t.Errorf("%s.%s: %v", e.name, tc.variant, err)
				continue
			}
			if want := tc.want(e); !reflect.DeepEqual(got, want) {
				t.Errorf("%s.%s:\n got %+v\nwant %+v", e.name, tc.variant, got, want)
			}
		}
	}
	for _, tc := range cases {
		if !checked[tc.variant] {
			t.Errorf("%s: no non-player enum has it", tc.variant)
		}
	}

	// Subject and Zones come from the enum, at every node of the tree.
	for _, tc := range []struct {
		enum  string
		subj  ir.Subject
		zones ir.ZoneSet
	}{
		{"Permanents", ir.SubjPermanent, zoneBit(ir.ZoneBattlefield)},
		{"Spells", ir.SubjSpell, zoneBit(ir.ZoneStack)},
		{"Cards", ir.SubjCard, 0},
		{"CardsInGraveyards", ir.SubjCard, zoneBit(ir.ZoneGraveyard)},
		{"CardsInExile", ir.SubjCard, zoneBit(ir.ZoneExile)},
		{"CardsInLibrary", ir.SubjCard, zoneBit(ir.ZoneLibrary)},
		{"CardsInHand", ir.SubjCard, zoneBit(ir.ZoneHand)},
	} {
		c := &Ctx{}
		tree := tn(tc.enum, "And", []any{
			tn(tc.enum, "Or", []any{tn(tc.enum, "IsPermanent"), tn(tc.enum, "IsPermanent")}),
			tn(tc.enum, "IsNonCardtype", tn("CardType", "Land")),
		})
		f, err := c.Filter(tree)
		if err != nil {
			t.Errorf("%s: %v", tc.enum, err)
			continue
		}
		var visit func(f *ir.Filter)
		nodes := 0
		visit = func(f *ir.Filter) {
			nodes++
			if f.Subject != tc.subj || f.Zones != tc.zones {
				t.Errorf("%s: node %v carries %v/%v", tc.enum, f.Op, f.Subject, f.Zones)
			}
			for _, a := range f.Args {
				visit(a)
			}
		}
		visit(f)
		if f.Op != ir.FAll || f.Args[0].Op != ir.FAny || nodes != 6 {
			t.Errorf("%s: And/Or shape wrong: %d nodes, %v/%v", tc.enum, nodes, f.Op, f.Args[0].Op)
		}
	}

	// Target references resolve through the declared slot.
	c := &Ctx{}
	c.DeclareTarget("TargetPermanents")
	f, err := c.Filter(tn("Permanents", "Ref_TargetPermanents"))
	want := &ir.Filter{Subject: ir.SubjPermanent, Zones: zoneBit(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PIs,
		Ref: &ir.Ref{Kind: ir.RefTarget, Slot: 0}}
	if err != nil || !reflect.DeepEqual(f, want) {
		t.Errorf("Ref_TargetPermanents: %+v, %v", f, err)
	}
	c = &Ctx{}
	if _, err := c.Filter(tn("Permanents", "Ref_TargetPermanents")); err == nil {
		t.Error("an undeclared target reference lowered")
	}
	c.DeclareTarget("TargetGraveyardCards")
	f, err = c.Filter(tn("CardsInGraveyards", "Ref_TargetGraveyardCards"))
	if err != nil || f.Ref.Kind != ir.RefTarget || f.Subject != ir.SubjCard || f.Zones != zoneBit(ir.ZoneGraveyard) {
		t.Errorf("Ref_TargetGraveyardCards: %+v, %v", f, err)
	}

	// A non-string where a word is wanted is an error.
	if _, err := (&Ctx{}).Filter(tn("Permanents", "IsNamed", tRef)); err == nil {
		t.Error("IsNamed of a non-string lowered")
	}
}

func TestFilterSmallEnums(t *testing.T) {
	c := &Ctx{}
	for _, tc := range []struct {
		node node
		want any
	}{
		{tn("Color", "White"), ir.ColorW},
		{tn("Color", "Blue"), ir.ColorU},
		{tn("Color", "Black"), ir.ColorB},
		{tn("Color", "Red"), ir.ColorR},
		{tn("Color", "Green"), ir.ColorG},
		{tn("NameFilter", "NamedCard", "Zz Synthetic"), "Zz Synthetic"},
		{tn("CheckHasable", "DoubleStrike"), "Double Strike"},
		{tn("CheckHasable", "FirstStrike"), "First Strike"},
		{tn("CheckHasable", "Flying"), "Flying"},
	} {
		got, err := c.lower(tc.node)
		if err != nil || got != tc.want {
			t.Errorf("%s: %#v, %v; want %#v", qualified(tc.node), got, err, tc.want)
		}
	}
	for _, v := range []string{"Flying", "Deathtouch", "Defender", "Flash", "Haste", "Indestructible", "Lifelink",
		"Menace", "Reach", "Trample", "Vigilance", "DoubleStrike", "FirstStrike"} {
		if !Registered("CheckHasable." + v) {
			t.Errorf("CheckHasable.%s unregistered", v)
		}
	}
	if Registered("CheckHasable.AnyKicker") || Registered("Color.TheChosenColor") || Registered("NameFilter.TheChosenName") {
		t.Error("a deferred variant is registered")
	}
}

func TestPlayerFilterLowering(t *testing.T) {
	filtStubs(t)
	var pe *filtEnum
	for i := range filtEnums {
		if filtEnums[i].name == "Players" {
			pe = &filtEnums[i]
		}
	}
	three := &ir.Value{Kind: ir.VConst, N: 3}
	perms := &ir.Filter{Subject: ir.SubjPermanent, Zones: zoneBit(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PAny}
	selfRef := &ir.Ref{Kind: ir.RefSelf}
	for _, tc := range []struct {
		variant string
		args    []any
		want    *ir.Filter
	}{
		{"AnyPlayer", nil, pe.pred(ir.PAny)},
		{"Opponent", nil, filtWant(pe, ir.POpponentOf, func(f *ir.Filter) { f.Ref = &ir.Ref{Kind: ir.RefYou} })},
		{"Other", []any{tRef}, pe.not(filtWant(pe, ir.PPlayerIs, func(f *ir.Filter) { f.Ref = selfRef }))},
		{"SinglePlayer", []any{tRef}, filtWant(pe, ir.PPlayerIs, func(f *ir.Filter) { f.Ref = selfRef })},
		{"AttackedThisTurn", nil, pe.pred(ir.PAttackedThisTurn)},
		{"ControlsA", []any{tn("Permanents", "AnyPermanent")}, filtWant(pe, ir.PControls, func(f *ir.Filter) { f.Args = []*ir.Filter{perms} })},
		{"ControlsMorePermanentThanPlayer", []any{tRef, tn("Permanents", "AnyPermanent")},
			filtWant(pe, ir.PControlsMoreThan, func(f *ir.Filter) { f.Ref, f.Args = selfRef, []*ir.Filter{perms} })},
		{"LifeTotalIs", []any{tCmp}, filtWant(pe, ir.PLifeTotal, func(f *ir.Filter) { f.Cmp, f.Value = ir.CmpGE, three })},
		{"LostLifeThisTurn", nil, pe.pred(ir.PLostLifeThisTurn)},
		{"NumCardsInGraveyardIs", []any{tCmp}, filtWant(pe, ir.PCardsInGraveyard, func(f *ir.Filter) { f.Cmp, f.Value = ir.CmpGE, three })},
		{"NumCardsInHandIs", []any{tCmp}, filtWant(pe, ir.PCardsInHand, func(f *ir.Filter) { f.Cmp, f.Value = ir.CmpGE, three })},
		{"HasACounterOfType", []any{tCounter}, filtWant(pe, ir.PCounters, func(f *ir.Filter) {
			f.Word, f.Cmp, f.Value = "P1P1", ir.CmpGE, &ir.Value{Kind: ir.VConst, N: 1}
		})},
		{"And", []any{[]any{tn("Players", "AnyPlayer"), tn("Players", "LostLifeThisTurn")}}, func() *ir.Filter {
			f := pe.node(ir.FAll)
			f.Args = []*ir.Filter{pe.pred(ir.PAny), pe.pred(ir.PLostLifeThisTurn)}
			return f
		}()},
		{"Or", []any{[]any{tn("Players", "AnyPlayer"), tn("Players", "Opponent")}}, func() *ir.Filter {
			f := pe.node(ir.FAny)
			f.Args = []*ir.Filter{pe.pred(ir.PAny), filtWant(pe, ir.POpponentOf, func(f *ir.Filter) { f.Ref = &ir.Ref{Kind: ir.RefYou} })}
			return f
		}()},
	} {
		got, err := (&Ctx{}).Filter(tn("Players", tc.variant, tc.args...))
		if err != nil {
			t.Errorf("Players.%s: %v", tc.variant, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Players.%s:\n got %+v\nwant %+v", tc.variant, got, tc.want)
		}
	}
}

func TestFilterSliceVariantsRegistered(t *testing.T) {
	deferred := map[string]string{
		"Permanents.AttachedToPermanent":                            "needs an attachment predicate",
		"Permanents.CastByPlayer":                                   "needs a cast-by predicate",
		"Permanents.ExceptFor":                                      "a set difference",
		"Permanents.HasTheMostVotesOrTiedForTheMostVotes":           "a vote result",
		"Permanents.IsCreatureTypeVariable":                         "a type chosen at resolution",
		"Permanents.OnTheBattlefield":                               "no predicate",
		"Permanents.SharesANameWithPermanent":                       "no predicate",
		"Permanents.SurgeCostWasPaid":                               "a cast-time fact",
		"Permanents.WasKicked":                                      "a cast-time fact",
		"Permanents.ThePermanentsNotChosenThisWay":                  "a receipt complement",
		"Spells.IsCreatureTypeVariable":                             "a type chosen at resolution",
		"CardsInGraveyards.WasPutIntoGraveyardFromAnywhereThisTurn": "an event history predicate",
		"CardsInExile.IsFaceUp":                                     "no predicate",
		"CardsInExile.WasExiledWithPermanent":                       "a link to a permanent",
		"CardsInLibrary.SharesANameWithTheCardRevealedThisWay":      "a receipt name",
		"Color.TheChosenColor":                                      "a colour chosen at resolution",
		"NameFilter.TheChosenName":                                  "a name chosen at resolution",
		"Target.NumberTargetGroupPermanents":                        "a group target",
		"Target.UptoNumberTargetGroupGraveyardCards":                "a group target",
		"Target.UptoNumberAnyTargetsExcept":                         "an excluded target",
	}
	checkFamilyCoverage(t, []string{"Permanents", "Players", "Spells", "Cards", "CardsInGraveyards", "CardsInExile",
		"CardsInLibrary", "CardsInHand", "Target", "CheckHasable", "Color", "NameFilter"}, deferred)
}
