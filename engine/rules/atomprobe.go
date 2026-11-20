package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// MatchesSpecAtZone reports whether live object id, lying in zone, matches the
// filter spec as source (0 for none) controlled by you reads it. On the
// battlefield that is the layer-aware match every rules-side filter site makes
// (matchesSpec over specCtx); elsewhere the zone-scoped match a Count$Valid
// over that zone makes, where a bare Permanent base is the permanent-card
// type test. The oracle's atom harness (dirac/oracle atomdiff) reads it beside
// the kernel's matcher; it emits nothing and changes no state.
func (e *Engine) MatchesSpecAtZone(spec string, id, source state.ObjID, you state.PlayerID, zone state.Zone) bool {
	if zone == state.ZBattlefield {
		return e.matchesSpec(spec, id, e.specCtx(source, you))
	}
	return effects.MatchesSpecInZone(e.G, spec, id, e.specCtx(source, you), zone)
}
