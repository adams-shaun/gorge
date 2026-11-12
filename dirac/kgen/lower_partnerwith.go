package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// lower_partnerwith.go: "Partner with [name]" (CR 702.124j). The keyword is
// two abilities. The deck-construction one is the commander-pair marker: like
// plain Partner it has no game characteristic, and the kernel seats a pair
// from Config.Commanders. The other is the enters trigger gorge's
// cards/kw_partner_with.go expands it into: "target player may search their
// library for a card named [name], reveal it, put it into their hand, then
// shuffle", spelled
//
//	DB$ ChangeZone | Origin$ Library | Destination$ Hand |
//	 ChangeType$ Card.named<name> | DefinedPlayer$ Targeted |
//	 Chooser$ Targeted | Optional$ True | ValidTgts$ Player
//
// and lowered from that line by changeZoneMove's library search: the
// targeted player is the searched library's owner (definedPlayer's player
// target slot), Optional$ True is the search confirmation (SrConfirm), and
// the kernel's search asks the fetching player both the confirmation and the
// pick (chooserOf), which is gorge's searchChooser for Chooser$ Targeted: the
// same targeted player.

// targetPlayerOwnSearch lowers the target of the library search the targeted
// player makes of their own library and answers themself: DefinedPlayer$
// Targeted (whose library) with Chooser$ Targeted (who answers) over one
// ValidTgts$ Player slot, into the hand. It reports false, with nothing read,
// for any other ChangeZone; only this shape admits a ChangeType$ beside
// ValidTgts$ for a library origin. The slot is the player target
// definedPlayer reads next; Chooser$ is consumed here because the kernel's
// default chooser is already that player.
func (lc *lowerCtx) targetPlayerOwnSearch(ps *params, origin, dest int) (bool, error) {
	if origin != zLibrary || dest != zHand || ps.peek("ValidTgts") != "Player" ||
		ps.peek("DefinedPlayer") != "Targeted" || ps.peek("Chooser") != "Targeted" {
		return false, nil
	}
	for _, k := range []string{"TargetMin", "TargetMax", "OriginAlternative", "ChooseFromDefined", "RememberSearcher"} {
		if _, ok := ps.m[k]; ok {
			return false, nil
		}
	}
	if lc.lastTarget() >= 0 {
		return false, refuse("a targeted player's own library search after another target slot")
	}
	pf, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return false, err
	}
	if len(pf.Clauses) != 0 || pf.Players == 0 {
		return false, refuse("library search ValidTgts$ %q", ps.str("ValidTgts"))
	}
	lc.addTarget(lTarget{F: pf})
	ps.consume("Chooser", "TgtPrompt")
	return true, nil
}

// partnerWithTrigger reports whether the face carries gorge kwPartnerWith's
// expansion: an enters trigger tagged Keyword$ Partner with whose body is
// the targeted player's own library search (targetPlayerOwnSearch).
func (lc *lowerCtx) partnerWithTrigger() bool {
	for _, t := range lc.face.Triggers {
		if t.Params["Keyword"] != "Partner with" || t.Mode != "ChangesZone" || t.Params["ValidCard"] != "Card.Self" ||
			t.Params["Destination"] != "Battlefield" {
			continue
		}
		body := cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
		if body != nil && body.API == "ChangeZone" && body.Params["DefinedPlayer"] == "Targeted" &&
			body.Params["Chooser"] == "Targeted" && body.Params["Origin"] == "Library" {
			return true
		}
	}
	return false
}
