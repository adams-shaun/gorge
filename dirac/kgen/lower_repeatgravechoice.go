package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// repeatBreach lowers Breach the Multiverse's per-player graveyard choice
// exactly:
//
//	DB$ RepeatEach | RepeatPlayers$ Player | RepeatSubAbility$ <ChooseCard>
//	<ChooseCard>: DB$ ChooseCard | Defined$ You
//	    | Choices$ Creature.RememberedPlayerCtrl,Planeswalker.RememberedPlayerCtrl
//	    | ChoiceZone$ Graveyard | Mandatory$ True | Amount$ 1 | ImprintChosen$ True
//
// followed directly by DB$ ChangeZone Defined$ Imprinted (breachReturn) and,
// later, the Cleanup of the imprint. The kernel's ExtBreachChoose is the
// whole loop: the controller picks for itself first, then the other player
// (gorge repeatPlayers from the resolving controller), skipping a player
// without a candidate. Any other RepeatEach-ChooseCard shape stays refused.
func (lc *lowerCtx) repeatBreach(who lRef, ps *params, body *cards.SA) (bool, error) {
	if body.API != "ChooseCard" || body.Params["ImprintChosen"] != "True" {
		return false, nil
	}
	if who.Kind != rEachPlayer || body.Sub != nil {
		return true, refuse("RepeatEach imprint ChooseCard shape")
	}
	bp := newParams(body.Params)
	for _, w := range [][2]string{
		{"Defined", "You"},
		{"Choices", "Creature.RememberedPlayerCtrl,Planeswalker.RememberedPlayerCtrl"},
		{"ChoiceZone", "Graveyard"},
		{"Mandatory", "True"},
		{"Amount", "1"},
		{"ImprintChosen", "True"},
	} {
		if bp.str(w[0]) != w[1] {
			return true, refuse("RepeatEach imprint ChooseCard %s$ %q", w[0], bp.str(w[0]))
		}
		bp.consume(w[0])
	}
	bp.consume("ChoiceTitle", "SpellDescription", "StackDescription")
	if left := bp.unread(); len(left) > 0 {
		return true, refuse("RepeatEach imprint ChooseCard: unread params %v", left)
	}
	ps.consume("RepeatPlayers", "RepeatSubAbility")
	if n := lc.curNode.Sub; n == nil || n.API != "ChangeZone" || n.Params["Defined"] != "Imprinted" {
		return true, refuse("RepeatEach imprint ChooseCard without the Imprinted ChangeZone")
	}
	lc.emit(lInstr{Op: "BreachChoose"})
	lc.breachStage = 1
	return true, nil
}

// breachReturn lowers the ChangeZone that follows repeatBreach exactly:
//
//	DB$ ChangeZone | Defined$ Imprinted | GainControl$ True | Origin$ Graveyard
//	    | Destination$ Battlefield
//
// as ExtBreachReturn (the chosen set enters under the controller's control).
func (lc *lowerCtx) breachReturn(ps *params) error {
	lc.breachStage = 0
	for _, w := range [][2]string{
		{"Defined", "Imprinted"},
		{"GainControl", "True"},
		{"Origin", "Graveyard"},
		{"Destination", "Battlefield"},
	} {
		if ps.str(w[0]) != w[1] {
			return refuse("Imprinted ChangeZone %s$ %q", w[0], ps.str(w[0]))
		}
		ps.consume(w[0])
	}
	ps.consume("StackDescription")
	if left := ps.unread(); len(left) > 0 {
		return refuse("Imprinted ChangeZone: unread params %v", left)
	}
	lc.emit(lInstr{Op: "BreachReturn"})
	lc.breachStage = 2
	return nil
}
