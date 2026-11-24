package kgen

import "strings"

// War Room: "{3}, {T}, Pay life equal to the number of colors in your
// commanders' color identity: Draw a card. Activate only in a game of
// Commander." The cost is PayLife<X> with SVar X Count$ColorsColorIdentity
// and the format gate is ActivationGameTypes$; the card is the corpus's only
// carrier of either.
//
// gorge resolves the SVar wherever the cost is priced or paid
// (pay.FixLifeXCost through the Host's CommanderIdentityColourCount) and folds
// it into a fixed life part, so nothing is announced: the cost is a fixed
// PayLife<N> whose N belongs to the payer. One table serves every seat, so
// the count cannot be a table constant; it lowers to kop.Cost.LifeIdent and
// the kernel reads the payer's commanders as it prices and pays
// (dirac/cost_lifeident.go).

// lifeIdentCost lowers PayLife<X> whose SVar X is x into c.LifeIdent, and
// reports whether it did: only an activated ability's own cost, beside no
// other life or announced-X part.
func (lc *lowerCtx) lifeIdentCost(c *lCost, x string) bool {
	if !lc.actCost || strings.TrimSpace(x) != "Count$ColorsColorIdentity" ||
		c.PayLife != 0 || c.LifeX || c.LifeBy != nil || c.LoyaltyX || c.LifeIdent {
		return false
	}
	c.LifeIdent = true
	return true
}

// lifeIdentGate reads ActivationGameTypes$ and holds LifeIdent to the shape
// the kernel prices: a battlefield activated ability. gorge models two
// formats, Commander and Constructed, and only the Commander token names one
// (rules.activationGameTypesOK), so a list naming Commander is a Commander-only
// ability and a list without it is an ability no modelled game offers.
func lifeIdentGate(ab *lAbility, ps *params, cost lCost) error {
	if raw, ok := ps.get("ActivationGameTypes"); ok {
		commander := false
		for _, tok := range strings.Split(raw, ",") {
			commander = commander || strings.TrimSpace(tok) == "Commander"
		}
		if !commander {
			return refuse("ActivationGameTypes$ %q names no modelled format", raw)
		}
		if ab.Kind != akActivated || ab.Zone != zNone && ab.Zone != zBattlefield {
			return refuse("ActivationGameTypes$ outside a battlefield activated ability")
		}
		ab.CmdOnly = true
	}
	if cost.LifeIdent && (ab.Kind != akActivated || ab.Zone != zNone && ab.Zone != zBattlefield) {
		return refuse("commander colour identity life cost outside a battlefield activated ability")
	}
	return nil
}
