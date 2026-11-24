package scriptfacts

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// PresentIsOwnCostPrecheck reports whether a trigger's IsPresent$ clause is
// only Forge's payability precheck for the trigger's own optional counter
// cost rather than an intervening-if (CR 603.4). It is the one read gorge's
// isPresentGateHolds (trigger_condition.go) and kgen's triggerIsPresentConds
// share, so the two engines cannot disagree.
//
// Sun Droplet and Living Artifact read "At the beginning of ... upkeep, you
// may remove a charge counter from CARDNAME. If you do, ...": the trigger
// event has no condition, so the ability triggers (CR 603.2) and the optional
// removal and its "If you do" happen on resolution. Their scripts still carry
// IsPresent$ Card.Self+counters_GE<N>_<KIND> beside an Execute$ whose Cost$ is
// exactly SubCounter<N/KIND>, which only mirrors that cost. A genuine
// intervening-if is printed right after the trigger condition ("At the
// beginning of your upkeep, if CARDNAME has ...": Ingenious Prodigy, the same
// script shape), so a TriggerDescription$ with ", if " keeps the gate. Any
// present rider (compare, defined set, zone, player, a second clause) keeps it
// too.
func PresentIsOwnCostPrecheck(t cards.Trigger) bool {
	if t.Effect == nil {
		return false
	}
	if _, ok := t.Param(cards.PKIsPresent2); ok {
		return false
	}
	for k := range t.Params {
		// PresentCompare$, PresentDefined$, PresentZone$, PresentPlayer$:
		// a rider makes the clause more than the cost's own mirror
		if strings.HasPrefix(k, "Present") {
			return false
		}
	}
	rest, ok := strings.CutPrefix(t.ParamStr(cards.PKIsPresent), "Card.Self+counters_GE")
	if !ok {
		return false
	}
	n, kind, ok := strings.Cut(rest, "_")
	if !ok || !allDigits(n) || kind == "" {
		return false
	}
	if t.Effect.ParamStr(cards.PKCost) != "SubCounter<"+n+"/"+kind+">" {
		return false
	}
	return !strings.Contains(t.ParamStr(cards.PKTriggerDescription), ", if ")
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}
