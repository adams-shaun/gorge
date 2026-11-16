package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// potentialSubCounterBounded (the pay gate only: potentialManaCounted) returns ma with its Amount$ X read at the most
// counters its announced SubCounter<X/Kind> cost can settle right now
// (pay.ManaSubCounterXBound), and true when ma is that shape (Icatian
// Store's "{T}, Remove X storage counters: Add {W} for each counter
// removed"). A bare Amount$ X prices unbounded in PotentialMana (the X is
// unknown to the ability's text), which would make a one-counter Store cover
// a {W}{W} trigger cost no activation can reach (CR 118.3: a cost that
// can't be paid isn't offered as payable). The bound is the same one the
// activation announces X against, so the offer and the activation agree.
func potentialSubCounterBounded(e *Engine, p state.PlayerID, id state.ObjID, ma *cards.SA) (*cards.SA, bool) {
	if !strings.EqualFold(effects.ManaOf(ma).AmountTrim, "x") {
		return nil, false
	}
	cost := e.manaAbilityCC(p, id, ma).Cost
	bound, any := pay.ManaSubCounterXBound(asPayer(e), p, e.G.Obj(id), id, cost)
	if !any {
		return nil, false
	}
	if bound < 0 {
		bound = 0
	}
	return pay.ManaAbilityWithSubX(ma, bound), true
}
