package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ReplacedCardLKI$CardPower is the power the replaced creature had just before
// a dies-to-exile rider moved it (CR 608.2h; gorge ctx.Repl.ReplacedPower read
// by evalReplacedCardLKIOK). The kernel captures it on the dying slot before
// the move (exRiderMark.pow, dirac/repl_exilerider.go) and exposes it to the
// rider's prog, so the read is exact only inside that prog: an SVar the rider
// reads anywhere else refuses.
func init() { exprKinds[vReplLKIPow] = kop.ExReplLKIPower }

func (lc *lowerCtx) replacedCardLKIValue(rest string) (lValue, error) {
	if rest != "CardPower" {
		return lValue{}, fmt.Errorf("ReplacedCardLKI$%s", rest)
	}
	if !lc.exileRider {
		return lValue{}, refuse("ReplacedCardLKI$CardPower outside a dies-to-exile rider")
	}
	return lValue{Kind: vReplLKIPow}, nil
}
