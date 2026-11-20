package kgen

import (
	"strings"

	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
)

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
	if (len(parts) == 2 || len(parts) == 3) && (parts[1] == "CARDNAME" || parts[1] == "NICKNAME") && n == 1 {
		// NICKNAME is Forge's self-reference spelling of CARDNAME; gorge's
		// pay.costparts reads both as the source (channel)
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
	if len(parts) != 2 || n < 1 || c.DrawN != 0 || c.DrawBy != nil || c.DrawWho != "" {
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

// drawXCost parses Draw<X/You> as a printed trigger's own cost (Champion of
// Wits: "you may draw cards equal to its power. If you do, ..."). X is the
// face's SVar:X, read as the cost is paid, not an announced amount; the
// unless-cost form of Draw<X/You> already lowers through resUnlessToken
// (lower_costtokens.go). Only the payer draws.
func (lc *lowerCtx) drawXCost(c *lCost, parts []string, t string) error {
	if !lc.trigBodyCost || len(parts) != 2 || parts[1] != "You" ||
		c.DrawN != 0 || c.DrawBy != nil || c.DrawWho != "" {
		return refuse("cost token %q", t)
	}
	if _, ok := lc.face.SVars["X"]; !ok {
		return refuse("cost token %q without SVar:X", t)
	}
	v, err := lc.value("X")
	if err != nil {
		return err
	}
	c.DrawBy = &v
	return nil
}

// namedCostToken lowers the cost tokens with their own head helper before
// costHand's generic N/Spec dispatch: Discard<X/Spec> and Draw<X/Who> (an X
// count) and RevealChosen<...> (no payment; an offer gate). handled is false
// for every other token, so costHand's dispatch falls through. It is a helper
// so costHand stays inside its codeshape rows.
func (lc *lowerCtx) namedCostToken(c *lCost, head string, parts []string, t string) (handled bool, err error) {
	switch {
	case head == "Discard" && parts[0] == "X":
		return true, lc.discardXCost(c, parts, t)
	case head == "Draw" && parts[0] == "X":
		return true, lc.drawXCost(c, parts, t)
	case head == "RevealChosen":
		return true, lc.revealChosenCost(c, parts, t)
	}
	return false, nil
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

// millCost parses Mill<N> (CR 701.17b, 118.3): the payer mills N cards from
// the top of their own library as the cost is paid (gorge cost.parse's Mill
// part, pay.PayMillCost, cast_commit.go's payMillCostParts; kop.Cost.Mill).
// An activated ability's own cost lowers it into the Cost.Mill byte, an
// UnlessCost$ into payInstr's CS=Mill instruction (gorge payUnlessCost mills
// through the shared pay.PayMillCost behind the election, gated
// pay.MillCostPayable; Deep Spawn), and a mana ability's Mill<N> into the
// Cost.Mill byte beside its {T} (Millikin): the head's Cost$ is parsed while
// the ability is still classified akMana -- the reclassification reads the
// parsed body (lower_stackmana.go stackManaAbility) -- and no CR 605.1a mana
// ability's cost moves a library card (cards.IsManaAbilitySA's library gate),
// so gorge resolves the whole ability on the stack as an ordinary activated
// ability and its activation pays the mill through the same Cost.Mill arm
// every activated ability pays (mana_window.go payAbilityCosts). A trigger's
// own Cost$ Mill (Ashcoat of the Shadow Swarm) parses here — costUnless — but
// payInstr refuses it: gorge's triggered-cost window keeps the component
// decline-only (cumulative.go triggeredCostComponentsPayable).
func (lc *lowerCtx) millCost(c *lCost, parts []string, n int, t string) error {
	if !lc.actCost && !lc.unlessParse && (lc.ab == nil || lc.ab.Kind != akMana) ||
		len(parts) != 1 || n < 1 || c.MillN != 0 {
		return refuse("mill cost %q", t)
	}
	c.MillN = n
	return nil
}

// revealSelfCost parses Reveal<1/CARDNAME> (the forecast family's activated
// cost, CR 702.54a; gorge cast_asks.go revealCostAsk): the cost reveals one
// card named like the source from its controller's hand and nothing moves
// (kop.Cost.RevealSelf; the candidates are every hand card of that name,
// gorge pay.CostCandidates' CARDNAME spec). Only there, and only the source
// spec: a cast's reveal candidates exclude the spell on the stack, and a
// filtered, whole-hand or announced-X reveal needs a pick or announcement
// the activation window does not pose yet (Living Destiny, Sasaya,
// Illuminated Folio, the Martyr cycle).
//
// An UnlessCost$ Reveal<N/Spec> lowers as a filtered hand reveal instead
// (Priest of the Wakening Sun's Reveal<1/Creature.Dinosaur>, Xyru Specter's
// Reveal<2/Card.Black>): gorge beginUnlessPayment's "revealcost" pick over the
// payer's matching hand cards, the settled picks announced as one Note and
// nothing moved (pay/unlessflow.go AdvanceUnless). A CARDNAME spec matches the
// source object alone (CR 201.5, engine/effects/filter.go matchesSpecFrom), so
// a battlefield or stack source has no hand candidate — gorge's offer gate
// keeps the election decline-only — and a whole-hand (IsWholeHandRevealSpec)
// or SameColor (Illuminated Folio) spec is the cast window's relational shape.
// A trigger's own Cost$ Reveal parses here too and payInstr refuses it: gorge's
// triggered-cost window keeps the component decline-only (cumulative.go
// triggeredCostComponentsPayable).
func (lc *lowerCtx) revealSelfCost(c *lCost, parts []string, n int, t string) error {
	if lc.unlessParse {
		// One card: gorge's unlessflow asks its N-of-M "revealcost" pick as
		// ONE KChoose of N selects while the kernel's pay pick asks one card
		// per answer, so N > 1 would diverge at the decision record (Xyru
		// Specter's Reveal<2/Card.Black> stays out for that and its
		// GenericChoice blocker alike).
		if len(parts) != 2 || n != 1 || c.RevealN != 0 || c.RevealSelf ||
			parts[1] == "CARDNAME" || parts[1] == "Hand" || costvocab.IsSameColorRevealSpec(parts[1]) {
			return refuse("reveal cost %q", t)
		}
		f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
		if err != nil {
			return err
		}
		c.RevealN, c.RevealF = n, f
		return nil
	}
	if !lc.actCost || n != 1 || len(parts) != 2 || parts[1] != "CARDNAME" || c.RevealSelf {
		return refuse("reveal cost %q", t)
	}
	c.RevealSelf = true
	return nil
}

// forecastParam reads a forecast ability's Forecast$ param: gorge reads
// nothing of it (legal_activation.go's abilityZoneOK: an AB line without
// ActivationZone$ activates from the battlefield alone, so a hand card's
// forecast ability is never offered, and the upkeep-only once-each-turn
// window CR 702.54a asks for is unmodelled). The corpus's forecast family
// lowers without it, and its Reveal<1/CARDNAME> cost keeps the ability
// unpayable off the hand on both engines (pay.NonManaCastableP's reveal
// candidates, canActivate's gate).
func (lc *lowerCtx) forecastParam(ps *params) {
	ps.consume("Forecast")
}

// drawRecipient is the Ref a Draw<N/Who> cost's cards go to. Draw<N/Player
// .targetedBy> draws for the parent's targeted player, not the payer
// (Tresserhorn's Lord): the kernel reads the recipient from the instruction's
// W (interp_ask.go applyResolveGen); "" (Draw<N/You>) is rNone, the payer.
func (lc *lowerCtx) drawRecipient(who string) (lRef, error) {
	switch who {
	case "":
		return lRef{}, nil
	case "Player.TriggeredPlayer":
		return lc.defined("TriggeredPlayer")
	}
	return lc.defined("TargetedPlayer")
}

// resTokenInstr lowers the resolution-time tokens that are payments the
// kernel performs as effects: Draw<N/Who> (a trigger's own or an unless cost;
// the drawing player may be the parent's targeted or the triggered player),
// DamageYou<N> (the resolving source deals N damage to the payer, a PayDraw
// carrying kop.PayBSubDamage), the UnlessCost$ Mill<N> (a PayDraw carrying
// kop.PayBSubMill) and the UnlessCost$ Reveal<N/Spec> (a PayDiscard carrying
// kop.PayBReveal). It returns the part count payInstr tracks and clears only
// the Mill/Reveal fields its own arm consumes: the switch is first-match, so a
// Draw or DamageYou arm leaves a following Mill/Reveal part for payInstr's
// final one-part shape check to refuse instead of silently dropping it.
func (lc *lowerCtx) resTokenInstr(c *lCost, in *lInstr, n int) (int, error) {
	switch {
	case c.DrawN > 0 || c.DrawBy != nil:
		// gorge's triggered-cost window (cumulative.go) or an UnlessCost$
		// (pay/unlessflow.go AdvanceUnless draws for the selected player)
		if !lc.trigOwnCost && !lc.unlessCost {
			return n, refuse("resolution draw cost outside a trigger's own or an unless cost")
		}
		n++
		in.CS = "Draw"
		if c.DrawBy != nil {
			in.A = *c.DrawBy // Draw<X/You>: the face's SVar:X read as the cost is paid
		} else {
			in.A = cst(c.DrawN)
		}
		r, err := lc.drawRecipient(c.DrawWho)
		if err != nil {
			return n, err
		}
		in.W = r // the drawing player (cb.instr fills ki.W)
	case c.DamageYouN > 0:
		// an UnlessCost$ DamageYou<N> (gorge payUnlessDamageCost; CR
		// 118.3): the resolving source deals the amount to the payer
		if !lc.unlessCost || n != 0 {
			return n, refuse("resolution damage-to-payer cost shape")
		}
		n++
		in.CS, in.A = "DamageYou", cst(c.DamageYouN)
	case c.MillN > 0:
		// an UnlessCost$ Mill<N> (gorge payUnlessCost: the shared
		// pay.PayMillCost behind the election, gated pay.MillCostPayable;
		// Deep Spawn). Alone: the instruction model pays one kind. A
		// trigger's own Cost$ Mill stays refused — gorge's triggered-cost
		// window keeps the component decline-only (cumulative.go
		// triggeredCostComponentsPayable).
		if !lc.unlessCost || n != 0 {
			return n, refuse("resolution mill cost shape")
		}
		n++
		in.CS, in.A = "Mill", cst(c.MillN)
		c.MillN = 0
	case c.RevealN > 0:
		// an UnlessCost$ Reveal<N/Spec> (gorge beginUnlessPayment's
		// "revealcost" pick, the settled picks one Note, nothing moved;
		// Priest of the Wakening Sun). A trigger's own Cost$ Reveal stays
		// refused, the same triggered-cost-window reading as the Mill arm.
		if !lc.unlessCost || n != 0 {
			return n, refuse("resolution reveal cost shape")
		}
		n++
		in.CS, in.A, in.F = "Reveal", cst(c.RevealN), c.RevealF
		c.RevealN, c.RevealF = 0, nil
	}
	return n, nil
}
