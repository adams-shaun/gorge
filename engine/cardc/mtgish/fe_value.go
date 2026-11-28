package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// valFixed registers a no-arg GameNumber that lowers to a fixed value.
func valFixed(variant string, mk func() *ir.Value) {
	register("GameNumber."+variant, func(c *Ctx, n node) (any, error) { return mk(), nil })
}

// valFilterOf registers a one-argument variant {kind, Filter: c.Filter(arg)}.
func valFilterOf(variant string, kind ir.ValueKind) {
	register("GameNumber."+variant, func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Value{Kind: kind, Filter: f}, nil
	})
}

// valRefOf registers a one-argument variant {kind, Ref: c.Ref(arg)}.
func valRefOf(variant string, kind ir.ValueKind) {
	register("GameNumber."+variant, func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Value{Kind: kind, Ref: r}, nil
	})
}

// valBinary registers Plus/Minus: {kind, Args: [a, b]}.
func valBinary(variant string, kind ir.ValueKind) {
	register("GameNumber."+variant, func(c *Ctx, n node) (any, error) {
		a, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		b, err := c.Value(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Value{Kind: kind, Args: []*ir.Value{a, b}}, nil
	})
}

func init() {
	register("GameNumber.Integer", func(c *Ctx, n node) (any, error) {
		v, ok := intOf(rawArgs(n))
		if !ok {
			return nil, fmt.Errorf("cardc: %s: Integer wants a number", c.Src())
		}
		return &ir.Value{Kind: ir.VConst, N: int32(v)}, nil
	})
	valFixed("ValueX", func() *ir.Value { return &ir.Value{Kind: ir.VX} })
	valBinary("Plus", ir.VAdd)
	valBinary("Minus", ir.VSub)
	register("GameNumber.Twice", func(c *Ctx, n node) (any, error) {
		a, err := c.Value(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Value{Kind: ir.VMul, Args: []*ir.Value{{Kind: ir.VConst, N: 2}, a}}, nil
	})

	valFilterOf("TheNumberOfPermanentsOnTheBattlefield", ir.VCount)
	valFilterOf("TheNumberOfGraveyardCards", ir.VCount)
	valFilterOf("TotalPowerOfPermanents", ir.VTotalPower)
	valFilterOf("NumDifferentManaValuesAmongPermanents", ir.VDistinctManaValues)

	valRefOf("PowerOfPermanent", ir.VPower)
	valRefOf("ToughnessOfPermanent", ir.VToughness)
	valRefOf("ManaValueOfPermanent", ir.VManaValue)
	valRefOf("LifeTotalOfPlayer", ir.VLife)
	valRefOf("StartingLifeTotalOfPlayer", ir.VStartingLife)
	valRefOf("LifeGainedByPlayerThisTurn", ir.VLifeGainedThisTurn)

	// The dying permanent, last known information.
	valFixed("PowerOfDeadPermanent", func() *ir.Value {
		return &ir.Value{Kind: ir.VPower, Ref: &ir.Ref{Kind: ir.RefEventObject}}
	})
	valFixed("ManaValueOfTheCardRevealedThisWay", func() *ir.Value {
		return &ir.Value{Kind: ir.VManaValue, Ref: &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcRevealed}}
	})

	register("GameNumber.PlayerDevotionTo", func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(argN(n, 0))
		if err != nil {
			return nil, err
		}
		col, err := c.lower(argN(n, 1))
		if err != nil {
			return nil, err
		}
		cs, ok := col.(ir.ColorSet)
		if !ok {
			return nil, fmt.Errorf("cardc: %s: colour lowers to %T, want ir.ColorSet", c.Src(), col)
		}
		return &ir.Value{Kind: ir.VDevotion, Ref: r, Colors: cs}, nil
	})

	register("GameNumber.TheTotalNumberOfCountersOfTypeAmongPermanents", func(c *Ctx, n node) (any, error) {
		w, err := c.lower(argN(n, 0))
		if err != nil {
			return nil, err
		}
		word, ok := w.(string)
		if !ok {
			return nil, fmt.Errorf("cardc: %s: counter type lowers to %T, want string", c.Src(), w)
		}
		f, err := c.Filter(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Value{Kind: ir.VCountersAmong, Word: word, Filter: f}, nil
	})

	// Spells matching Filter that Ref cast this turn before the Ref2 spell.
	register("GameNumber.NumSpellsCastByPlayerBeforeSpellThisTurn", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := c.Ref(argN(n, 1))
		if err != nil {
			return nil, err
		}
		r2, err := c.Ref(argN(n, 2))
		if err != nil {
			return nil, err
		}
		return &ir.Value{Kind: ir.VSpellsCastBeforeThisTurn, Filter: f, Ref: r, Ref2: r2}, nil
	})

	valFixed("TheChosenNumber", func() *ir.Value { return &ir.Value{Kind: ir.VChosenNumber} })
	for _, v := range []string{"Trigger_AmountOfDamageDealt", "Trigger_AmountOfLifeLost", "Trigger_ThatMuch", "WouldDealDamage_ThatMuchDamage"} {
		valFixed(v, func() *ir.Value { return &ir.Value{Kind: ir.VEventAmount} })
	}
	valFixed("LifeLostThisWay", func() *ir.Value { return &ir.Value{Kind: ir.VReceiptAmount, Word: "life-lost"} })
	valFixed("ThatManyCounters", func() *ir.Value { return &ir.Value{Kind: ir.VReceiptAmount, Word: "counters"} })
	valFixed("AmountOfExcessDamageDealtThisWay", func() *ir.Value { return &ir.Value{Kind: ir.VExcessDamage} })
}
