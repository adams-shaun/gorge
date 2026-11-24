package mtgish

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func valNode(enum, variant string, args any) node {
	n := node{"_" + enum: variant}
	if args != nil {
		n["args"] = args
	}
	return n
}

// valStubs stubs the nested variants other tickets own: each lowers to a
// distinct marker keyed off its variant name.
func valStubs(t *testing.T) {
	t.Helper()
	filt := func(c *Ctx, n node) (any, error) { return &ir.Filter{Op: ir.FAll, Word: variantOf(n)}, nil }
	ref := func(kind ir.RefKind) lowerFunc {
		return func(c *Ctx, n node) (any, error) { return &ir.Ref{Kind: kind, Src: variantOf(n)}, nil }
	}
	withStubs(t, map[string]lowerFunc{
		"Permanents.Any":          filt,
		"Spells.Any":              filt,
		"CardsInGraveyards.Any":   filt,
		"Permanent.Any":           ref(ir.RefYou),
		"Player.You":              ref(ir.RefYou),
		"Spell.Trigger_ThatSpell": ref(ir.RefEventObject),
		"Spell.Ref_TargetSpell":   ref(ir.RefTarget),
		"Spell.ThisSpell":         ref(ir.RefSelf),
		"Color.Red":               func(c *Ctx, n node) (any, error) { return ir.ColorR, nil },
	})
}

func valLower(t *testing.T, x node) *ir.Value {
	t.Helper()
	v, err := (&Ctx{}).Value(x)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestValueLowering(t *testing.T) {
	valStubs(t)
	perms := valNode("Permanents", "Any", nil)
	player := valNode("Player", "You", nil)
	perm := valNode("Permanent", "Any", nil)
	gn := func(v string, a any) node { return valNode("GameNumber", v, a) }
	one := gn("Integer", 1.0)
	three := gn("Integer", 3.0)
	c1 := &ir.Value{Kind: ir.VConst, N: 1}
	c3 := &ir.Value{Kind: ir.VConst, N: 3}
	pf := &ir.Filter{Op: ir.FAll, Word: "Any"}
	pr := &ir.Ref{Kind: ir.RefYou, Src: "You"}
	prm := &ir.Ref{Kind: ir.RefYou, Src: "Any"}

	cases := []struct {
		name string
		in   node
		want *ir.Value
	}{
		{"Integer", three, c3},
		{"ValueX", gn("ValueX", nil), &ir.Value{Kind: ir.VX}},
		{"Plus", gn("Plus", []any{one, three}), &ir.Value{Kind: ir.VAdd, Args: []*ir.Value{c1, c3}}},
		{"Minus", gn("Minus", []any{three, one}), &ir.Value{Kind: ir.VSub, Args: []*ir.Value{c3, c1}}},
		{"Twice", gn("Twice", three), &ir.Value{Kind: ir.VMul, Args: []*ir.Value{{Kind: ir.VConst, N: 2}, c3}}},
		{"Permanents", gn("TheNumberOfPermanentsOnTheBattlefield", perms), &ir.Value{Kind: ir.VCount, Filter: pf}},
		{"Graveyard", gn("TheNumberOfGraveyardCards", valNode("CardsInGraveyards", "Any", nil)), &ir.Value{Kind: ir.VCount, Filter: pf}},
		{"Power", gn("PowerOfPermanent", perm), &ir.Value{Kind: ir.VPower, Ref: prm}},
		{"DeadPower", gn("PowerOfDeadPermanent", nil), &ir.Value{Kind: ir.VPower, Ref: &ir.Ref{Kind: ir.RefEventObject}}},
		{"Toughness", gn("ToughnessOfPermanent", perm), &ir.Value{Kind: ir.VToughness, Ref: prm}},
		{"ManaValue", gn("ManaValueOfPermanent", perm), &ir.Value{Kind: ir.VManaValue, Ref: prm}},
		{"RevealedMV", gn("ManaValueOfTheCardRevealedThisWay", nil), &ir.Value{Kind: ir.VManaValue, Ref: &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcRevealed}}},
		{"Life", gn("LifeTotalOfPlayer", player), &ir.Value{Kind: ir.VLife, Ref: pr}},
		{"StartingLife", gn("StartingLifeTotalOfPlayer", player), &ir.Value{Kind: ir.VStartingLife, Ref: pr}},
		{"LifeGained", gn("LifeGainedByPlayerThisTurn", player), &ir.Value{Kind: ir.VLifeGainedThisTurn, Ref: pr}},
		{"Devotion", gn("PlayerDevotionTo", []any{player, valNode("Color", "Red", nil)}), &ir.Value{Kind: ir.VDevotion, Ref: pr, Colors: ir.ColorR}},
		{"CountersAmong", gn("TheTotalNumberOfCountersOfTypeAmongPermanents", []any{valNode("CounterType", "ChargeCounter", nil), perms}), &ir.Value{Kind: ir.VCountersAmong, Word: "CHARGE", Filter: pf}},
		{"TotalPower", gn("TotalPowerOfPermanents", perms), &ir.Value{Kind: ir.VTotalPower, Filter: pf}},
		{"DistinctMV", gn("NumDifferentManaValuesAmongPermanents", perms), &ir.Value{Kind: ir.VDistinctManaValues, Filter: pf}},
		{"ChosenNumber", gn("TheChosenNumber", nil), &ir.Value{Kind: ir.VChosenNumber}},
		{"AmountDamage", gn("Trigger_AmountOfDamageDealt", nil), &ir.Value{Kind: ir.VEventAmount}},
		{"AmountLife", gn("Trigger_AmountOfLifeLost", nil), &ir.Value{Kind: ir.VEventAmount}},
		{"ThatMuch", gn("Trigger_ThatMuch", nil), &ir.Value{Kind: ir.VEventAmount}},
		{"WouldDeal", gn("WouldDealDamage_ThatMuchDamage", nil), &ir.Value{Kind: ir.VEventAmount}},
		{"LifeLost", gn("LifeLostThisWay", nil), &ir.Value{Kind: ir.VReceiptAmount, Word: "life-lost"}},
		{"ThatMany", gn("ThatManyCounters", nil), &ir.Value{Kind: ir.VReceiptAmount, Word: "counters"}},
		{"Excess", gn("AmountOfExcessDamageDealtThisWay", nil), &ir.Value{Kind: ir.VExcessDamage}},
		// The third argument goes through the general c.Ref: no spell
		// reference is hard-wired.
		{"SpellsBeforeThatSpell", gn("NumSpellsCastByPlayerBeforeSpellThisTurn", []any{valNode("Spells", "Any", nil), player, valNode("Spell", "Trigger_ThatSpell", nil)}),
			&ir.Value{Kind: ir.VSpellsCastBeforeThisTurn, Filter: pf, Ref: pr, Ref2: &ir.Ref{Kind: ir.RefEventObject, Src: "Trigger_ThatSpell"}}},
		{"SpellsBeforeTargetSpell", gn("NumSpellsCastByPlayerBeforeSpellThisTurn", []any{valNode("Spells", "Any", nil), player, valNode("Spell", "Ref_TargetSpell", nil)}),
			&ir.Value{Kind: ir.VSpellsCastBeforeThisTurn, Filter: pf, Ref: pr, Ref2: &ir.Ref{Kind: ir.RefTarget, Src: "Ref_TargetSpell"}}},
		{"SpellsBeforeThisSpell", gn("NumSpellsCastByPlayerBeforeSpellThisTurn", []any{valNode("Spells", "Any", nil), player, valNode("Spell", "ThisSpell", nil)}),
			&ir.Value{Kind: ir.VSpellsCastBeforeThisTurn, Filter: pf, Ref: pr, Ref2: &ir.Ref{Kind: ir.RefSelf, Src: "ThisSpell"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := valLower(t, tc.in)
			if !ir.Equal(got, tc.want) {
				t.Errorf("got %s\nwant %s", got.String(), tc.want.String())
			}
		})
	}
}

func TestValueThousandYearStorm(t *testing.T) {
	_, ix := corpus(t)
	card, ok := ix.Lookup("Thousand-Year Storm")
	if !ok {
		t.Fatal("Thousand-Year Storm not in the index")
	}
	var found node
	for _, f := range card.Faces {
		walk(f, func(n node, enum, variant string) bool {
			if found == nil && enum == "_GameNumber" && variant == "NumSpellsCastByPlayerBeforeSpellThisTurn" {
				found = n
			}
			return found == nil
		})
	}
	if found == nil {
		t.Fatal("no NumSpellsCastByPlayerBeforeSpellThisTurn node in the card")
	}
	// Stub every nested variant under the node that this ticket does not own.
	stubs := map[string]lowerFunc{}
	walk(found["args"], func(n node, enum, variant string) bool {
		key := strings.TrimPrefix(enum, "_") + "." + variant
		switch strings.TrimPrefix(enum, "_") {
		case "Spells", "Player", "Spell":
			stubs[key] = func(c *Ctx, n node) (any, error) {
				return &ir.Ref{Kind: ir.RefYou, Src: key}, nil
			}
			if strings.TrimPrefix(enum, "_") == "Spells" {
				stubs[key] = func(c *Ctx, n node) (any, error) { return &ir.Filter{Op: ir.FAll, Word: key}, nil }
			}
			return false
		}
		return true
	})
	withStubs(t, stubs)
	v, err := (&Ctx{}).Value(found)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != ir.VSpellsCastBeforeThisTurn || v.Filter == nil || v.Ref == nil || v.Ref2 == nil {
		t.Errorf("got %s, want spells-cast-before with filter, ref and ref2 set", v.String())
	}
}
