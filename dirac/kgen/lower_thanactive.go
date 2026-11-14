package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// oathPlayerTargets are the five ValidTgts$ spellings of the Oath cycle (Oath
// of Druids, Lieges, Ghouls, Scholars, Mages): an opponent of the ACTIVE
// player whose count of one quantity beats (or, for the graveyard, trails)
// the active player's. gorge reads them in engine/effects/player_filter.go
// (Player.OpponentToActive and engine/effects/player_than_active.go); the
// kernel narrows a TgtPlayer slot with kop.TgfOppToActive and the ThanActive
// field of kop.TgfThanMask (dirac/tgt_thanactive.go).
var oathPlayerTargets = map[string]int{
	"withMoreCreaturesThanActive":       kop.ThanCreatures,
	"withMoreLandsThanActive":           kop.ThanLands,
	"hasMoreCardsInHandThanActive":      kop.ThanHandCards,
	"hasMoreLifeThanActive":             kop.ThanLife,
	"hasFewerCreaturesInYardThanActive": kop.ThanYardFewer,
}

// oathPlayerTarget recognizes one exact Oath spelling and returns its
// comparison with the spec reduced to the plain "Player" head the generic
// target lowering reads. Any other spec comes back unchanged and false.
func oathPlayerTarget(spec string) (int, string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(spec), "Player.OpponentToActive+")
	if !ok {
		return 0, spec, false
	}
	n, ok := oathPlayerTargets[rest]
	if !ok {
		return 0, spec, false
	}
	return n, "Player", true
}

// thanActiveChooser reads the Oath target's TargetingPlayer$ TriggeredPlayer
// (gorge targetChooserCore, engine/rules/cast_targets.go: the player whose
// upkeep it is answers the target ask, legality judged from the controller)
// onto the slot as kop.TgfTrigChooser. Without a Phase trigger the target
// ask has no trigger player to move to, and the spelling refuses.
func (lc *lowerCtx) thanActiveChooser(ps *params, t *lTarget) error {
	v, ok := ps.get("TargetingPlayer")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "TriggeredPlayer" {
		return refuse("TargetingPlayer$ %q beside an Oath player target", v)
	}
	if tr := lc.ab.Trigger; tr == nil || tr.Event != "Phase" {
		return refuse("TargetingPlayer$ TriggeredPlayer outside a Phase trigger")
	}
	t.TrigChooser = true
	return nil
}

// digUntilOptional reads DigUntil's Optional$ True (gorge digUntilMayReveal:
// the dug player is asked yes/no before the reveal). It lowers to an OpMayAsk
// (kop.MayGuardDigUntil) over the one DigUntil instruction, asking the dug
// player; only the plain walk takes it (the imprint and remembered-revealed
// shapes emit more than one instruction), over one player the kernel names:
// the controller or the trigger player. Any other Optional$ value refuses.
func (lc *lowerCtx) digUntilOptional(ps *params, who lRef) (bool, error) {
	v, ok := ps.get("Optional")
	if !ok {
		return false, nil
	}
	if strings.TrimSpace(v) != "True" {
		return false, refuse("DigUntil Optional$ %q", v)
	}
	switch who.Kind {
	case rYou, rTriggeredPlayer:
	default:
		return false, refuse("DigUntil Optional$ over %s", who)
	}
	if ps.is("ImprintFound", "True") || ps.is("RememberRevealed", "True") {
		return false, refuse("DigUntil Optional$ beside ImprintFound$/RememberRevealed$")
	}
	return true, nil
}
