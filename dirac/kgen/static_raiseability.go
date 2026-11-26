package kgen

import "strings"

// raiseAbility lowers a RaiseCost static of activated abilities (Type$
// Ability; CR 601.2f, 602.2b) into kop.StatRaiseAbility (kernel
// cost_abraise.go abRaised): activated abilities of battlefield permanents
// matching ValidCard$ cost Amount$ generic more (a literal, an inline Count$ or
// an SVar of the face; a Cost$ of pure generic mana is the same raise), or pay
// the Cost$ part the activation owes at its cost stage: Sac<1/<filter>> (Brutal
// Suppression's "Sacrifice a land", kop.RaiseAbPick, the pick's permanents in
// F2) or PayLife<N> (kop.StRaiseLife, the life rides the build's life
// payment). gorge composes every matching static's raise onto the total before
// the reductions (pay.ApplyTo); the kernel composes the generic and life into
// the priced cost the same way and poses the pick as the entry's pick cost
// (buildPickCost). The static's AffectedZone$ must be Battlefield (the kernel
// prices battlefield sources only, as abReduced does); EffectZone$ All beside
// ValidCard$ Card.Self is the card's own raise (Thran Portal's shape, whose
// ValidSpell$ refuses first). Refused: any ValidSpell$ (the mana window prices
// no ability raise; Thran Portal's ValidSpell$ Activated.ManaAbility), a
// Cost$ with any part beyond the three above (pips, hybrid, Discard, an
// announced Sac<X> or PayLife<X>), Cost$ beside Amount$, Relative$,
// ValidTarget$, Activator$ (gorge prices a restricted activator; no corpus
// card carries it), a Sac<N> count other than one, AffectedZone$ absent or any
// other zone.
func (lc *lowerCtx) raiseAbility(sp *params, st *lStatic) (bool, error) {
	if sp.str("Type") != "Ability" {
		return false, nil
	}
	if err := lc.raiseAbilityParams(sp, st); err != nil {
		return true, err
	}
	st.AbilityRaise = 1
	st.Kind = "RaiseCost" // the build arm reads AbilityRaise (build_ab.go); the LIR line says RaiseCost like a spell raise's
	cost, hasCost := sp.m["Cost"]
	amount, hasAmount := sp.m["Amount"]
	if hasCost && hasAmount {
		return true, refuse("RaiseCost Type$ Ability with a Cost$ beside an Amount$")
	}
	if !hasCost && !hasAmount {
		return true, refuse("RaiseCost Type$ Ability with no Cost$ or Amount$")
	}
	if hasAmount {
		a, err := lc.value(strings.TrimSpace(amount))
		if err != nil {
			return true, err
		}
		st.Amount = a
		sp.consume("Amount")
	} else if err := lc.raiseAbilityCost(sp, strings.TrimSpace(cost), st); err != nil {
		return true, err
	}
	sp.consume("Type", "ValidCard", "AffectedZone")
	if left := sp.unread(); len(left) != 0 {
		return true, refuse("RaiseCost Type$ Ability with unread params %v", left)
	}
	return true, nil
}

// raiseAbilityParams lowers the static's shape: which abilities (ValidCard$ on
// battlefield sources) and which refusals outside the kernel's pricing
// (raiseAbility's doc).
func (lc *lowerCtx) raiseAbilityParams(sp *params, st *lStatic) error {
	if vs, ok := sp.get("ValidSpell"); ok {
		return refuse("RaiseCost Type$ Ability with ValidSpell$ %q: the mana window prices no ability raise", vs)
	}
	var err error
	vc := sp.str("ValidCard")
	if st.F, err = lc.filter(vc); err != nil {
		return err
	}
	if vc == "" {
		return refuse("RaiseCost Type$ Ability without ValidCard$")
	}
	switch az := sp.str("AffectedZone"); az {
	case "Battlefield":
	case "":
		return refuse("RaiseCost Type$ Ability with no AffectedZone$")
	default:
		return refuse("RaiseCost AffectedZone %q", az)
	}
	for _, k := range []string{"Relative", "ValidTarget", "Activator"} {
		if v, ok := sp.get(k); ok {
			return refuse("RaiseCost %s$ %q on Type$ Ability", k, v)
		}
	}
	if ez, ok := sp.get("EffectZone"); ok {
		if ez != "All" || vc != "Card.Self" {
			return refuse("RaiseCost EffectZone %q on Type$ Ability", ez)
		}
	}
	return nil
}

// raiseAbilityCost lowers the Cost$ raise part (raiseAbility's doc): a pure
// generic mana cost is the raise, a Sac<1/filter> is the pick (F2) and a
// PayLife<N> is a life raise (RaiseLife).
func (lc *lowerCtx) raiseAbilityCost(sp *params, text string, st *lStatic) error {
	c, err := lc.cost(text)
	if err != nil {
		return refuse("RaiseCost Type$ Ability Cost$ %q: %v", text, err)
	}
	if c.Sac != nil {
		if c.SacN != 1 || c.SacX || !raiseManaEmpty(&c.Mana) || c.Tap || c.PayLife != 0 || c.LifeX ||
			c.DiscardN != 0 || c.SubCtr != 0 || c.AddCtr != 0 || c.Loyalty != 0 || c.Exert ||
			c.Forage != nil || c.ExileG != nil || c.TapN != 0 || c.TapPowN != 0 {
			return refuse("RaiseCost Type$ Ability Cost$ %q: a part kop.Cost cannot carry beside the pick (mana %s)", text, &c.Mana)
		}
		st.F2 = c.Sac
		st.Flags = append(st.Flags, "SacPick")
	} else if c.PayLife != 0 {
		if !raiseManaEmpty(&c.Mana) || c.Tap || c.DiscardN != 0 || c.SubCtr != 0 || c.AddCtr != 0 ||
			c.Loyalty != 0 || c.Exert || c.Forage != nil || c.ExileG != nil || c.TapN != 0 || c.TapPowN != 0 {
			return refuse("RaiseCost Type$ Ability Cost$ %q: a part kop.Cost cannot carry beside the life", text)
		}
		if c.LifeX {
			return refuse("RaiseCost Type$ Ability Cost$ %q: an announced PayLife<X>", text)
		}
		st.Amount = cst(c.PayLife)
		st.Flags = append(st.Flags, "RaiseLife")
	} else if c.pureMana() && !c.Mana.X && len(c.Mana.Pips) == 0 && c.Mana.C == 0 && c.Mana.Snow == 0 &&
		len(c.Mana.Hybrid) == 0 && len(c.Mana.Twobrid) == 0 && len(c.Mana.Phyrexian) == 0 {
		if c.Mana.Generic > 255 {
			return refuse("RaiseCost Type$ Ability Cost$ %q: %d generic", text, c.Mana.Generic)
		}
		st.Amount = cst(c.Mana.Generic)
	} else {
		return refuse("RaiseCost Type$ Ability Cost$ %q: a part kop.Cost cannot carry", text)
	}
	sp.consume("Cost")
	return nil
}

// raiseManaEmpty reports whether a lowered mana cost carries no mana symbol:
// None is the parsed "no mana part" mark a pick-only cost carries (raiseMana
// reads it the same way), not a printed "no cost" here.
func raiseManaEmpty(m *manaCost) bool {
	return m.Generic == 0 && m.C == 0 && m.Snow == 0 && !m.X && !m.XX && !m.XXX &&
		m.Pips == [5]int{} && len(m.Hybrid) == 0 && len(m.Twobrid) == 0 && len(m.Phyrexian) == 0
}
