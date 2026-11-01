package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// subTargetAtResolution reports whether a spell's or activation's chain
// link (not the root) declares targets gorge asks for mid-resolution
// rather than as the spell is cast (rules/cast_targets.go
// collectSubTargetPreAsks): a ChangeZone link whose target zone the cast
// census cannot judge (castSubChangeZoneAnnounceable: no single
// graveyard or battlefield origin; every battlefield
// origin announces on cast), and a mandatory link whose pool or chooser reads an
// earlier target (castSubPreAskable). Fiery Annihilation's "up to one
// target Equipment attached to that creature" is the first shape.
func subTargetAtResolution(sa *cards.SA) bool {
	tp := effects.TargetsOf(sa)
	if !tp.Targeted() {
		return false
	}
	return !subChangeZoneAnnounceable(sa) || subRootUnbindable(sa)
}

func subChangeZoneAnnounceable(sa *cards.SA) bool {
	if sa.API != "ChangeZone" && sa.CompiledAPI() != cards.APIChangeZone {
		return true
	}
	tp := effects.TargetsOf(sa)
	if effects.SpecTargetsOnlyPlayers(tp.ValidTgts) {
		return true
	}
	cz := effects.ChangeZoneOf(sa)
	if tp.ZoneText != "" {
		if len(tp.Zones) != 1 || tp.Has(effects.TgtTypeStack) || tp.Has(effects.TgtValidPlayers) {
			return false
		}
		z := tp.Zones[0]
		return (z == state.ZGraveyard || z == state.ZBattlefield) && cz.OriginExactly(z)
	}
	if tp.Has(effects.TgtTypeStack) || tp.Has(effects.TgtValidPlayers) {
		return false
	}
	switch {
	case cz.OriginExactly(state.ZGraveyard):
		return true
	case cz.OriginExactly(state.ZBattlefield):
		return true // gorge (3ea231b90): every battlefield-origin link announces on cast
	}
	return false
}

// subRootUnbindable is gorge's subTargetingRootUnbindable (a bound body
// naming an earlier target counts by its text).
func subRootUnbindable(sa *cards.SA) bool {
	tp := effects.TargetsOf(sa)
	reads := func(s string) bool { return strings.Contains(s, "Targeted") || strings.Contains(s, "ParentTarget") }
	if reads(tp.TargetingPlayer) {
		return true
	}
	if strings.TrimSpace(tp.Min.Text) != "0" && (reads(tp.ValidTgts) || reads(tp.DefinedController)) {
		return true
	}
	return reads(tp.Min.Text) || reads(tp.Max.Text)
}
