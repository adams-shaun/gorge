package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"regexp"
	"strings"
)

// payInstr encodes a resolution-time cost offer (unless/if-you-do).
// wardEvidenceRe is a ward cost of collect evidence N.
var wardEvidenceRe = regexp.MustCompile(`^CollectEvidence<([0-9]+)>$`)

// wardDiscManaRe is a ward cost of discarding a card or paying {N}
// (Forge's "Discard<1/Card>:N"; gorge beginWardPayment's ">:" arm).
var wardDiscManaRe = regexp.MustCompile(`^Discard<1/Card>:([0-9]+)$`)

func (lc *lowerCtx) payInstr(c lCost, payer lRef, subCtrOK bool) (lInstr, error) {
	in := lInstr{Obj: payer}
	if c.Forage != nil {
		// gorge (trigger_cost_forage.go): the triggered-cost window
		// pays a forage-only Cost$ of a printed trigger's own body ("you
		// may forage. If you do, ..."): PayForage, kernel act_forage.go's
		// resolution forage. A forage beside any other part, in an UnlessCost or
		// deeper in a chain stays refused.
		if !lc.trigOwnCost || !c.forageOnly() {
			return in, refuse("resolution forage cost")
		}
		in.CS, in.A, in.F = "Forage", cst(1), c.Forage
		return in, nil
	}
	if c.Waterbend != 0 {
		// an UnlessCost$ Waterbend<N> (gorge pay.chargeUnlessWaterbend; kernel
		// cost_unlesswaterbend.go): alone, with N the cost's whole generic
		m, rest := c.Mana, c
		rest.Waterbend, rest.Mana = 0, manaCost{None: true}
		if !lc.unlessCost && !lc.wardCost || m.Generic != c.Waterbend || c.Waterbend > 255 || m.Pips != [5]int{} || m.X || m.XX || m.XXX || m.C != 0 ||
			len(m.Hybrid)+len(m.Twobrid)+len(m.Phyrexian) != 0 || !rest.pureMana() {
			return in, refuse("waterbend cost shape")
		}
		in.CS, in.A = "Waterbend", cst(c.Waterbend)
		return in, nil
	}
	n := 0
	hasGraveExile := c.ExileG != nil
	if c.ExileG != nil {
		// a printed trigger's own ExileFromGrave<N/Spec> (gorge's
		// triggered-cost window: trigger_cost pay/decline, then the
		// "exile_cost" pick over the payer's graveyard, unasked with
		// exactly N candidates; Masked Vandal): PayExileGrave
		if !lc.trigOwnCost && !lc.unlessCost {
			return in, refuse("resolution exile-from-graveyard cost")
		}
		for _, cl := range c.ExileG.Clauses {
			if cl.Flags&fSelf != 0 {
				return in, refuse("resolution exile-from-graveyard cost of the source")
			}
		}
		n++
		in.CS, in.A, in.F = "ExileGrave", cst(c.ExileGN), c.ExileG
		c.ExileG = nil
	}
	if c.ExileStackSelf {
		// ExileFromStack<1/Card.Self> beside the graveyard exile (Sphinx's
		// Approach, Dragon's Approach): the resolving spell itself leaves the
		// stack as the same unless cost is paid, after the grave picks.
		if !hasGraveExile || !lc.unlessCost {
			return in, refuse("resolution exile-from-stack cost shape")
		}
		in.CS = "ExileStackSelf"
		c.ExileStackSelf = false
	}
	var err error
	if n, err = lc.resTokenInstr(&c, &in, n); err != nil { // lower_costtokenforms.go
		return in, err
	}
	if !c.Mana.None {
		n++
		if c.Mana.X && lc.trigXMax != nil {
			return lc.payManaX(c, in) // lower_triggerxcost.go
		}
		if c.Mana.X || c.Mana.C > 0 || len(c.Mana.Hybrid) > 0 && !lc.trigHybridOK(c) {
			return in, refuse("resolution mana cost %s", c.Mana)
		}
		in.CS = "Mana"
		in.A = cst(c.Mana.Generic)
		in.B = cst(packPips(c.Mana.Pips))
		in.Hyb = c.Mana.Hybrid // lower_trighybrid.go
	}
	if c.DiscardN > 0 {
		n++
		switch c.DiscardSpec {
		case "Random":
			in.CS, in.A = "DiscardRandom", cst(c.DiscardN)
		case "Hand":
			in.CS, in.A = "DiscardHand", cst(c.DiscardN)
		default:
			in.CS, in.A = "Discard", cst(c.DiscardN)
		}
	}
	if c.Sac != nil {
		n++
		in.CS, in.A, in.F = "Sacrifice", cst(c.SacN), c.Sac
	}
	if c.PayLife > 0 && in.CS == "Mana" && (lc.trigOwnCost || lc.wardCost) {
		// a printed trigger's own "pay {W}{B} and 2 life" (Zoraline; gorge
		// triggeredCostPayable's mana/life half, pay.CostPayableOther): the
		// mana pay carries the life in C (kernel F). A ward's
		// "{N} and pay L life" (Ovika; gorge AdvanceUnless charges both in
		// PayManaConv) carries it the same way (build_ab.go WardLife).
		in.C = c.PayLife
	} else if c.PayLife > 0 {
		n++
		in.CS, in.A = "PayLife", cst(c.PayLife)
	}
	if c.LifeBy != nil {
		// PayLife<X> of a trigger's own cost: the amount is the value read as
		// the cost is offered (kernel interp_ask.go execPay evaluates in.A);
		// beside any other part the count below refuses
		n++
		in.CS, in.A = "PayLife", *c.LifeBy
	}
	if c.Energy > 0 {
		n++
		in.CS, in.A = "PayEnergy", cst(c.Energy)
	}
	if c.EnergyBy != nil {
		n++
		in.CS, in.A = "PayEnergy", *c.EnergyBy
	}
	if c.Return != nil {
		// one pick: gorge settles all N at once, the kernel one per answer
		if c.ReturnN != 1 {
			return in, refuse("resolution return cost of %d", c.ReturnN)
		}
		n++
		in.CS, in.A, in.F = "Return", cst(1), c.Return
	}
	if c.SacSelf && n == 0 {
		// Sac<1/CARDNAME> alone (gorge's triggered-cost window: the one
		// candidate is the source, settled unasked): PaySacSelf
		n++
		in.CS, in.A = "SacrificeSelf", cst(1)
		c.SacSelf = false
	}
	if c.SacHost && n == 0 && lc.trigOwnCost && lc.grantAttached {
		// Sac<1/OriginalHost> as the own cost of a trigger an Equipment or Aura
		// grants (Sunfire Torch, Spare Dagger; gorge's triggered-cost window binds
		// the part to the granting host): the host is sacrificed, the one
		// candidate settled unasked (CR 701.21a)
		n++
		in.CS, in.A = "SacrificeHost", cst(1)
		c.SacHost = false
	}
	if c.BlightN > 0 {
		// Blight<N> as a trigger's own cost or an UnlessCost (gorge
		// trigger_cost_blight / unlessflow): the payer chooses a creature
		// they control (the source included) and puts N -1/-1 counters on
		// it at payment.
		if n != 0 {
			return in, refuse("resolution blight cost shape")
		}
		in.CS, in.A = "Blight", cst(c.BlightN)
		in.F = &lFilter{Clauses: []lClause{{Types: tCreature, Ctrl: whoYou}}}
		c.BlightN = 0
		n++
	}
	if c.EvidenceN > 0 {
		// CollectEvidence<N> as a printed trigger's own cost (gorge's
		// triggered-cost window: cumulative.go payability, then
		// evidence_cost.go evidenceStage): PayEvidence, kernel
		// cost_collectevidence.go. Alone, and only there: an UnlessCost
		// pays through unlessflow, not modelled.
		if !lc.trigOwnCost || n != 0 {
			return in, refuse("resolution evidence cost shape")
		}
		in.CS, in.A = "Evidence", cst(c.EvidenceN)
		if c.ExileTrigNew {
			// Lamplight Phoenix's ExileAnyGrave<1/Card.TriggeredNewCard>
			// CollectEvidence<N>: gorge reserves the triggered card's exile
			// first (cumulative.go evidenceReserveOrder), then the evidence
			// stage runs over the rest of the graveyard
			// (cost_collectevidence.go)
			if !lc.selfDiesTrigger() {
				return in, refuse("resolution ExileAnyGrave cost shape")
			}
			in.CS = "ExileTrigNewEvidence"
			c.ExileTrigNew = false
		}
		c.EvidenceN = 0
		n++
	}
	if c.EvidenceX {
		// CollectEvidence<X> as a printed trigger's own cost (Incinerator of
		// the Guilty): gorge's triggeredCostXAsk announces X over 0..the
		// graveyard's total mana value (capped at 64), then the ordinary
		// pay election and the evidence stage; the body's Count$xPaid is X
		// (kernel cost_collectevidencex.go). Alone, and only there.
		if !lc.trigOwnCost || n != 0 || !c.Mana.None {
			return in, refuse("resolution evidence X cost shape")
		}
		if x, ok := lc.face.SVars["X"]; !ok || strings.TrimSpace(x) != "Count$xPaid" {
			return in, refuse("trigger CollectEvidence<X> cost with SVar:X other than Count$xPaid")
		}
		in.CS, in.A = "EvidenceX", cst(trigXFoldCap)
		lc.trigXPaid = true
		c.EvidenceX = false
		n++
	}
	if c.SubAmongN > 0 {
		// RemoveAnyCounter<N/Any/CARDNAME> as a trigger's own cost: the
		// source's counters, any kind (gorge trigger_cost_subcounter.go's
		// subCounterKindAsk). A filtered among-part is a different payment.
		if c.SubAmongF != nil || n != 0 {
			return in, refuse("resolution RemoveAnyCounter cost shape")
		}
		var ck int
		if strings.EqualFold(c.SubAmongKnd, "Any") {
			ck = int(kop.CtrAll)
		} else {
			k, err := counterKind(c.SubAmongKnd)
			if err != nil {
				return in, refuse("resolution RemoveAnyCounter cost of kind %q", c.SubAmongKnd)
			}
			ck = int(k)
		}
		in.CS, in.A, in.B = "SubCounter", cst(c.SubAmongN), cst(ck)
		c.SubAmongN = 0
		n++
	}
	if c.SubCtr > 0 {
		// B: a fixed SubCounter<N/Kind> drained from the resolving
		// ability's own source (gorge trigger_cost_subcounter.go:31, the
		// trigger's own cost; unless_cost.go:47, an UnlessCost$). The cost
		// parser admits only the source form and rejects X/announced and
		// named targets; a wildcard "Any" kind is CtrAll.
		if !subCtrOK || n != 0 {
			return in, refuse("resolution SubCounter cost shape")
		}
		var ck int
		if strings.EqualFold(c.SubCtrKnd, "Any") {
			ck = int(kop.CtrAll)
		} else {
			k, err := counterKind(c.SubCtrKnd)
			if err != nil {
				return in, refuse("resolution SubCounter cost of kind %q", c.SubCtrKnd)
			}
			ck = int(k)
		}
		in.CS, in.A, in.B = "SubCounter", cst(c.SubCtr), cst(ck)
		c.SubCtr = 0
		n++
	}
	if c.TapXVar {
		// tapXType<X/Spec> as a printed trigger's own whole cost (lower_tapxvar.go)
		if !lc.trigOwnCost || n != 0 || c.TapF == nil {
			return in, refuse("resolution tapXType<X> cost shape")
		}
		in.CS, in.A, in.F = "TapXVar", cst(0), c.TapF
		lc.trigXPaid = true
		c.TapXVar = false
		n++
	}
	if c.TapN > 0 {
		// C: a literal tapXType<N/Spec> that is the WHOLE cost of a
		// printed trigger (gorge cumulative.go:1130-1137 and
		// triggered_literal_tap.go: one KChoose, Min=Max=N, over
		// TapCostCandidates; the tap election is the payment). As an
		// UnlessCost$ (gorge, unlessflow.go
		// AdvanceUnless's UnlessKindTap part) the pay / decline ask comes
		// first, then the pick, unasked with exactly N candidates.
		if !lc.trigOwnCost && !lc.unlessCost || n != 0 || c.TapF == nil {
			return in, refuse("resolution tapXType cost shape")
		}
		in.CS, in.A, in.F = "TapX", cst(c.TapN), c.TapF
		c.TapN = 0
		n++
	}
	if c.ExileTrigNew {
		// ExileAnyGrave<1/Card.TriggeredNewCard> as a printed dies
		// trigger's own cost (gorge's triggered-cost
		// window: the pay/decline ask, then the triggered card is exiled
		// unasked — one candidate, no pick). Alone, and only there: a cost
		// with another part (Lamplight Phoenix's evidence, Kinzu's life),
		// an unless cost (Flitting Guerrilla) or a non-dies trigger keeps
		// refusing.
		if !lc.trigOwnCost || n != 0 || !lc.selfDiesTrigger() {
			return in, refuse("resolution ExileAnyGrave cost shape")
		}
		in.CS, in.A = "ExileTrigNew", cst(1)
		c.ExileTrigNew = false
		n++
	}
	if c.ExileHandN > 0 {
		// ExileFromHand<N/Card> as a printed trigger's own cost (gorge's
		// triggered-cost window: the pay/decline ask, then the "exile_cost"
		// pick over the hand, unasked with exactly N cards): PayExileGrave
		// with the hand sentinel. Alone, and only there.
		if !lc.trigOwnCost || lc.unlessCost || n != 0 {
			return in, refuse("resolution exile-from-hand cost shape")
		}
		in.CS, in.A = "ExileHand", cst(c.ExileHandN)
		c.ExileHandN = 0
		n++
	}
	if c.BeholdN > 0 {
		// Behold<1/Type> as a resolution cost (CR 701.4a): an UnlessCost$ or
		// a printed trigger's own cost. gorge's unless and triggered-cost
		// flows pose the pay election, then the "beholdcost" pick over the
		// payer's matching permanents then hand cards, unasked with exactly
		// the count; nothing moves (kernel cost_unlessbehold.go). Alone, one.
		if !lc.trigOwnCost && !lc.unlessCost || n != 0 || c.BeholdN != 1 || c.BeholdF == nil {
			return in, refuse("resolution behold cost shape")
		}
		in.CS, in.A, in.F = "Behold", cst(1), c.BeholdF
		c.BeholdN, c.BeholdF = 0, nil
		n++
	}
	if c.AddCtr > 0 {
		// An UnlessCost$ AddCounter<N/KIND/recipient> (gorge
		// pay/costparts.go PayManaSourceParts' unless placement; kernel
		// cost_unlessaddcounter.go): alone, with the recipient the source
		// (CARDNAME/NICKNAME) or the parent's target. A two-field token
		// stays an activation cost.
		if !lc.unlessCost || c.AddCtrRef.Kind == rNone || n != 0 {
			return in, refuse("resolution AddCounter cost shape")
		}
		ck, err := counterKind(c.AddCtrKnd)
		if err != nil {
			return in, refuse("resolution AddCounter cost of kind %q", c.AddCtrKnd)
		}
		in.CS, in.A, in.B, in.Obj2 = "AddCounter", cst(c.AddCtr), cst(int(ck)), c.AddCtrRef
		c.AddCtr, c.AddCtrRef, c.AddCtrKnd = 0, lRef{}, ""
		n++
	}
	if !c.resolutionCostShape(n) {
		return in, refuse("resolution cost shape")
	}
	return in, nil
}

// forageOnly reports whether a forage is the cost's only payment part. The
// Forage arm returns before the final one-part shape check below, so it needs
// its own complete gate: pureMana's field list covers every other part once
// the mana cost is checked here and the two fields it does not list
// (Waterbend, RevealSelf) are excluded.
func (c lCost) forageOnly() bool {
	if !c.Mana.None || c.RevealSelf || c.Waterbend != 0 || c.WaterbendX {
		return false
	}
	rest := c
	rest.Forage = nil
	return rest.pureMana()
}

// resolutionCostShape is payInstr's final one-part check: exactly one payment
// part was lowered and no part is left unconsumed. resTokenInstr clears the
// Mill/Reveal fields only when its own first-match arm consumes them, so an
// unconsumed part refuses here instead of being silently dropped.
func (c lCost) resolutionCostShape(n int) bool {
	return n == 1 && !c.Tap && !c.SacSelf && c.Loyalty == 0 && c.SubCtr == 0 && c.SubAmongN == 0 && c.TapN == 0 && !c.TapHost && !c.SacHost && c.BlightN == 0 && c.EvidenceN == 0 && !c.EvidenceX && c.AddCtr == 0 && c.AddCtrRef.Kind == rNone && c.MillN == 0 && c.RevealN == 0 && c.RevealF == nil
}

// packPips packs WUBRG pip counts 3 bits each.
func packPips(p [5]int) int {
	v := 0
	for i := 4; i >= 0; i-- {
		v = v<<3 | p[i]
	}
	return v
}
