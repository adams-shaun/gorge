package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// repeatExileLand lowers Ensign Pavel Chekov's per-opponent land choice
// exactly:
//
//	DB$ Dig | Defined$ Player.Opponent | ChangeNum$ All | DestinationZone$ Exile
//	    | RememberChanged$ True | SubAbility$ <RepeatEach>
//	<RepeatEach>: DB$ RepeatEach | RepeatPlayers$ Opponent | RepeatSubAbility$ <ChooseCard>
//	<ChooseCard>: DB$ ChooseCard | Choices$ Land.IsRemembered+RememberedPlayerOwn
//	    | ChoiceZone$ Exile | Mandatory$ True | Amount$ 1 | ImprintChosen$ True
//
// followed directly by an Effect over RememberObjects$ Imprinted that grants
// the play permission (exileLandPlay, via partitionMayPlay) and the Cleanup
// of the chosen, remembered and imprinted sets. The kernel's ExtExileLandChoose
// is the whole loop: the dirac engine is two-seat, so "each opponent" is the
// one other player (one iteration) and the controller picks for it, none and
// no ask without a candidate. Any other RepeatEach-ChooseCard-from-exile shape
// stays refused by name.
func (lc *lowerCtx) repeatExileLand(who lRef, ps *params, body *cards.SA) (bool, error) {
	if body.API != "ChooseCard" || body.Params["ImprintChosen"] != "True" || body.Params["ChoiceZone"] != "Exile" {
		return false, nil
	}
	if who.Kind != rEachOpp || body.Sub != nil {
		return true, refuse("RepeatEach imprint ChooseCard from exile shape")
	}
	lc.twoSeat("RepeatEach over the opponents read as one iteration")
	bp := newParams(body.Params)
	for _, w := range [][2]string{
		{"Choices", "Land.IsRemembered+RememberedPlayerOwn"},
		{"ChoiceZone", "Exile"},
		{"Mandatory", "True"},
		{"Amount", "1"},
		{"ImprintChosen", "True"},
	} {
		if bp.str(w[0]) != w[1] {
			return true, refuse("RepeatEach imprint ChooseCard from exile %s$ %q", w[0], bp.str(w[0]))
		}
		bp.consume(w[0])
	}
	if d, ok := bp.get("Defined"); ok && d != "You" {
		return true, refuse("RepeatEach imprint ChooseCard from exile Defined$ %q", d)
	}
	bp.consume("ChoiceTitle", "SpellDescription", "StackDescription")
	if left := bp.unread(); len(left) > 0 {
		return true, refuse("RepeatEach imprint ChooseCard from exile: unread params %v", left)
	}
	ps.consume("RepeatPlayers", "RepeatSubAbility")
	// the remembered set the choice reads is the Dig's exiled cards
	if n := len(lc.ab.Prog); n == 0 || lc.ab.Prog[n-1].Op != "Dig" || lc.ab.Prog[n-1].C != zExile || lc.ab.Prog[n-1].Obj.Kind != rEachOpp {
		return true, refuse("RepeatEach imprint ChooseCard from exile without the opponents' Dig to exile")
	}
	if n := lc.curNode.Sub; n == nil || n.API != "Effect" || n.Params["RememberObjects"] != "Imprinted" {
		return true, refuse("RepeatEach imprint ChooseCard from exile without the Imprinted Effect")
	}
	lc.emit(lInstr{Op: "ExileLandChoose"})
	lc.exileLandStage = 1
	return true, nil
}
