package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectLandTapRepl lowers an Effect's one ReplacementEffects$ line that
// makes the controller's lands enter tapped this turn (Nahiri's
// Lithoforming's LandETB):
//
//	Event$ Moved | ValidCard$ Land.YouCtrl | Destination$ Battlefield |
//	ReplaceWith$ Body | ReplacementResult$ Updated
//	DB$ Tap | ETB$ True | Defined$ ReplacedCard
//
// gorge registers it as a live effect-created Moved replacement
// (effects/restrictions.go replacementMovedBodyLive's Tap arm) for the
// Effect's no-Duration$ lifetime, this turn. The kernel's ExtTurnRepl
// TurnReplEtbLandTapped is a per-player count read as a land enters
// (dirac/repl_landtapped.go). A ValidCard$ Permanent line (Due Respect)
// stays unread: gorge keeps it on the loud-Note path and never applies it.
// Any other shape is left for effectGraveRepl and the unread refusal.
func (lc *lowerCtx) effectLandTapRepl(ps *params) error {
	if !lc.isEffectLandTapLine(ps) {
		return nil
	}
	ps.consume("ReplacementEffects")
	lc.emit(lInstr{Op: "TurnRepl", C: kop.TurnReplEtbLandTapped, Note: "each land the controller controls enters tapped until end of turn"})
	return nil
}

// isEffectLandTapLine reports whether the Effect's ReplacementEffects$ names
// exactly the one line effectLandTapRepl lowers.
func (lc *lowerCtx) isEffectLandTapLine(ps *params) bool {
	name := strings.TrimSpace(ps.peek("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return false
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "Moved" {
		return false
	}
	rp := newParams(r.Params)
	rp.consume("Event", "Description")
	if !rp.is("ValidCard", "Land.YouCtrl") || !rp.is("Destination", "Battlefield") ||
		!rp.is("ReplacementResult", "Updated") {
		return false
	}
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if len(body) != 3 || body["DB"] != "Tap" || body["ETB"] != "True" || body["Defined"] != "ReplacedCard" {
		return false
	}
	return len(rp.unread()) == 0
}
