package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
)

// Reveal<1/Spec> as a cast-route cost: the first part of an
// AlternateAdditionalCost (Daring Buccaneer's Reveal<1/Pirate>:2) and the
// turn-up cost of a Morph family keyword (Dragon's Eye Savants: reveal a blue
// card) reveal one hand card matching the spec (gorge cast_asks.go revealCostAsk, morph_turnup.go turnUpRevealAsk; CR
// 701.20a). The kernel poses gorge's "revealcost" pick for it, and withholds the
// offer without a candidate (dirac/act_revealpick.go; CardDef.RevealN/RevealF).
// Nothing moves, so the part is table data, not a cost part. N is 1; a
// whole-hand, CARDNAME or SameColor spec is the cast window's relational
// shape and stays refused, as does a card that prints both routes.

// revealHandSpec parses the spec of a Reveal<1/Spec[/description]> cast-route
// part (parts is the cost token's "/" split) into the filter of its hand
// candidates.
func (lc *lowerCtx) revealHandSpec(c *lCost, parts []string, n int, t string) error {
	if len(parts) < 2 || len(parts) > 3 || n != 1 || c.RevealN != 0 || c.RevealSelf ||
		parts[1] == "CARDNAME" || costvocab.IsWholeHandRevealSpec(parts[1]) || strings.Contains(parts[1], "ChosenType") || costvocab.IsSameColorRevealSpec(parts[1]) {
		return refuse("reveal cost %q", t)
	}
	f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
	if err != nil {
		return err
	}
	c.RevealN, c.RevealF = n, f
	return nil
}

// altAddFirstPart lowers the first part of an AlternateAdditionalCost: the
// only cost that may carry a Behold<1/Type> (kernel act_behold.go) or a
// Reveal<1/Spec> (kernel act_revealpick.go), alone beside no other part.
func (lc *lowerCtx) altAddFirstPart(a string) (lCost, error) {
	lc.beholdOK, lc.revealOK = true, true
	c, err := lc.cost(a)
	lc.beholdOK, lc.revealOK = false, false
	if err == nil && c.RevealN != 0 {
		rest := c
		rest.RevealN, rest.RevealF = 0, nil
		if lc.out.BeholdExile || !rest.pureMana() || !c.Mana.None {
			err = refuse("reveal cost %q", a)
		}
	}
	return c, err
}

// morphCostParse parses a Morph/Megamorph/Disguise turn-face-up cost: a mana
// cost or a life payment alone (morphTurnUpCost), or Reveal<1/Spec> alone, whose
// mana is {0} and whose reveal rides CardDef.RevealN/RevealF.
func (lc *lowerCtx) morphCostParse(cs string) (mc manaCost, life uint8, rev *lFilter, err error) {
	arg, ok := strings.CutPrefix(cs, "Reveal<")
	if !ok {
		mc, life, err = morphTurnUpCost(cs)
		return mc, life, nil, err
	}
	arg, ok = strings.CutSuffix(arg, ">")
	var c lCost
	if !ok {
		return mc, 0, nil, refuse("morph cost %q", cs)
	}
	if n := strings.Count(arg, "/"); n < 1 || n > 2 || !strings.HasPrefix(arg, "1/") {
		return mc, 0, nil, refuse("morph cost %q", cs)
	}
	if err = lc.revealHandSpec(&c, strings.Split(arg, "/"), 1, cs); err != nil {
		return mc, 0, nil, err
	}
	return mc, 0, c.RevealF, nil
}

// spellPartCost builds the either-or additional cost's first part, its
// reveal (if any) moved to d's table fields.
func (cb *cardBuilder) spellPartCost(d *kop.CardDef, c lCost) (kop.Cost, error) {
	if c.RevealN != 0 {
		f, err := cb.filter(c.RevealF, kop.ZoneHand)
		if err != nil {
			return kop.Cost{}, err
		}
		d.RevealN, d.RevealF = uint8(c.RevealN), f
		c.RevealN, c.RevealF = 0, nil
	}
	return cb.cost(c)
}

// morphCostOf is the table form of the card's turn-up mana cost, with the
// turn-up's reveal (K:Morph:Reveal<1/Spec>) moved to d's table fields.
func (cb *cardBuilder) morphCostOf(d *kop.CardDef, c *lCard) (kop.ManaCost, error) {
	if c.MorphReveal != nil {
		if d.RevealN != 0 {
			return kop.ManaCost{}, refuse("a reveal on both the either-or additional cost and the turn-up")
		}
		f, err := cb.filter(c.MorphReveal, kop.ZoneHand)
		if err != nil {
			return kop.ManaCost{}, err
		}
		d.RevealN, d.RevealF = 1, f
	}
	return manaCostOf(*c.MorphCost)
}
