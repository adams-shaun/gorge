package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// condArg returns positional argument i of n, or an error when absent.
func condArg(n node, i int) (any, error) {
	a := argsOf(n)
	if i >= len(a) {
		return nil, fmt.Errorf("cardc: %s: missing argument %d", qualified(n), i)
	}
	return a[i], nil
}

// condRefFilter lowers (Ref, Filter) arguments at positions ri and fi.
func condRefFilter(c *Ctx, n node, ri, fi int, kind ir.CondKind) (any, error) {
	rx, err := condArg(n, ri)
	if err != nil {
		return nil, err
	}
	fx, err := condArg(n, fi)
	if err != nil {
		return nil, err
	}
	r, err := c.Ref(rx)
	if err != nil {
		return nil, err
	}
	f, err := c.Filter(fx)
	if err != nil {
		return nil, err
	}
	return &ir.Cond{Kind: kind, Ref: r, Filter: f}, nil
}

// condEventFilter is {CRefMatches, Ref: event object, Filter: arg 0}.
func condEventFilter(c *Ctx, n node) (any, error) {
	fx, err := condArg(n, 0)
	if err != nil {
		return nil, err
	}
	f, err := c.Filter(fx)
	if err != nil {
		return nil, err
	}
	return &ir.Cond{Kind: ir.CRefMatches, Ref: &ir.Ref{Kind: ir.RefEventObject}, Filter: f}, nil
}

// condFilterOnly is {kind, Filter: arg 0, Receipt: rc}.
func condFilterOnly(kind ir.CondKind, rc ir.ReceiptKind) lowerFunc {
	return func(c *Ctx, n node) (any, error) {
		fx, err := condArg(n, 0)
		if err != nil {
			return nil, err
		}
		f, err := c.Filter(fx)
		if err != nil {
			return nil, err
		}
		return &ir.Cond{Kind: kind, Filter: f, Receipt: rc}, nil
	}
}

// condAllFilter lowers the filters at positions i and j to one FAll, in order.
func condAllFilter(c *Ctx, n node, i, j int) (*ir.Filter, error) {
	var args []*ir.Filter
	for _, k := range []int{i, j} {
		x, err := condArg(n, k)
		if err != nil {
			return nil, err
		}
		f, err := c.Filter(x)
		if err != nil {
			return nil, err
		}
		args = append(args, f)
	}
	return &ir.Filter{Op: ir.FAll, Args: args}, nil
}

// condList lowers a Vec<Condition>.
func condList(c *Ctx, n node, kind ir.CondKind) (any, error) {
	out := &ir.Cond{Kind: kind}
	for _, x := range argsOf(n) {
		s, err := c.Cond(x)
		if err != nil {
			return nil, err
		}
		out.Args = append(out.Args, s)
	}
	return out, nil
}

func init() {
	register("Condition.PlayerPassesFilter", func(c *Ctx, n node) (any, error) {
		return condRefFilter(c, n, 0, 1, ir.CRefMatches)
	})
	register("Condition.PermanentPassesFilter", func(c *Ctx, n node) (any, error) {
		return condRefFilter(c, n, 0, 1, ir.CRefMatches)
	})
	register("Condition.EnteringPermanentPassesFilter", condEventFilter)
	register("Condition.DeadPermanentPassesFilter", condEventFilter)
	register("Condition.APlayerPassesFilter", func(c *Ctx, n node) (any, error) {
		f, err := condAllFilter(c, n, 0, 1)
		if err != nil {
			return nil, err
		}
		return &ir.Cond{Kind: ir.CExistsPlayer, Filter: f}, nil
	})
	register("Condition.NumPermanentsPassFilter", func(c *Ctx, n node) (any, error) {
		cx, err := condArg(n, 0)
		if err != nil {
			return nil, err
		}
		cmp, err := c.Compare(cx)
		if err != nil {
			return nil, err
		}
		f, err := condAllFilter(c, n, 1, 2)
		if err != nil {
			return nil, err
		}
		return &ir.Cond{Kind: ir.CCountCompare, Filter: f, Cmp: cmp.Op, R: cmp.V}, nil
	})
	register("Condition.CostWasPaid", func(c *Ctx, n node) (any, error) {
		return &ir.Cond{Kind: ir.CCostPaid}, nil
	})
	register("Condition.IsPlayersTurn", func(c *Ctx, n node) (any, error) {
		x, err := condArg(n, 0)
		if err != nil {
			return nil, err
		}
		r, err := c.Ref(x)
		if err != nil {
			return nil, err
		}
		return &ir.Cond{Kind: ir.CIsTurnOf, Ref: r}, nil
	})
	register("Condition.IsAPlayersTurn", func(c *Ctx, n node) (any, error) {
		x, err := condArg(n, 0)
		if err != nil {
			return nil, err
		}
		f, err := c.Filter(x)
		if err != nil {
			return nil, err
		}
		return &ir.Cond{Kind: ir.CIsTurnOf, Filter: f}, nil
	})
	register("Condition.IsPlayersNthTurn", func(c *Ctx, n node) (any, error) {
		rx, err := condArg(n, 0)
		if err != nil {
			return nil, err
		}
		cx, err := condArg(n, 1)
		if err != nil {
			return nil, err
		}
		r, err := c.Ref(rx)
		if err != nil {
			return nil, err
		}
		cmp, err := c.Compare(cx)
		if err != nil {
			return nil, err
		}
		return &ir.Cond{Kind: ir.CNthTurn, Ref: r, Cmp: cmp.Op, R: cmp.V}, nil
	})
	register("Condition.ACardWasExiledThisWay", condFilterOnly(ir.CReceiptNonEmpty, ir.RcExiled))
	register("Condition.ACardWasRevealedThisWay", condFilterOnly(ir.CReceiptNonEmpty, ir.RcRevealed))
	register("Condition.ACreatureOrPlaneswalkerDiedThisTurn", condFilterOnly(ir.CDiedThisTurn, ir.RcNone))
	register("Condition.APermanentLeftTheBattlefieldThisTurn", condFilterOnly(ir.CLeftBattlefieldThisTurn, ir.RcNone))
	register("Condition.And", func(c *Ctx, n node) (any, error) { return condList(c, n, ir.CAll) })
	register("Condition.Or", func(c *Ctx, n node) (any, error) { return condList(c, n, ir.CAny) })
}
