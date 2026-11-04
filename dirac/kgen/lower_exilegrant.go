package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Animate Abilities$ on the remembered exiled card (Q, Stacking the Deck:
// "That nonland card gains 'Exile a card from your hand: Create a copy of this
// card. You may cast the copy. Activate only during your turn and only if this
// card is exiled.'"). gorge's effAnimate registers a layer-6 AddAbilities
// effect on the card that ends when it leaves exile (Duration$ Permanent), and
// the exile granted walk (rules exileGrantedWalk) offers the body to the
// card's owner while it is exiled. The body is lowered as a latent activated
// ability of the granting card whose ActivationZone$ is Exile; the Animate
// emits GrantExile, which records the grant on each remembered exiled card.
//
// Only this shape lowers: Defined$ Remembered, Duration$ Permanent, one named
// body, an AB$ Play (copy-and-cast) with an ExileFromHand<1/Card> cost, an
// exile activation zone and PlayerTurn$ True. Anything else refuses by name.

func (lc *lowerCtx) animateGrantExile(ps *params) error {
	if ps.str("Defined") != "Remembered" || !ps.is("Duration", "Permanent") {
		return refuse("Animate Abilities$ outside Defined$ Remembered with Duration$ Permanent")
	}
	names := strings.Split(ps.str("Abilities"), ",")
	if len(names) != 1 || strings.TrimSpace(names[0]) == "" {
		return refuse("Animate Abilities$ %q: not one ability", ps.str("Abilities"))
	}
	ps.consume("Abilities", "Defined", "Duration")
	if left := ps.unread(); len(left) > 0 {
		return refuse("Animate Abilities$ unread params %v", left)
	}
	sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(names[0]))
	if sa == nil || sa.Kind != "AB" || sa.API != "Play" {
		return refuse("Animate Abilities$ %q is not an AB$ Play", names[0])
	}
	idx := len(lc.out.Abilities)
	lc.granted++
	lc.grantExile = true
	err := lc.withAbility(lc.ab, func() error { return lc.lowerAbility(sa) })
	lc.grantExile = false
	lc.granted--
	if err != nil {
		return err
	}
	if len(lc.out.Abilities) != idx+1 {
		return refuse("Animate Abilities$ %q lowering", names[0])
	}
	ab := lc.out.Abilities[idx]
	if ab.Kind != akActivated || ab.Zone != zExile || ab.Cost.ExileG == nil || !ab.Cost.ExileHand ||
		ab.ActCond.Kind != cYourTurn || ab.Cost.Tap || len(ab.Targets) != 0 {
		return refuse("Animate Abilities$ %q: not an exile-zone activated ability costing a hand card on your turn", names[0])
	}
	ab.Latent, ab.Name = true, "GrantedExile"
	lc.emit(lInstr{Op: "GrantExile", Obj: lRef{Kind: rRemembered}, C: idx,
		Note: "each remembered exiled card gains the ability while it stays exiled"})
	return nil
}
