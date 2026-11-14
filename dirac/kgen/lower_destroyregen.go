package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// destroyRegenRepl lowers the self-regenerating destroy replacement, "If
// CARDNAME would be destroyed, regenerate it" (Mossbridge Troll, Clergy and
// Knight of the Holy Nimbus): R:Event$ Destroy | ActiveZones$ Battlefield |
// ValidCard$ Card.Self | Regeneration$ True with the body DB$ Regeneration |
// Defined$ ReplacedCard. gorge's replacementMatchesDestroy
// (engine/rules/replacement_destroy.go) admits the line only for a
// destruction that can be regenerated from, and effRegeneration regenerates
// the permanent with no shield involved; the kernel arm is ReplDestroyRegenerate
// (dirac/repl_destroyheal.go destroyLine). ok is false when the line is not
// this shape (destroyHealRepl reads it next).
func (lc *lowerCtx) destroyRegenRepl(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if !rp.is("Regeneration", "True") {
		return false, nil
	}
	if rp.str("ActiveZones") != "Battlefield" || rp.str("ValidCard") != "Card.Self" {
		return false, refuse("repl-hard: Destroy Regeneration$ replacement shape")
	}
	if !withIs(w, "Regeneration", map[string]string{"Defined": "ReplacedCard"}) {
		return false, refuse("repl-hard: Destroy Regeneration$ replacement body")
	}
	rl.Kind = "DestroyRegenerate"
	return true, nil
}
