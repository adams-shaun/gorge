package kgen

import "strings"

// The cost tokens costHand parses through a named helper: the body of a
// function already over the codeshape ceiling does not grow (AGENTS.md), and
// the Discard body moves here so the new arms stay inside it.

// discardCost parses Discard<N/Spec[/Description]> (CR 118.3, 601.2h).
func (lc *lowerCtx) discardCost(c *lCost, parts []string, n int, t string) error {
	// One discard part per cost: lCost carries a single N/Spec, so a
	// second token would silently overwrite the first. A spell's own
	// alternative cost takes one second part (Foil; lower_discardtwo.go).
	if c.DiscardN > 0 || c.DiscardX || c.DiscardSelf || c.DiscardF != nil {
		if lc.altCostHand {
			return lc.altDiscardTwo(c, parts, n, t)
		}
		return refuse("discard cost %q: more than one discard part", t)
	}
	if (len(parts) == 2 || len(parts) == 3) && parts[1] == "CARDNAME" && n == 1 {
		c.DiscardSelf = true // cycling (CR 702.29a): see costHand
		return nil
	}
	// Discard<N/Spec[/Description]> (gorge cost.nonManaCost): the
	// third part is the reminder text only. Card is any N cards;
	// Random and Hand are the unless-cost selection methods
	// and are refused outside one.
	// Gorge's triggered-cost window ignores the count for a Hand
	// discard (it pays the whole hand whatever the token spells;
	// the corpus writes the ignored count as 0), so Discard<0/Hand>
	// reads as Discard<1/Hand>. Every other n < 1 still refuses.
	if len(parts) == 2 && parts[1] == "Hand" && n == 0 {
		n = 1
	}
	if len(parts) < 2 || len(parts) > 3 || n < 1 {
		return refuse("discard cost %q", t)
	}
	switch parts[1] {
	case "Card":
		c.DiscardSpec = ""
	case "Random", "Hand":
		c.DiscardSpec = parts[1]
	default:
		// A filtered discard (CR 118.3, 601.2h; gorge cost
		// nonManaCost's spec over pay.DiscardCandidates'
		// MatchesSpecFrom): Solitary Cell's Card.Legendary,
		// grandeur's Card.named<self>. The filter split keeps a
		// comma inside a named<Name> argument (splitAlternatives).
		f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
		if err != nil {
			return err
		}
		c.DiscardF = f
	}
	c.DiscardN = n
	return nil
}

// discardXCost parses Discard<X/Spec[/description]> (gorge cost.nonManaCost's
// announced part; CR 601.2b, 601.2h): discard the announced X hand cards. Only
// a spell's or an activated ability's own cost; with no printed {X} the
// matching hand cards are the bound (Channeled Force). Random/Hand selection
// methods stay refused.
func (lc *lowerCtx) discardXCost(c *lCost, parts []string, t string) error {
	if !lc.lifeXSpell && !lc.actCost || len(parts) < 2 || len(parts) > 3 ||
		c.DiscardN > 0 || c.DiscardX || c.DiscardSelf || c.DiscardF != nil || c.DiscardSpec != "" || c.DiscardHand {
		return refuse("discard cost %q", t)
	}
	switch parts[1] {
	case "Card":
		c.DiscardSpec = ""
	case "Random", "Hand":
		return refuse("discard cost %q", t)
	default:
		f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
		if err != nil {
			return err
		}
		c.DiscardF = f
	}
	c.DiscardX = true
	return nil
}

// drawCost parses Draw<N/Who> (gorge cost.parse nonManaCost; paid in the
// triggered-cost or unless window by drawing). Who is the drawing player: You,
// the parent's targeted player or the triggered player (Tresserhorn's Lord,
// Academy Loremaster).
func (lc *lowerCtx) drawCost(c *lCost, parts []string, n int, t string) error {
	if len(parts) != 2 || n < 1 || c.DrawN != 0 || c.DrawWho != "" {
		return refuse("cost token %q", t)
	}
	switch parts[1] {
	case "You":
	case "Player.targetedBy", "Player.TriggeredPlayer":
		if !lc.unlessParse {
			return refuse("draw cost %q outside an unless cost", t)
		}
		c.DrawWho = parts[1]
	default:
		return refuse("draw cost %q: drawing player %q", t, parts[1])
	}
	c.DrawN = n
	return nil
}

// damageYouCost parses DamageYou<N> (gorge payUnlessDamageCost; CR 118.3): the
// resolving source deals N damage to the payer as an unless cost (Vexing
// Devil). Only there, and only a literal amount: an SVar-sized X is lowered by
// resUnlessToken (lower_costtokens.go).
func (lc *lowerCtx) damageYouCost(c *lCost, parts []string, n int, t string) error {
	if !lc.unlessParse || len(parts) != 1 || n < 1 || c.DamageYouN != 0 {
		return refuse("cost token %q", t)
	}
	c.DamageYouN = n
	return nil
}

// resTokenInstr lowers the resolution-time tokens that are payments the
// kernel performs as effects: Draw<N/Who> (a trigger's own or an unless cost;
// the drawing player may be the parent's targeted or the triggered player) and
// DamageYou<N> (the resolving source deals N damage to the payer, a PayDraw
// carrying kop.PayBSubDamage). It returns the part count payInstr tracks.
func (lc *lowerCtx) resTokenInstr(c lCost, in *lInstr, n int) (int, error) {
	switch {
	case c.DrawN > 0:
		// gorge's triggered-cost window (cumulative.go) or an UnlessCost$
		// (pay/unlessflow.go AdvanceUnless draws for the selected player)
		if !lc.trigOwnCost && !lc.unlessCost {
			return n, refuse("resolution draw cost outside a trigger's own or an unless cost")
		}
		n++
		in.CS, in.A = "Draw", cst(c.DrawN)
		if c.DrawWho != "" {
			// Draw<N/Player.targetedBy> draws for the parent's targeted
			// player, not the payer (Tresserhorn's Lord): the kernel reads
			// the recipient from the instruction's W (interp_ask.go
			// applyResolveGen)
			who := "TargetedPlayer"
			if c.DrawWho == "Player.TriggeredPlayer" {
				who = "TriggeredPlayer"
			}
			r, err := lc.defined(who)
			if err != nil {
				return n, err
			}
			in.W = r // the drawing player (cb.instr fills ki.W)
		}
	case c.DamageYouN > 0:
		// an UnlessCost$ DamageYou<N> (gorge payUnlessDamageCost; CR
		// 118.3): the resolving source deals the amount to the payer
		if !lc.unlessCost || n != 0 {
			return n, refuse("resolution damage-to-payer cost shape")
		}
		n++
		in.CS, in.A = "DamageYou", cst(c.DamageYouN)
	}
	return n, nil
}
