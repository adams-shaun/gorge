package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Target lowerings: mtgish's Target enum to *ir.Target.

type tgtCount uint8

const (
	tgtOne     tgtCount = iota // exactly one
	tgtUptoOne                 // 0..1
	tgtExact                   // n..n, the count is argument 0
	tgtUpto                    // 0..n, the count is argument 0
	tgtAny                     // 0..no limit
)

// tgtRow is one Target variant. nf is how many filter arguments follow the
// count; two combine with FAny.
type tgtRow struct {
	variant string
	stem    string
	count   tgtCount
	nf      int
}

var tgtRows = []tgtRow{
	{"TargetPermanent", "TargetPermanent", tgtOne, 1},
	{"UptoOneTargetPermanent", "TargetPermanent", tgtUptoOne, 1},
	{"TargetPlayer", "TargetPlayer", tgtOne, 1},
	{"TargetSpell", "TargetSpell", tgtOne, 1},
	{"TargetGraveyardCard", "TargetGraveyardCard", tgtOne, 1},
	{"UptoOneTargetGraveyardCard", "TargetGraveyardCard", tgtUptoOne, 1},
	{"NumberTargetPermanents", "TargetPermanents", tgtExact, 1},
	{"UptoNumberTargetPermanents", "TargetPermanents", tgtUpto, 1},
	{"AnyNumberOfTargetPermanents", "TargetPermanents", tgtAny, 1},
	{"UptoNumberTargetGraveyardCards", "TargetGraveyardCards", tgtUpto, 1},
	{"AnyTarget", "AnyTarget", tgtOne, 0},
	{"TargetPlayerOrPermanent", "TargetPlayerOrPermanent", tgtOne, 2},
}

// tgtAnyTarget is CR 115.4: a creature, planeswalker or battle, or a player.
func tgtAnyTarget() *ir.Filter {
	perm := func(t string) *ir.Filter {
		return &ir.Filter{Subject: ir.SubjPermanent, Zones: zoneBit(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PCardType, Word: t}
	}
	return &ir.Filter{Subject: ir.SubjObject, Op: ir.FAny, Args: []*ir.Filter{
		perm("Creature"), perm("Planeswalker"), perm("Battle"),
		{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny},
	}}
}

func init() {
	for _, r := range tgtRows {
		r := r
		register("Target."+r.variant, func(c *Ctx, n node) (any, error) {
			t := &ir.Target{}
			i := 0
			if r.count == tgtExact || r.count == tgtUpto {
				v, err := c.Value(argN(n, 0))
				if err != nil {
					return nil, err
				}
				i = 1
				switch {
				case v.Kind != ir.VConst:
					t.MaxV = v
				case r.count == tgtExact:
					t.Min, t.Max = v.N, v.N
				default:
					t.Max = v.N
				}
			}
			switch r.count {
			case tgtOne:
				t.Min, t.Max = 1, 1
			case tgtUptoOne:
				t.Max = 1
			case tgtAny:
				t.Max = -1
			}
			switch r.nf {
			case 0:
				t.Filter = tgtAnyTarget()
			case 1:
				f, err := c.Filter(argN(n, i))
				if err != nil {
					return nil, err
				}
				t.Filter = f
			case 2:
				a, err := c.Filter(argN(n, i))
				if err != nil {
					return nil, err
				}
				b, err := c.Filter(argN(n, i+1))
				if err != nil {
					return nil, err
				}
				t.Filter = &ir.Filter{Subject: ir.SubjObject, Op: ir.FAny, Args: []*ir.Filter{a, b}}
			default:
				return nil, fmt.Errorf("cardc: Target.%s: bad filter count", r.variant)
			}
			c.DeclareTarget(r.stem)
			return t, nil
		})
	}
}
