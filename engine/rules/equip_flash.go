package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// equipAtInstantSpeed reports whether a live printed CastWithFlash static whose
// ValidSA$ names Activated.Equip lets p activate the equip ability ab of
// permanent id "any time you could cast an instant" (CR 702.6; Leonin Shikari,
// Forge Anew). It reads the same Caster$ / Condition$ timing / EffectZone$ /
// ValidCard$ gates loyaltyAtInstantSpeed does. The grant changes timing only;
// ab must be a minted Equip SA (Fortify is not an equip ability).
func (e *Engine) equipAtInstantSpeed(p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
	if ab == nil || strings.TrimSpace(ab.ParamStr(cards.PKKeyword)) != "Equip" {
		return false
	}
	for _, sv := range e.activeStatics("CastWithFlash") {
		if !equipFlashValidSA(sv.ParamStr(cards.PKValidSA)) {
			continue
		}
		if !e.actorMatches(sv, "Caster", p) || !e.staticTimingGate(sv) {
			continue
		}
		if az, ok := sv.Param(cards.PKEffectZone); ok {
			src := e.G.Obj(sv.Source)
			if src == nil || !affectedZoneOK(az, src.Zone) {
				continue
			}
		}
		if vc := sv.ParamStr(cards.PKValidCard); vc != "" && !e.matchesSpec(vc, id, e.staticSpecCtx(sv)) {
			continue
		}
		return true
	}
	return false
}

// equipFlashValidSA reports whether a CastWithFlash ValidSA$ names the
// equip-ability alternative (`Activated.Equip`).
func equipFlashValidSA(v string) bool {
	for alt := range effects.FilterAlternatives(v) {
		if strings.TrimSpace(alt) == "Activated.Equip" {
			return true
		}
	}
	return false
}
