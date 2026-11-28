package mtgish

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func condNode(variant string, args ...any) node {
	n := node{"_Condition": variant}
	switch len(args) {
	case 0:
	case 1:
		n["args"] = args[0]
	default:
		n["args"] = args
	}
	return n
}

func condStub(enum, name string) node { return node{"_" + enum: name} }

func TestCondLowering(t *testing.T) {
	refA := &ir.Ref{Kind: ir.RefEventObject}
	cmpV := &ir.Value{Kind: ir.VX}
	withStubs(t, map[string]lowerFunc{
		"Player.StubP":      func(*Ctx, node) (any, error) { return &ir.Ref{Kind: ir.RefYou}, nil },
		"Players.StubPs1":   func(*Ctx, node) (any, error) { return &ir.Filter{Word: "ps1"}, nil },
		"Players.StubPs2":   func(*Ctx, node) (any, error) { return &ir.Filter{Word: "ps2"}, nil },
		"Permanent.StubP":   func(*Ctx, node) (any, error) { return refA, nil },
		"Permanents.StubF1": func(*Ctx, node) (any, error) { return &ir.Filter{Word: "f1"}, nil },
		"Permanents.StubF2": func(*Ctx, node) (any, error) { return &ir.Filter{Word: "f2"}, nil },
		"Cards.StubC":       func(*Ctx, node) (any, error) { return &ir.Filter{Word: "cs"}, nil },
		"Comparison.StubC": func(*Ctx, node) (any, error) {
			return &ir.Compare{Op: ir.CmpOp(1), V: cmpV}, nil
		},
	})
	p, ps1, ps2 := condStub("Player", "StubP"), condStub("Players", "StubPs1"), condStub("Players", "StubPs2")
	pm, f1, f2 := condStub("Permanent", "StubP"), condStub("Permanents", "StubF1"), condStub("Permanents", "StubF2")
	cs, cmp := condStub("Cards", "StubC"), condStub("Comparison", "StubC")
	fw := func(w string) *ir.Filter { return &ir.Filter{Word: w} }
	all := func(a, b string) *ir.Filter { return &ir.Filter{Op: ir.FAll, Args: []*ir.Filter{fw(a), fw(b)}} }
	you := &ir.Ref{Kind: ir.RefYou}
	ev := &ir.Ref{Kind: ir.RefEventObject}
	cases := []struct {
		in   node
		want *ir.Cond
	}{
		{condNode("PlayerPassesFilter", p, ps1), &ir.Cond{Kind: ir.CRefMatches, Ref: you, Filter: fw("ps1")}},
		{condNode("PermanentPassesFilter", pm, f1), &ir.Cond{Kind: ir.CRefMatches, Ref: refA, Filter: fw("f1")}},
		{condNode("EnteringPermanentPassesFilter", f1), &ir.Cond{Kind: ir.CRefMatches, Ref: ev, Filter: fw("f1")}},
		{condNode("DeadPermanentPassesFilter", f1), &ir.Cond{Kind: ir.CRefMatches, Ref: ev, Filter: fw("f1")}},
		{condNode("APlayerPassesFilter", ps1, ps2), &ir.Cond{Kind: ir.CExistsPlayer, Filter: all("ps1", "ps2")}},
		{condNode("NumPermanentsPassFilter", cmp, f1, f2), &ir.Cond{Kind: ir.CCountCompare, Filter: all("f1", "f2"), Cmp: ir.CmpOp(1), R: cmpV}},
		{condNode("CostWasPaid"), &ir.Cond{Kind: ir.CCostPaid}},
		{condNode("IsPlayersTurn", p), &ir.Cond{Kind: ir.CIsTurnOf, Ref: you}},
		{condNode("IsAPlayersTurn", ps1), &ir.Cond{Kind: ir.CIsTurnOf, Filter: fw("ps1")}},
		{condNode("IsPlayersNthTurn", p, cmp), &ir.Cond{Kind: ir.CNthTurn, Ref: you, Cmp: ir.CmpOp(1), R: cmpV}},
		{condNode("ACardWasExiledThisWay", cs), &ir.Cond{Kind: ir.CReceiptNonEmpty, Receipt: ir.RcExiled, Filter: fw("cs")}},
		{condNode("ACardWasRevealedThisWay", cs), &ir.Cond{Kind: ir.CReceiptNonEmpty, Receipt: ir.RcRevealed, Filter: fw("cs")}},
		{condNode("ACreatureOrPlaneswalkerDiedThisTurn", f1), &ir.Cond{Kind: ir.CDiedThisTurn, Filter: fw("f1")}},
		{condNode("APermanentLeftTheBattlefieldThisTurn", f1), &ir.Cond{Kind: ir.CLeftBattlefieldThisTurn, Filter: fw("f1")}},
		{condNode("And", []any{condNode("CostWasPaid"), condNode("IsPlayersTurn", p)}),
			&ir.Cond{Kind: ir.CAll, Args: []*ir.Cond{{Kind: ir.CCostPaid}, {Kind: ir.CIsTurnOf, Ref: you}}}},
		{condNode("Or", []any{condNode("CostWasPaid"), condNode("IsPlayersTurn", p)}),
			&ir.Cond{Kind: ir.CAny, Args: []*ir.Cond{{Kind: ir.CCostPaid}, {Kind: ir.CIsTurnOf, Ref: you}}}},
	}
	for _, tc := range cases {
		c := &Ctx{}
		got, err := c.Cond(tc.in)
		if err != nil {
			t.Errorf("%s: %v", variantOf(tc.in), err)
			continue
		}
		if !ir.Equal(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", variantOf(tc.in), got, tc.want)
		}
	}
}

func TestCondSliceVariantsRegistered(t *testing.T) {
	checkFamilyCoverage(t, []string{"Condition"}, map[string]string{
		"Condition.NumGroupCardsWereMilledThisWay": "needs GroupFilter",
	})
}
