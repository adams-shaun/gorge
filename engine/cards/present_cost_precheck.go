package cards

import "strings"

// PresentIsOwnCostPrecheck reports whether a trigger's IsPresent$ clause is
// only Forge's payability precheck for the trigger's own optional counter
// cost rather than an intervening-if (CR 603.4).
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
func (t Trigger) PresentIsOwnCostPrecheck() bool {
	if t.Effect == nil {
		return false
	}
	for _, k := range []ParamKey{PKPresentCompare, PKPresentDefined, PKPresentZone, PKIsPresent2} {
		if _, ok := t.Param(k); ok {
			return false
		}
	}
	if _, ok := t.Params["PresentPlayer"]; ok {
		return false
	}
	cost := strings.TrimSpace(t.Effect.ParamStr(PKCost))
	body, ok := strings.CutPrefix(cost, "SubCounter<")
	if !ok {
		return false
	}
	body, ok = strings.CutSuffix(body, ">")
	parts := strings.Split(body, "/")
	if !ok || len(parts) != 2 || parts[0] == "" || strings.Trim(parts[0], "0123456789") != "" || parts[1] == "" {
		return false
	}
	if strings.TrimSpace(t.ParamStr(PKIsPresent)) != "Card.Self+counters_GE"+parts[0]+"_"+parts[1] {
		return false
	}
	return !strings.Contains(strings.ToLower(t.ParamStr(PKTriggerDescription)), ", if ")
}
