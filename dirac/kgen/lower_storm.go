package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Printed K:Storm (CR 702.40a; dirac/kw_storm.go). gorge kwStorm
// (engine/cards/kw_storm.go) expands the keyword into one SpellCast
// Card.Self trigger whose body is the __kwStorm SVar it mints, DB$
// CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$
// TriggerCount$CurrentStormCount/Minus1 | MayChooseTarget$ True. The body is
// never lowered: its amount is the fire-time storm count -- the spell cast
// before it this turn, by any player, captured when the trigger fires (gorge
// rules/trigger_referents.go's TriggerStormCount over h.CastThisTurn()) --
// which the kernel reads off its own turn history when it enqueues the
// trigger, so the trigger is the kernel-native abStorm the StatStackKw Storm
// grant uses (dirac/kw_storm.go queueStorm on CardDef.Storm), and a copy of a
// permanent spell resolves into a token (genPrep.anyPermCopy;
// dirac/copy_permanentspell.go, CR 707.10f). The validators below refuse a
// line whose shape is not exactly the expander's, so no other
// CopySpellAbility body slips past the skip.

// stormBodyOK reports whether body is the printed K:Storm expansion's body
// (gorge StormGrantedBody, engine/events/apply_trigger.go, which the
// __kwStorm SVar and the granted walk share) and nothing more.
func stormBodyOK(sa *cards.SA) bool {
	if sa == nil || sa.Kind != "DB" || sa.API != "CopySpellAbility" ||
		sa.Params["Defined"] != "TriggeredSpellAbility" ||
		sa.Params["Amount"] != "TriggerCount$CurrentStormCount/Minus1" ||
		sa.Params["MayChooseTarget"] != "True" {
		return false
	}
	for k := range sa.Params {
		switch k {
		case "Defined", "Amount", "MayChooseTarget":
		default:
			return false
		}
	}
	return true
}

// stormExpansion reports whether t is the printed K:Storm expansion's trigger
// (gorge kwStorm's addKeywordTrigger: Mode$ SpellCast, ValidCard$ Card.Self,
// TriggerZones$ Stack, Keyword$ Storm, Execute$ __kwStorm).
func (lc *lowerCtx) stormExpansion(t cards.Trigger) bool {
	if t.Mode != "SpellCast" || t.Effect == nil ||
		strings.TrimSpace(t.Params["Keyword"]) != "Storm" ||
		strings.TrimSpace(t.Params["ValidCard"]) != "Card.Self" ||
		strings.TrimSpace(t.Params["TriggerZones"]) != "Stack" {
		return false
	}
	return stormBodyOK(cards.ResolveSVar(lc.face.SVars, t.Params["Execute"]))
}

// scanStorm finds the face's printed K:Storm expansion trigger before the
// lines lower: the Storm keyword case (lower.go) sets CardDef.Storm off it
// and refuses the keyword without it, and lowerLines skips the line itself
// (the firstCastTrig shape).
func (lc *lowerCtx) scanStorm() {
	lc.stormTrig = 0
	for i, t := range lc.face.Triggers {
		if lc.stormExpansion(t) {
			lc.stormTrig = i + 1
			return
		}
	}
}
