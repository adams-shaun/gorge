package kgen

import "strings"

// handMiracleGrant lowers a Continuous static whose AffectedZone$ is Hand and
// which grants miracle to cards in a hand (Lorehold, the Historian: "miracle
// {2}"; Molecule Man: "miracle {0}"; Aminatou, Veil Piercer: "its mana cost
// reduced by {4}", CR 702.94a). gorge reads the granted keyword off the
// card's derived keywords (rules miracle.go miracleCost, offerMiracle), at
// the draw, the reveal-trigger resolution and the cast, so the kernel reads
// the static live from the battlefield (kop.StatMiracleGrant, kw_miracle.go
// miracleCostOf): no mark is stamped on the card.
//
// The grant's parameter is a plain mana cost, or "ManaCost" / "ManaCost:<N>"
// (the card's own cost, less N generic: CardCostLess is N+1, 0 = the grant's
// own cost). A cost with an {X}, a hybrid or Phyrexian pip, a non-mana part
// or a coloured reduction, an EffectZone$ off the battlefield and anything
// beside the grant refuse by name.
func (lc *lowerCtx) handMiracleGrant(sp *params, st *lStatic, aff string) error {
	kw := strings.TrimSpace(sp.str("AddKeyword"))
	head, param, hasParam := strings.Cut(kw, ":")
	if head != "Miracle" || !hasParam || strings.Contains(kw, "&") {
		return refuse("AffectedZone Hand grant of %q", kw)
	}
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "AffectedZone", "AddKeyword", "Description", "Condition", "Secondary":
		case "EffectZone":
			if strings.TrimSpace(sp.str("EffectZone")) != "Battlefield" {
				return refuse("AffectedZone Hand miracle grant with EffectZone %q", sp.str("EffectZone"))
			}
		default:
			return refuse("AffectedZone Hand miracle grant beside %s$", k)
		}
	}
	param = strings.TrimSpace(param)
	if rest, ok := strings.CutPrefix(param, "ManaCost"); ok {
		less := 0
		if rest != "" {
			n, ok := strings.CutPrefix(rest, ":")
			mc, err := parseManaCost(n)
			if !ok || err != nil || mc.Generic == 0 || mc.Pips != [5]int{} || mc.C != 0 || mc.X || mc.XX || mc.None ||
				len(mc.Hybrid) != 0 || len(mc.Twobrid) != 0 || len(mc.Phyrexian) != 0 || mc.Generic > 250 {
				return refuse("AffectedZone Hand miracle grant cost %q", param)
			}
			less = mc.Generic
		}
		st.CardCostLess = less + 1
	} else {
		mc, err := parseManaCost(param)
		if err != nil || mc.X || mc.XX || mc.None || mc.Snow != 0 || len(mc.Hybrid) != 0 || len(mc.Twobrid) != 0 || len(mc.Phyrexian) != 0 || strings.ContainsAny(param, ":<|") {
			return refuse("AffectedZone Hand miracle grant cost %q", param)
		}
		st.Cost = lCost{Mana: mc}
	}
	f, err := lc.filter(aff)
	if err != nil {
		return err
	}
	st.F = f
	sp.consume("AddKeyword", "AffectedZone", "EffectZone")
	st.Kind = "MiracleGrant"
	st.Zone = int(zHand)
	return nil
}
