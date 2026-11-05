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
// card's own "costs more". Refused: Cost$ (a non-generic raise) bar PayLife and raisePerTarget's shape, Relative$,
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
		if ez != "All" || sp.str("ValidCard") != "Card.Self" {
			return refuse("RaiseCost EffectZone %q", ez)
		}
		st.Zone = zAny
	}
	return nil
}

// raisePerTarget lowers the one shape of "this spell costs {pips} more for
// each target beyond the first" (Officious Interrogation): a self spell raise
// with Cost$ (digits and colour letters), Relative$ True and Amount$
// TargetedObjects$Amount/Minus.1, no other parameter. st.Amount is the generic
// per charge, st.RaiseCol the pips per charge. false, nil for any other
// static, which keeps its existing path (and refusal).
func (lc *lowerCtx) raisePerTarget(sp *params, st *lStatic) (bool, error) {
	cost, ok := sp.m["Cost"]
	if !ok || sp.str("Relative") != "True" || sp.str("ValidCard") != "Card.Self" ||
		sp.str("Type") != "Spell" || sp.str("EffectZone") != "All" {
		return false, nil
	}
	amt := sp.str("Amount")
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
	generic := 0
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
	for _, k := range []string{"Cost", "Amount", "Relative"} {
		sp.consume(k)
	}
	st.Amount = cst(generic)
	st.Zone = zAny // EffectZone$ All with Card.Self: the card's own raise
	st.Flags = append(st.Flags, "RaisePerTarget")
	return true, nil
}
