package kgen

import (
	"strconv"
	"strings"
)

// lower_dreadtriggerpick.go: ChangeZone ChooseFromDefined$ TriggeredCards
// (Paranormal Analyst): "whenever you manifest dread, put a card you put
// into your graveyard this way into your hand". gorge's body is
// effChangeZone's hidden pick (zone_hidden.go effHiddenPick) over
// ChooseFromDefined$ TriggeredCards: the pool is the trigger object (gorge
// TriggerContext.TriggeredCards, bound from the ManifestDreadAction marker's
// ID list, engine/rules/trigger_match.go) in the game-wide graveyards (no
// DefinedPlayer$, the origin is the Graveyard: effHiddenPick's gameWide
// pool), the chooser the trigger's controller, the ask Mandatory$ True.
//
// The kernel lowers it to the game-wide graveyard pick (SrGameGrave,
// gameGraveCands) whose filter carries the trigger-object predicate
// (pred:TriggerObj, kernel isTriggerObj): the ManifestDread marker's milled
// card, none on the one-card window's empty marker.
//
// Only the Analyst's shape lowers: Origin$ Graveyard, Destination$ Hand,
// Hidden$ True, a constant ChangeNum$ 1, Mandatory$ True. Every other key
// refuses (the unread check).
func (lc *lowerCtx) dreadTriggeredPick(ps *params) error {
	if strings.TrimSpace(ps.str("Origin")) != "Graveyard" {
		return refuse("ChooseFromDefined$ TriggeredCards Origin$ %q", ps.str("Origin"))
	}
	if strings.TrimSpace(ps.str("Destination")) != "Hand" {
		return refuse("ChooseFromDefined$ TriggeredCards Destination$ %q", ps.str("Destination"))
	}
	if !ps.is("Mandatory", "True") {
		return refuse("ChooseFromDefined$ TriggeredCards without Mandatory$ True")
	}
	if v := ps.str("Hidden"); v != "True" {
		return refuse("ChooseFromDefined$ TriggeredCards Hidden$ %q", v)
	}
	n := 1
	if v, ok := ps.get("ChangeNum"); ok {
		var err error
		if n, err = strconv.Atoi(strings.TrimSpace(v)); err != nil || n != 1 {
			return refuse("ChooseFromDefined$ TriggeredCards ChangeNum$ %q", v)
		}
	}
	ps.consume("ChooseFromDefined", "Origin", "Destination", "Mandatory", "Hidden", "ChangeNum")
	f := &lFilter{Clauses: []lClause{{CmpStat: cmpTriggerObj, CmpN: 1}}}
	lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: cst(n), F: f, C: zHand, CS: "from=Graveyard|GameGrave|Mandatory"})
	return nil
}
