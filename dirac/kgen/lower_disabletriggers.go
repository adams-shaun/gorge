package kgen

import "strings"

// disableTriggersStatic lowers S:Mode$ DisableTriggers | ValidMode$
// ChangesZone[,ChangesZoneAll] | Destination$ Battlefield | ValidCause$ F
// (CR 603.2g; Torpor Orb, Karn Argent Defender,
// Doorkeeper Thrull) to kop.StatDisableETB with F = ValidCause$. Gorge
// disableTriggersExcludes matches ValidCause$ against the event's object and
// suppresses every trigger line whose mode is listed and whose event
// destination admits ev.To; only the enters pair is read here. Any other
// grammar (ValidCard$, ValidTrigger$ -- Nowhere to Run's Ward form --,
// Origin$, another mode or destination, a gate) refuses by name.
func (lc *lowerCtx) disableTriggersStatic(sp *params, st *lStatic) error {
	for _, k := range []string{"ValidCard", "ValidTrigger", "Origin"} {
		if v, ok := sp.get(k); ok {
			return refuse("DisableTriggers %s$ %q", k, v)
		}
	}
	if st.Cond.Kind != cNone {
		return refuse("DisableTriggers with a Condition$ / IsPresent$ / CheckSVar$ gate")
	}
	modes := map[string]bool{}
	for _, m := range strings.Split(sp.str("ValidMode"), ",") {
		modes[strings.TrimSpace(m)] = true
	}
	if !modes["ChangesZone"] || len(modes) > 2 || len(modes) == 2 && !modes["ChangesZoneAll"] {
		return refuse("DisableTriggers ValidMode$ %q", sp.str("ValidMode"))
	}
	if v := sp.str("Destination"); v != "Battlefield" {
		return refuse("DisableTriggers Destination$ %q", v)
	}
	if v, ok := sp.get("Secondary"); ok && v != "True" {
		return refuse("DisableTriggers Secondary$ %q", v)
	}
	v, ok := sp.get("ValidCause")
	if !ok {
		return refuse("DisableTriggers without ValidCause$")
	}
	f, err := lc.filter(v)
	if err != nil {
		return err
	}
	st.F = f
	return nil
}
