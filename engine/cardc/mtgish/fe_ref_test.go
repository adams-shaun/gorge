package mtgish

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func refNode(enum, variant string, args any) node {
	n := node{"_" + enum: variant}
	if args != nil {
		n["args"] = args
	}
	return n
}

func TestRefLowering(t *testing.T) {
	// Stubs for variants other tickets own: a filter and a value.
	filt := &ir.Filter{Subject: ir.SubjPermanent, Op: ir.FPred, Pred: ir.PAny}
	val := &ir.Value{Kind: ir.VConst, N: 7}
	withStubs(t, map[string]lowerFunc{
		"Permanents.Stub": func(*Ctx, node) (any, error) { return filt, nil },
		"Players.Stub":    func(*Ctx, node) (any, error) { return filt, nil },
		"GameNumber.Stub": func(*Ctx, node) (any, error) { return val, nil },
	})
	sPerms := refNode("Permanents", "Stub", nil)
	sPlayers := refNode("Players", "Stub", nil)
	sNum := refNode("GameNumber", "Stub", nil)

	self := &ir.Ref{Kind: ir.RefSelf}
	you := &ir.Ref{Kind: ir.RefYou}
	ev := &ir.Ref{Kind: ir.RefEventObject}
	tgt := func(s uint8) *ir.Ref { return &ir.Ref{Kind: ir.RefTarget, Slot: s} }
	one := &ir.Value{Kind: ir.VConst, N: 1}
	thisPerm := refNode("Permanent", "ThisPermanent", nil)
	thisSpell := refNode("Spell", "ThisSpell", nil)
	rc := func(k ir.ReceiptKind) *ir.Ref { return &ir.Ref{Kind: ir.RefReceipt, Receipt: k} }

	// Declarations: TargetPermanent x3, TargetSpell, TargetPlayer,
	// AnyTarget, TargetPlayerOrPermanent, TargetGraveyardCard, AnyTargets.
	decl := []string{
		"TargetPermanent", "TargetPermanent", "TargetPermanent", "TargetSpell",
		"TargetPlayer", "AnyTarget", "TargetPlayerOrPermanent", "TargetGraveyardCard", "AnyTargets",
	}
	slot := func(i int) uint8 { return uint8(i) }

	cases := []struct {
		name string
		in   node
		want *ir.Ref
	}{
		{"ThisPermanent", thisPerm, self},
		{"HostPermanent", refNode("Permanent", "HostPermanent", nil), &ir.Ref{Kind: ir.RefAttachedTo, Of: self}},
		{"Ref_TargetPermanent", refNode("Permanent", "Ref_TargetPermanent", nil), tgt(slot(2))},
		{"Ref_TargetPermanent1", refNode("Permanent", "Ref_TargetPermanent1", nil), tgt(0)},
		{"Ref_TargetPermanent2", refNode("Permanent", "Ref_TargetPermanent2", nil), tgt(1)},
		{"ThatEnteringPermanent", refNode("Permanent", "ThatEnteringPermanent", nil), ev},
		{"Trigger_ThatCreature", refNode("Permanent", "Trigger_ThatCreature", nil), ev},
		{"Trigger_ThatPermanent", refNode("Permanent", "Trigger_ThatPermanent", nil), ev},
		{"You", refNode("Player", "You", nil), you},
		{"ControllerOfPermanent", refNode("Player", "ControllerOfPermanent", thisPerm), &ir.Ref{Kind: ir.RefControllerOf, Of: self}},
		{"ControllerOfSpell", refNode("Player", "ControllerOfSpell", thisSpell), &ir.Ref{Kind: ir.RefControllerOf, Of: self}},
		{"ControllerOfTargetPermanent", refNode("Player", "ControllerOfTargetPermanent", nil), &ir.Ref{Kind: ir.RefControllerOf, Of: tgt(2)}},
		{"ControllerOfTargetSpell", refNode("Player", "ControllerOfTargetSpell", nil), &ir.Ref{Kind: ir.RefControllerOf, Of: tgt(3)}},
		{"OwnerOfPermanent", refNode("Player", "OwnerOfPermanent", thisPerm), &ir.Ref{Kind: ir.RefOwnerOf, Of: self}},
		{"PlayerOrControllerOfPermanent", refNode("Player", "PlayerOrControllerOfPermanent", refNode("PlayerOrPermanent", "Ref_AnyTarget", nil)), &ir.Ref{Kind: ir.RefPlayerOrControllerOf, Of: tgt(5)}},
		{"Ref_TargetPlayer", refNode("Player", "Ref_TargetPlayer", nil), tgt(4)},
		{"TheActivePlayer", refNode("Player", "TheActivePlayer", nil), &ir.Ref{Kind: ir.RefActivePlayer}},
		{"Trigger_DefendingPlayer", refNode("Player", "Trigger_DefendingPlayer", nil), &ir.Ref{Kind: ir.RefDefendingPlayer}},
		{"Trigger_ThatPlayer", refNode("Player", "Trigger_ThatPlayer", nil), &ir.Ref{Kind: ir.RefEventPlayer}},
		{"ActionPlayer", refNode("Player", "ActionPlayer", nil), &ir.Ref{Kind: ir.RefIterPlayer}},
		{"PoP.Ref_AnyTarget", refNode("PlayerOrPermanent", "Ref_AnyTarget", nil), tgt(5)},
		{"PoP.Ref_TargetPlayerOrPermanent", refNode("PlayerOrPermanent", "Ref_TargetPlayerOrPermanent", nil), tgt(6)},
		{"ThisSpell", thisSpell, self},
		{"Ref_TargetSpell", refNode("Spell", "Ref_TargetSpell", nil), tgt(3)},
		{"Trigger_ThatSpell", refNode("Spell", "Trigger_ThatSpell", nil), ev},
		{"ThisCard", refNode("Card", "ThisCard", nil), self},
		{"ThisGraveyardCard", refNode("CardInGraveyards", "ThisGraveyardCard", nil), self},
		{"Ref_TargetGraveyardCard", refNode("CardInGraveyards", "Ref_TargetGraveyardCard", nil), tgt(7)},
		{"Trigger_ThatGraveyardCard", refNode("CardInGraveyards", "Trigger_ThatGraveyardCard", nil), ev},
		{"TheCardExiledThisWay", refNode("CardInExile", "TheCardExiledThisWay", nil), rc(ir.RcExiled)},
		{"TheChosenExiledCard", refNode("CardInExile", "TheChosenExiledCard", nil), rc(ir.RcChosen)},
		{"TheExiledCardChosenThisWay", refNode("CardInExile", "TheExiledCardChosenThisWay", nil), rc(ir.RcChosen)},
		{"DR.EachPermanent", refNode("DamageRecipient", "EachPermanent", sPerms), &ir.Ref{Kind: ir.RefAll, Filter: filt}},
		{"DR.EachPlayer", refNode("DamageRecipient", "EachPlayer", sPlayers), &ir.Ref{Kind: ir.RefAll, Filter: filt}},
		{"DR.Permanent", refNode("DamageRecipient", "Permanent", thisPerm), self},
		{"DR.Player", refNode("DamageRecipient", "Player", refNode("Player", "You", nil)), you},
		{"DR.Ref_AnyTarget", refNode("DamageRecipient", "Ref_AnyTarget", nil), tgt(5)},
		{"DR.Ref_AnyTargets", refNode("DamageRecipient", "Ref_AnyTargets", nil), tgt(8)},
		{"DR.Ref_TargetPlayerOrPermanent", refNode("DamageRecipient", "Ref_TargetPlayerOrPermanent", nil), tgt(6)},
		{"SDR.Permanent", refNode("SingleDamageRecipient", "Permanent", thisPerm), self},
		{"Ex.Permanent", refNode("Exilable", "Permanent", thisPerm), self},
		{"Ex.Spell", refNode("Exilable", "Spell", thisSpell), self},
		{"Ex.CardInGraveyards", refNode("Exilable", "CardInGraveyards", refNode("CardInGraveyards", "ThisGraveyardCard", nil)), self},
		{"Ex.EachPermanent", refNode("Exilable", "EachPermanent", sPerms), &ir.Ref{Kind: ir.RefAll, Filter: filt}},
		{"Ex.EachCardInGraveyards", refNode("Exilable", "EachCardInGraveyards", sPerms), &ir.Ref{Kind: ir.RefAll, Filter: filt}},
		{"Ex.ACardFromHand", refNode("Exilable", "ACardFromHand", nil), &ir.Ref{Kind: ir.RefChoose, N: one, Filter: &ir.Filter{
			Subject: ir.SubjCard, Zones: ir.MakeZoneSet(ir.ZoneHand), Op: ir.FPred, Pred: ir.PAny}}},
		{"Ex.ACardInGraveyards", refNode("Exilable", "ACardInGraveyards", sPerms), &ir.Ref{Kind: ir.RefChoose, Filter: filt, N: one}},
		{"Ex.TheTopCardOfLibrary", refNode("Exilable", "TheTopCardOfLibrary", nil), &ir.Ref{Kind: ir.RefTopOfLibrary, N: one}},
		{"Ex.TheTopNumberCardsOfLibrary", refNode("Exilable", "TheTopNumberCardsOfLibrary", sNum), &ir.Ref{Kind: ir.RefTopOfLibrary, N: val}},
		{"Ex.TheTopCardOfEachPlayersLibraries", refNode("Exilable", "TheTopCardOfEachPlayersLibraries", sPlayers),
			&ir.Ref{Kind: ir.RefTopOfLibrary, Of: &ir.Ref{Kind: ir.RefAll, Filter: filt}, N: one}},
		{"Ex.AllCardsFromPlayersLibrary", refNode("Exilable", "AllCardsFromPlayersLibrary", refNode("Player", "You", nil)),
			&ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{Subject: ir.SubjCard, Zones: ir.MakeZoneSet(ir.ZoneLibrary),
				Op: ir.FPred, Pred: ir.POwnedBy, Ref: you}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Ctx{}
			for _, s := range decl {
				c.DeclareTarget(s)
			}
			got, err := c.Ref(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if !ir.Equal(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestRefUnboundTarget(t *testing.T) {
	cases := map[string]node{
		"Permanent.Ref_TargetPermanent":            refNode("Permanent", "Ref_TargetPermanent", nil),
		"Player.ControllerOfTargetPermanent":       refNode("Player", "ControllerOfTargetPermanent", nil),
		"DamageRecipient.Ref_AnyTargets":           refNode("DamageRecipient", "Ref_AnyTargets", nil),
		"Spell.Ref_TargetSpell":                    refNode("Spell", "Ref_TargetSpell", nil),
		"PlayerOrPermanent.Ref_AnyTarget":          refNode("PlayerOrPermanent", "Ref_AnyTarget", nil),
		"CardInGraveyards.Ref_TargetGraveyardCard": refNode("CardInGraveyards", "Ref_TargetGraveyardCard", nil),
	}
	for key, in := range cases {
		c := &Ctx{}
		_, err := c.Ref(in)
		var re *RefusalError
		if !errors.As(err, &re) {
			t.Fatalf("%s: err = %v, want RefusalError", key, err)
		}
		if re.Key != key+":unbound" {
			t.Errorf("%s: key %q", key, re.Key)
		}
		if re.Path == "" {
			t.Errorf("%s: empty path", key)
		}
	}
	// A declared target of another stem does not bind.
	c := &Ctx{}
	c.DeclareTarget("TargetPlayer")
	if _, err := c.Ref(refNode("Permanent", "Ref_TargetPermanent", nil)); err == nil {
		t.Error("bound to the wrong stem")
	}
}

func TestRefSliceVariantsRegistered(t *testing.T) {
	checkFamilyCoverage(t, []string{
		"Permanent", "Player", "PlayerOrPermanent", "Spell", "Card", "CardInGraveyards",
		"CardInExile", "DamageRecipient", "SingleDamageRecipient", "Exilable",
	}, map[string]string{"Permanent.ThisGuest": "Guest mechanic"})
}
