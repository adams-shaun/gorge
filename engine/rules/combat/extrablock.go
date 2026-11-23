package combat

import (
	"math"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// BlockCapacity is how many attackers blocker may be declared against: 1 plus every live CanBlockAmount$
// static's amount, or unbounded when a CanBlockAny$ True static or a resolved Pump's CanBlockAny$ True grant applies (CR 509.1a, 702.22h aside).
//
// It is the one home of the rule: askBlockers publishes it as the blocker's
// Decision.GroupLimits entry and validateBlockers enforces it, so the offer,
// the validator and the bot top-up cannot drift. A non-literal amount
// (CanBlockAmount$ X, Kemba's Legion's Equipment count) is skipped -- the
// literalBlockCount fail-closed convention: the blocker keeps its printed
// capacity rather than a mis-evaluated one. Zero matching statics returns
// exactly 1.
func BlockCapacity(b Board, blocker state.ObjID) int {
	// A resolved Pump's CanBlockAny$ True (Give No Ground, Valor Made Real)
	// is a registered until-end-of-turn restriction scoped to its creature.
	for i, ces := 0, b.Active(); i < len(ces); i++ {
		if ce := &ces[i]; ce.Restriction == effects.ModeCanBlockAny && b.RestrictionApplies(ce, blocker) {
			return math.MaxInt32
		}
	}
	total := int64(1)
	for _, sv := range b.Statics("Continuous") {
		raw, hasAmount := sv.Param(cards.PKCanBlockAmount)
		anyRaw, hasAny := sv.Param(cards.PKCanBlockAny)
		if !hasAmount && !hasAny {
			continue
		}
		if !b.StaticGateHolds(sv) {
			continue
		}
		spec := sv.ParamStr(cards.PKAffected)
		if spec == "" {
			spec = "Card.Self"
		}
		if !b.MatchesStaticSpec(spec, blocker, sv) {
			continue
		}
		if hasAny && anyRaw == "True" {
			return math.MaxInt32
		}
		if hasAmount {
			if n, ok := literalBlockCount(raw); ok {
				total += int64(n)
			}
		}
		if total > math.MaxInt32 {
			total = math.MaxInt32
		}
	}
	return int(total)
}
