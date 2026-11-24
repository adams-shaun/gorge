package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Costs: the Cost enum and its parts, the exile and remove-counter
// satellites, and the cost-reduction symbols.

// part wraps one cost part as a Cost.
func part(p *ir.CostPart) *ir.Cost { return &ir.Cost{Parts: []*ir.CostPart{p}} }

// chooseOne is a choice of one object matching f.
func chooseOne(f *ir.Filter) *ir.Ref {
	return &ir.Ref{Kind: ir.RefChoose, Filter: f, N: refConst1()}
}

// costPart lowers x to an *ir.CostPart.
func (c *Ctx) costPart(x any) (*ir.CostPart, error) { return lowerAs[ir.CostPart](c, x) }

// manaSyms lowers a list of mana symbols.
func manaSyms(c *Ctx, xs []any) ([]ir.ManaSym, error) {
	var out []ir.ManaSym
	for _, x := range xs {
		v, err := c.lower(x)
		if err != nil {
			return nil, err
		}
		s, ok := v.(*ir.ManaSym)
		if !ok {
			return nil, fmt.Errorf("%s lowers to %T, want a mana symbol", qualified(x), v)
		}
		out = append(out, *s)
	}
	return out, nil
}

// reduction sums a list of CostReduceGeneric symbols. Any other symbol is a
// refusal.
func reduction(c *Ctx, syms []any) (int32, error) {
	var sum int32
	for _, x := range syms {
		v, err := c.lower(x)
		if err != nil {
			return 0, err
		}
		s, ok := v.(*ir.ManaSym)
		if !ok || s.Kind != ir.ManaGeneric {
			return 0, &RefusalError{Key: "CostReductionSymbol:shape", Path: c.Src()}
		}
		sum += s.N
	}
	return sum, nil
}

// costPartsOf lowers each element to a part, as one cost.
func costPartsOf(c *Ctx, xs []any) (*ir.Cost, error) {
	out := &ir.Cost{}
	for _, x := range xs {
		p, err := c.costPart(x)
		if err != nil {
			return nil, err
		}
		out.Parts = append(out.Parts, p)
	}
	return out, nil
}

// costPartReg registers a Cost variant that is one part built from the
// variant's node.
func costPartReg(key string, f func(c *Ctx, n node) (*ir.CostPart, error)) {
	register(key, func(c *Ctx, n node) (any, error) {
		p, err := f(c, n)
		if err != nil {
			return nil, err
		}
		return part(p), nil
	})
}

func init() {
	register("Cost.And", func(c *Ctx, n node) (any, error) {
		out := &ir.Cost{}
		for _, x := range list(rawArgs(n)) {
			sub, err := c.Cost(x)
			if err != nil {
				return nil, err
			}
			out.Mana = append(out.Mana, sub.Mana...)
			out.HasMana = out.HasMana || sub.HasMana
			out.Parts = append(out.Parts, sub.Parts...)
		}
		return out, nil
	})
	register("Cost.PayMana", func(c *Ctx, n node) (any, error) {
		m, err := manaSyms(c, list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Cost{Mana: m, HasMana: true}, nil
	})
	register("Cost.PayManaX", func(c *Ctx, n node) (any, error) {
		m, err := manaSyms(c, list(argN(n, 0)))
		if err != nil {
			return nil, err
		}
		v, err := c.Value(argN(n, 1))
		if err != nil {
			return nil, err
		}
		if v.Kind != ir.VX {
			return nil, &RefusalError{Key: "Cost.PayManaX:value", Path: c.Src()}
		}
		return &ir.Cost{Mana: m, HasMana: true}, nil
	})
	costPartReg("Cost.TapPermanent", func(c *Ctx, n node) (*ir.CostPart, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		if r.Kind == ir.RefSelf && r.Of == nil {
			return &ir.CostPart{Kind: ir.CostTapSelf}, nil
		}
		return &ir.CostPart{Kind: ir.CostTapUntapped, Obj: r}, nil
	})
	costPartReg("Cost.TapNumberPermanents", func(c *Ctx, n node) (*ir.CostPart, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		f, err := c.Filter(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostTapUntapped, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: f, N: v}}, nil
	})
	costPartReg("Cost.SacrificePermanent", func(c *Ctx, n node) (*ir.CostPart, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostSacrifice, Obj: r}, nil
	})
	costPartReg("Cost.SacrificeAPermanent", func(c *Ctx, n node) (*ir.CostPart, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostSacrifice, Obj: chooseOne(f)}, nil
	})
	costPartReg("Cost.DiscardACard", func(c *Ctx, n node) (*ir.CostPart, error) {
		return &ir.CostPart{Kind: ir.CostDiscard, Obj: chooseOne(anyCardIn(ir.ZoneHand))}, nil
	})
	costPartReg("Cost.DiscardHand", func(c *Ctx, n node) (*ir.CostPart, error) {
		return &ir.CostPart{Kind: ir.CostDiscard, Obj: &ir.Ref{Kind: ir.RefAll, Filter: anyCardIn(ir.ZoneHand)}}, nil
	})
	costPartReg("Cost.PayLife", func(c *Ctx, n node) (*ir.CostPart, error) {
		v, err := c.Value(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostPayLife, N: v}, nil
	})
	costPartReg("Cost.Loyalty", func(c *Ctx, n node) (*ir.CostPart, error) {
		v, ok := intOf(rawArgs(n))
		if !ok {
			return nil, fmt.Errorf("cardc: %s: loyalty cost needs a number", c.Src())
		}
		return &ir.CostPart{Kind: ir.CostLoyalty, N: constV(int32(v))}, nil
	})
	register("Cost.Exile", func(c *Ctx, n node) (any, error) {
		if len(list(argN(n, 1))) > 0 {
			return nil, &RefusalError{Key: "Cost.Exile:flag", Path: c.Src()}
		}
		return costPartsOf(c, list(argN(n, 0)))
	})
	costPartReg("Cost.PutAPermanentIntoItsOwnersHand", func(c *Ctx, n node) (*ir.CostPart, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostReturnToHand, Obj: chooseOne(f)}, nil
	})
	register("Cost.RemoveCounters", func(c *Ctx, n node) (any, error) {
		return costPartsOf(c, list(rawArgs(n)))
	})
	costPartReg("Cost.RevealTopCardOfLibrary", func(c *Ctx, n node) (*ir.CostPart, error) {
		return &ir.CostPart{Kind: ir.CostReveal, Obj: &ir.Ref{Kind: ir.RefTopOfLibrary, N: refConst1()}}, nil
	})

	// ExilableCost.
	register("ExilableCost.Permanent", func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostExile, Obj: r}, nil
	})
	register("ExilableCost.ACardOfTypeFromPlayersHand", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		pl, err := c.Ref(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostExile, Obj: &ir.Ref{Kind: ir.RefChoose, Of: pl, Filter: f, N: refConst1()}}, nil
	})

	// RemoveCountersCost.
	register("RemoveCountersCost.ACounterOfTypeFromPermanent", func(c *Ctx, n node) (any, error) {
		w, err := counterWord(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := c.Ref(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostRemoveCounters, Obj: r, N: refConst1(), Word: w}, nil
	})
	register("RemoveCountersCost.NumberCountersOfTypeFromPermanent", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		w, err := counterWord(c, argN(n, 1))
		if err != nil {
			return nil, err
		}
		r, err := c.Ref(argN(n, 2))
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostRemoveCounters, Obj: r, N: v, Word: w}, nil
	})
	register("RemoveCountersCost.NumberCountersFromAmongPermanents", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := refAllOf(c, n, 1)
		if err != nil {
			return nil, err
		}
		return &ir.CostPart{Kind: ir.CostRemoveCounters, Obj: r, N: v}, nil
	})

	register("ReplacementActionWouldEnterCost.PayLife", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return part(&ir.CostPart{Kind: ir.CostPayLife, N: v}), nil
	})

	// Cost-reduction symbols.
	register("CostReductionSymbol.CostReduceGeneric", func(c *Ctx, n node) (any, error) {
		v, ok := intOf(rawArgs(n))
		if !ok {
			return nil, fmt.Errorf("cardc: %s: generic reduction needs a number", c.Src())
		}
		return &ir.ManaSym{Kind: ir.ManaGeneric, N: int32(v)}, nil
	})
	for l, cs := range map[string]ir.ColorSet{"W": ir.ColorW, "U": ir.ColorU, "B": ir.ColorB, "R": ir.ColorR, "G": ir.ColorG} {
		sym := ir.ManaSym{Kind: ir.ManaColored, Colors: cs}
		register("CostReductionSymbol.CostReduce"+l, func(*Ctx, node) (any, error) { s := sym; return &s, nil })
	}
	register("CostReductionSymbolX.CostReduceX", func(*Ctx, node) (any, error) {
		return &ir.ManaSym{Kind: ir.ManaX}, nil
	})
}

// counterWord lowers a CounterType to its word.
func counterWord(c *Ctx, x any) (string, error) {
	v, err := c.lower(x)
	if err != nil {
		return "", err
	}
	w, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s lowers to %T, want a counter word", qualified(x), v)
	}
	return w, nil
}
