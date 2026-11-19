package pay

import (
	"cmp"
	"slices"

	"github.com/adams-shaun/gorge/engine/state"
)

// PlanManaUnderRiderSpread is the board-shaped pinned alternative a payment
// plan under an AnyColor may-play rider needs when the payer's coloured units
// span several colours: every pip the rider relaxes takes one COLOURED unit of
// any colour ("any color" never includes colourless, CR 107.4c, CR 105.1), so
// a single-colour pin (PlanManaUnderRiderPinned) asks that one colour for all
// of them and misses a board whose units are spread — four {W/U} pips beside
// one Island, two Swamps and a Mountain (U1 B2 R1) are payable, yet no colour
// holds four units. The spread pins the pips over the colours the payment
// census can fund, largest supply first with the WUBRG order breaking ties, so
// the search funds each pinned demand from that colour's units and the rider
// lets each unit pay its printed pip at the payment. The caller's witness
// still verifies the plan against the real rider cost, so a spread that
// over-asks one colour only costs a failed planning attempt and the cast stays
// withheld — the fold-and-pin scheme never gains a looser payment than the
// rider grants.
//
// nil when no spread is wanted or provable: an AnyType rider lets colourless
// mana pay (the fold is already exact, no pin), a rider with no pips to fund
// pins nothing, and a census whose coloured supply is below the pip count
// cannot fund the pips on any distribution (the single-colour pins and the
// folded plan then answer as they did). The census read is the same view the
// planner itself plans over (PaymentPlanQueryUnits's cached units, else
// Eval().ManaUnits), so a unit whose restricted admission the payment refuses
// only inflates the bound — the witness, not the spread, decides payability.
func PlanManaUnderRiderSpread(e Engine, p state.PlayerID, c Cost, r PipRider) *Cost {
	if !r.AnyColor || r.AnyType {
		return nil
	}
	n := pipUnitsUnderRider(c, r)
	if n == 0 {
		return nil
	}
	var avail [state.MG + 1]int32
	for _, u := range PaymentPlanQueryUnits(e, p) {
		for _, alt := range PaymentPlanQueryAlternatives(e, u) {
			for i := state.MW; i <= state.MG; i++ {
				avail[i] += alt.Mana[i]
			}
		}
	}
	total := int32(0)
	for i := state.MW; i <= state.MG; i++ {
		total += avail[i]
	}
	if total < n {
		return nil
	}
	order := []int{int(state.MW), int(state.MU), int(state.MB), int(state.MR), int(state.MG)}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(avail[b], avail[a]) })
	out := PlanManaUnderRider(c, r)
	if out.Generic < n {
		return nil
	}
	out.Generic -= n
	left := n
	for _, i := range order {
		take := avail[i]
		if take > left {
			take = left
		}
		out.Colored[i] += take
		left -= take
		if left == 0 {
			break
		}
	}
	return &out
}
