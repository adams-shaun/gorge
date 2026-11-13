package kgen

import (
	"strconv"
	"strings"
)

// The block tax: a printed S:Mode$ CantBlockUnless whose Cost$ is generic
// mana (Archangel of Tithes, Qal Sisma Behemoth, Myr Prototype, Hipparion,
// Awesome Presence, Cowed by Wisdom, Oppressive Rays; CR 509.1c, 509.1d).
// gorge prices each (blocker, attacker) pair in rules/attack_cost.go
// blockPairCharge: a static in force whose ValidCard$ admits the blocker and
// whose Attacker$ admits the attacker (an absent spec is unconditional on
// that half) charges its Cost$ (a literal, an inline Count$ or an SVar of
// the face, read from the static's source and controller), the matching
// statics summing. The kernel's half is dirac/combat_blocktax.go
// (kop.StatBlockTax). The other Cost$ shapes gorge's chargeFromCost reads
// (PayLife<>, tapXType<>, Sac<>, Return<>, a Phyrexian pip) stay refused, as
// do the Effect/Animate-delivered statics (War Cadence, Whipgrass Entangler).

// blockTaxKeys are the parameters gorge reads on a CantBlockUnless static
// (effects cantBlockUnlessRestrictionParamsReadableKeys): one carrying any
// other is skipped there, not enforced.
var blockTaxKeys = map[string]bool{
	"Mode": true, "ValidCard": true, "Attacker": true, "Cost": true, "Description": true,
	"Secondary": true, "IsPresent": true, "IsPresent2": true, "CheckSVar": true,
	"SVarCompare": true, "Condition": true,
}

// blockTaxReadable refuses a CantBlockUnless static gorge does not read. It
// runs on the static as printed, before lowerStatic drops a spelled-out
// EffectZone$ Battlefield, which gorge's list does not admit either.
func blockTaxReadable(params map[string]string) error {
	for _, k := range sortedParamKeys(params) {
		if !blockTaxKeys[k] {
			return refuse("CantBlockUnless %s$: gorge skips a static it cannot read (effects.cantBlockUnlessRestrictionParamsReadableKeys)", k)
		}
	}
	return nil
}

// cantBlockUnless lowers the static to kop.StatBlockTax: F the blockers
// (ValidCard$; absent reads as Creature, gorge's unconditional blocker
// half), F2 the attackers (Attacker$; absent stays nil, unconditional),
// Amount the charge. staticBody has read the gate (IsPresent$, Condition$,
// CheckSVar$) into st.Cond, gorge's continuousGateHolds.
func (lc *lowerCtx) cantBlockUnless(sp *params, st *lStatic) error {
	if err := blockTaxReadable(sp.m); err != nil {
		return err
	}
	if lc.granted > 0 {
		return refuse("CantBlockUnless granted by another static")
	}
	vc, ok := sp.get("ValidCard")
	if !ok {
		vc = "Creature"
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	if f.Players != 0 {
		return refuse("CantBlockUnless ValidCard$ %q with a player filter", vc)
	}
	st.F = f
	if av, ok := sp.get("Attacker"); ok {
		f2, err := lc.filter(av)
		if err != nil {
			return err
		}
		if f2.Players != 0 {
			return refuse("CantBlockUnless Attacker$ %q with a player filter", av)
		}
		st.F2 = f2
	}
	// gorge blockUnlessCharge: a literal integer, then an inline Count$, then
	// an SVar of the face; anything else is the cost-token grammar
	raw := sp.str("Cost")
	_, svar := lc.face.SVars[raw]
	switch n, err := strconv.Atoi(raw); {
	case raw == "":
		return refuse("CantBlockUnless without Cost$")
	case err == nil:
		if n < 0 {
			return refuse("CantBlockUnless Cost$ %q", raw) // gorge: unpriceable
		}
		st.Amount = cst(n)
	case strings.HasPrefix(raw, "Count$") || svar:
		if st.Amount, err = lc.value(raw); err != nil {
			return err
		}
	default:
		return refuse("CantBlockUnless Cost$ %q: a non-mana, coloured or Phyrexian block cost is not lowered", raw)
	}
	sp.consume("Secondary")
	lc.twoSeat("a block tax: its lockstep plays two seats")
	return nil
}
