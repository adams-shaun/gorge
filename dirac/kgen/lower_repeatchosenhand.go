package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatEachChosenHand lowers api:RepeatEach RepeatCards$ Card.ChosenCard |
// Zone$ Hand (Sylvan Library): gorge's repeatedCards walks the hands for the
// resolution's chosen cards, and a ChooseCard earlier in the chain chose them
// from its controller's hand, so the subjects are exactly the chosen cards
// still in that hand, in hand order. The header reads them through RefChosen
// (dirac act_repeatcards.go eachCardList) rather than a battlefield walk. The
// loop is the plain resumable frame (repeatEachCardsLoop): its body may ask
// (the UnlessCost$ payment, the hand pick). ChooseOrder$ True rides
// kop.RepeatEachOrder: the controller orders the subjects before the first
// pass, as gorge's repeatEachChooseOrder asks.
func (lc *lowerCtx) repeatEachChosenHand(ps *params, in lInstr) error {
	if ps.is("ChangeZoneTable", "True") || ps.peek("DamageMap") != "" {
		return refuse("RepeatEach chosen hand cards with a zone or damage batch")
	}
	if _, ok := ps.get("UseImprinted"); ok {
		return refuse("RepeatEach chosen hand cards UseImprinted$")
	}
	order := false
	if v, ok := ps.get("ChooseOrder"); ok {
		if v != "True" {
			return refuse("RepeatEach cards ChooseOrder$ %q", v)
		}
		order = true
	}
	name := ps.str("RepeatSubAbility")
	body := cards.ResolveSVar(lc.face.SVars, name)
	if body == nil {
		return refuse("RepeatEach RepeatSubAbility %q", name)
	}
	in.Obj = lRef{Kind: rChosen}
	at := len(lc.ab.Prog)
	if err := lc.repeatEachCardsLoop(in, body); err != nil {
		return err
	}
	if order {
		lc.ab.Prog[at].C |= kop.RepeatEachOrder
	}
	return nil
}
