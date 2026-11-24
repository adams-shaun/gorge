package mtgish

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() {
	for name, op := range map[string]ir.CmpOp{
		"EqualTo":              ir.CmpEQ,
		"NotEqualTo":           ir.CmpNE,
		"LessThan":             ir.CmpLT,
		"LessThanOrEqualTo":    ir.CmpLE,
		"GreaterThan":          ir.CmpGT,
		"GreaterThanOrEqualTo": ir.CmpGE,
	} {
		register("Comparison."+name, func(c *Ctx, n node) (any, error) {
			v, err := c.Value(rawArgs(n))
			if err != nil {
				return nil, err
			}
			return &ir.Compare{Op: op, V: v}, nil
		})
	}
}
