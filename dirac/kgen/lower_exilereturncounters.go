package kgen

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
)

// exileReturnCounters reads ChangeZone's ReturnAbility$/ReturnValid$ rider on
// a Duration$ UntilHostLeavesPlay exile (Lagrella, the Magpie; gorge
// engine/rules/exile_return_ability.go runExileReturnAbility, CR 610.3): when
// a card returns, if it matches ReturnValid$ the named ability runs on it.
// Only the exact shape "a returned card its exiler's controller controls gets
// N +1/+1 counters" (ReturnValid$ Card.IsTriggerRemembered+YouCtrl, the
// return ability one PutCounter P1P1 CounterNum$ N on TriggeredCardLKICopy,
// N in 1..7) lowers; N is returned (0: no rider). Anything else refuses.
func (lc *lowerCtx) exileReturnCounters(ps *params) (int, error) {
	ra, hasRA := ps.get("ReturnAbility")
	rv, hasRV := ps.get("ReturnValid")
	if !hasRA && !hasRV {
		return 0, nil
	}
	if !hasRA || !hasRV || rv != "Card.IsTriggerRemembered+YouCtrl" {
		return 0, refuse("ReturnAbility$/ReturnValid$ shape")
	}
	sa := cards.ResolveSVar(lc.face.SVars, ra)
	if sa == nil || sa.API != "PutCounter" || sa.Sub != nil {
		return 0, refuse("ReturnAbility$ %q is not one PutCounter", ra)
	}
	p := sa.Params
	n, err := strconv.Atoi(p["CounterNum"])
	if err != nil || n < 1 || n > 7 || p["CounterType"] != "P1P1" || p["Defined"] != "TriggeredCardLKICopy" {
		return 0, refuse("ReturnAbility$ PutCounter shape")
	}
	for k := range p {
		switch k {
		case "CounterNum", "CounterType", "Defined", "SpellDescription":
		default:
			return 0, refuse("ReturnAbility$ PutCounter param %s", k)
		}
	}
	return n, nil
}
