package kgen

import (
	"strconv"
	"strings"
)

// The attack tax: a printed S:Mode$ CantAttackUnless whose Cost$ is generic
// mana (Propaganda, Ghostly Prison, Windborn Muse, Baird, Steward of Argive,
// Sphere of Safety, Onakke Oathkeeper; CR 508.1d, 508.1g-i). gorge prices
// each (attacker, defender) pair in rules/attack_cost.go attackPairCharge:
// a static in force whose ValidCard$ admits the attacking creature and whose
// Target$ admits the defender charges its Cost$ (a literal, an inline Count$
// or an SVar of the face, read from the static's source and controller), the
// matching statics summing. The kernel's half is dirac/combat_attacktax.go
// (kop.StatAttackTax). The other Cost$ shapes gorge's chargeFromCost reads
// (Sac<>, Return<>, tapXType<>, PayLife<>, a Phyrexian pip: Norn's Annex) and
// RememberingAttacker$ (Nils, Discipline Enforcer) stay refused.

// attackTaxKeys are the parameters gorge reads on a CantAttackUnless static
// (effects.CantAttackUnlessRestrictionParamsReadable): one carrying any other
// is skipped there, not enforced, so a lowering that taxed it would charge
// what gorge lets attack free.
var attackTaxKeys = map[string]bool{
	"Mode": true, "ValidCard": true, "Target": true, "Cost": true, "Description": true,
	"TriggerDescription": true, "Secondary": true, "Attacker": true, "IsPresent": true,
	"IsPresent2": true, "CheckSVar": true, "SVarCompare": true, "Condition": true,
	"RememberingAttacker": true,
}

// attackTaxReadable refuses a CantAttackUnless static gorge does not read. It
// runs on the static as printed, before lowerStatic drops a spelled-out
// EffectZone$ Battlefield, which gorge's list does not admit either.
func attackTaxReadable(params map[string]string) error {
	for _, k := range sortedParamKeys(params) {
		if !attackTaxKeys[k] {
			return refuse("CantAttackUnless %s$: gorge skips a static it cannot read (effects.CantAttackUnlessRestrictionParamsReadable)", k)
		}
	}
	return nil
}

// cantAttackUnless lowers the static to kop.StatAttackTax: F the attackers
// (ValidCard$, matched from the static's source and controller), Amount the
// charge, Flags its Target$ scope ("TaxPlayer" You, "TaxWalker"
// Planeswalker.YouCtrl, neither every defender). staticBody has read the
// gate (IsPresent$, Condition$, CheckSVar$) into st.Cond, gorge's
// continuousGateHolds.
func (lc *lowerCtx) cantAttackUnless(sp *params, st *lStatic) error {
	if err := attackTaxReadable(sp.m); err != nil {
		return err
	}
	if lc.granted > 0 {
		return refuse("CantAttackUnless granted by another static")
	}
	vc, ok := sp.get("ValidCard")
	if !ok {
		return refuse("CantAttackUnless without ValidCard$") // gorge attackPairCharge skips it
	}
	if v, ok := sp.get("RememberingAttacker"); ok {
		return refuse("CantAttackUnless RememberingAttacker$ %q: a charge read off the attacking creature is not lowered", v)
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	if f.Players != 0 {
		return refuse("CantAttackUnless ValidCard$ %q with a player filter", vc)
	}
	st.F = f
	if t, ok := sp.get("Target"); ok {
		for _, part := range strings.Split(t, ",") {
			switch strings.TrimSpace(part) {
			case "You":
				st.Flags = append(st.Flags, "TaxPlayer")
			case "Planeswalker.YouCtrl":
				st.Flags = append(st.Flags, "TaxWalker")
			default:
				return refuse("CantAttackUnless Target$ %q", t)
			}
		}
	}
	// gorge attackUnlessCharge: a literal integer, then an inline Count$, then
	// an SVar of the face; anything else is the cost-token grammar
	raw := sp.str("Cost")
	_, svar := lc.face.SVars[raw]
	switch n, err := strconv.Atoi(raw); {
	case raw == "":
		return refuse("CantAttackUnless without Cost$")
	case err == nil:
		if n < 0 {
			return refuse("CantAttackUnless Cost$ %q", raw) // gorge: unpriceable
		}
		st.Amount = cst(n)
	case strings.HasPrefix(raw, "Count$") || svar:
		if st.Amount, err = lc.value(raw); err != nil {
			return err
		}
	default:
		return refuse("CantAttackUnless Cost$ %q: a non-mana, coloured or Phyrexian attack cost is not lowered", raw)
	}
	sp.consume("Secondary", "TriggerDescription")
	lc.twoSeat("an attack tax: its lockstep plays two seats")
	return nil
}
