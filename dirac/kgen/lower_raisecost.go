package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// raiseCost lowers S:Mode$ RaiseCost for spells (gorge costModifiersCompose's
// raise group, CR 601.2f: raises before reductions, generic never below
// zero): Type$ Spell, ValidCard$ the spells, a constant or SVar Amount$ of
// generic mana, Activator$ You / Opponent / Player (absent: any caster),
// the static's conditions; EffectZone$ All with ValidCard$ Card.Self is the
// card's own "costs more". EffectZone$ Stack with ValidTarget$ Spell.Self is
// the spell's own "spells that target it cost more while it is on the stack"
// (Kaervek's Torch): the source is a stack object (kop.ZoneStack), and the
// raise charges only a cast whose chosen target is that source. Refused:
// Cost$ (a non-generic raise) bar PayLife and raisePerTarget's shape, Relative$,
// ValidSpell$/AffectedZone$, abilities (Type$ Ability).
func (lc *lowerCtx) raiseCost(sp *params, st *lStatic) error {
	if sp.str("Type") != "Spell" {
		return refuse("RaiseCost Type %q", sp.str("Type"))
	}
	var err error
	if vc := sp.str("ValidCard"); vc != "" {
		if st.F, err = lc.filter(vc); err != nil {
			return err
		}
	}
	if per, err := lc.raisePerTarget(sp, st); err != nil {
		return err
	} else if per {
		return nil
	}
	if life, err := lc.raiseLife(sp, st); err != nil {
		return err
	} else if !life {
		if st.Amount, err = lc.value(sp.str("Amount")); err != nil {
			return err
		}
	}
	if vt, ok := sp.get("ValidTarget"); ok {
		// a raise that applies once a chosen target matches (Dragon's Prey;
		// gorge costTargetsMatch): read as the ReduceCost arm reads it
		lc.spellBaseOK = true
		st.F2, err = lc.filter(vt)
		lc.spellBaseOK = false
		if err != nil {
			return err
		}
	}
	switch a := sp.str("Activator"); a {
	case "", "Player":
	case "You":
		st.Flags = append(st.Flags, "ActivatorYou")
	case "Opponent", "Player.Opponent":
		st.Flags = append(st.Flags, "ActivatorOpp")
	default:
		return refuse("RaiseCost Activator %q", a)
	}
	if ez, ok := sp.get("EffectZone"); ok {
		switch ez {
		case "All":
			if sp.str("ValidCard") != "Card.Self" {
				return refuse("RaiseCost EffectZone %q", ez)
			}
			st.Zone = zAny
		case "Stack":
			// "As long as CARDNAME is on the stack, spells that target it
			// cost {N} more to cast" (Kaervek's Torch): the only stack-zone
			// raise shape this kernel prices. ValidTarget$ Spell.Self makes
			// the charge read the cast's chosen target as the source spell
			// itself; a Cost$ or ValidCard$ is a different shape, refused
			// under its own true reason.
			if sp.str("ValidTarget") != "Spell.Self" {
				return refuse("RaiseCost EffectZone %q without ValidTarget$ Spell.Self", ez)
			}
			if sp.str("ValidCard") != "" {
				return refuse("RaiseCost EffectZone %q with ValidCard$ %q", ez, sp.str("ValidCard"))
			}
			if _, ok := sp.get("Cost"); ok {
				return refuse("RaiseCost EffectZone %q with Cost$", ez)
			}
			if !st.Amount.isConst() {
				return refuse("RaiseCost EffectZone %q with a non-literal Amount$", ez)
			}
			st.Zone = zStack
		default:
			return refuse("RaiseCost EffectZone %q", ez)
		}
	}
	return nil
}

// raisePerTarget lowers the one shape of "this spell costs {pips} more for
// each target beyond the first" (Officious Interrogation): a self spell raise
// with Cost$ (digits and colour letters), Relative$ True and Amount$
// TargetedObjects$Amount/Minus.1, no other parameter. st.Amount is the generic
// per charge, st.RaiseCol the pips per charge. A Cost$-less raise (Fireball's
// "costs {1} more for each target beyond the first", Amount$ an SVar reading
// the targets less one) charges one generic per target: gorge composeRaiseCost
// adds the Amount$ itself as generic mana when no Cost$ names the cost. false,
// nil for any other static, which keeps its existing path (and refusal).
func (lc *lowerCtx) raisePerTarget(sp *params, st *lStatic) (bool, error) {
	// A Cost$-less static reads the map, not sp.get: a read marks the key
	// used, and a static that is not this shape must keep Relative$ and
	// EffectZone$ unread so its own path still refuses them (Hum of the
	// Radix). The Cost$ shape reads as it always did.
	cost, hasCost := sp.m["Cost"]
	str := func(k string) string { return strings.TrimSpace(sp.m[k]) }
	if hasCost {
		str = sp.str
	}
	if str("Relative") != "True" || str("ValidCard") != "Card.Self" ||
		str("Type") != "Spell" || str("EffectZone") != "All" {
		return false, nil
	}
	amt := str("Amount")
	if body, ok := lc.face.SVars[amt]; ok {
		amt = body
	}
	if strings.TrimSpace(amt) != "TargetedObjects$Amount/Minus.1" {
		return false, nil
	}
	for k := range sp.m {
		switch k {
		case "Mode", "ValidCard", "Type", "EffectZone", "Relative", "Cost", "Amount", "Description":
		default:
			return false, nil
		}
	}
	generic := 1 // Cost$ absent: one generic per charge
	if hasCost {
		generic = 0
	}
	var cnt [6]int
	for _, tok := range strings.Fields(cost) {
		if n, err := strconv.Atoi(tok); err == nil && n >= 0 && tok[0] != '+' && tok[0] != '-' {
			generic += n
			continue
		}
		for _, r := range tok {
			i := strings.IndexRune("WUBRGC", r)
			if i < 0 {
				return false, nil
			}
			cnt[i]++
		}
	}
	if generic > 255 {
		return false, nil
	}
	for i, n := range cnt {
		if n > 3 {
			return false, nil
		}
		st.RaiseCol |= uint16(n) << (kop.ReduceColorBits * i)
	}
	sp.consume("Cost", "Amount", "Relative", "EffectZone", "ValidCard", "Type")
	st.Amount = cst(generic)
	st.Zone = zAny // EffectZone$ All with Card.Self: the card's own raise
	st.Flags = append(st.Flags, "RaisePerTarget")
	return true, nil
}
