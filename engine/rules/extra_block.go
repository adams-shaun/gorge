package rules

import (
	"iter"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/combat"
)

// blockerTally is one blocker's (one Group's) share of a KBlockers offer.
type blockerTally struct {
	group            string
	offered, mustAll int
	capacity         int
}

// blockerGroupLimits yields the per-blocker GroupLimits entries of a
// KBlockers offer (Decision.GroupCapFor reads them; an absent group keeps the
// at-most-one rule), in the offer's first-seen blocker order. For each blocker
// (one Group) the limit is the larger of its BlockMustAll options (a
// BlockAllDefined$ duty pair set) and combat.BlockCapacity capped by the
// number of attackers it is offered against (CR 509.1a). Limits below 2 are
// not yielded. A sequence, so the caller collects it into its map.
func blockerGroupLimits(b combat.Board, opts []decision.Option) iter.Seq2[string, int] {
	var rows []blockerTally
	for _, opt := range opts {
		i := 0
		for i < len(rows) && rows[i].group != opt.Group {
			i++
		}
		if i == len(rows) {
			rows = append(rows, blockerTally{group: opt.Group, capacity: combat.BlockCapacity(b, opt.Obj)})
		}
		rows[i].offered++
		if opt.BlockMustAll {
			rows[i].mustAll++
		}
	}
	return func(yield func(string, int) bool) {
		for _, r := range rows {
			if limit := max(r.mustAll, min(r.capacity, r.offered)); limit >= 2 && !yield(r.group, limit) {
				return
			}
		}
	}
}
