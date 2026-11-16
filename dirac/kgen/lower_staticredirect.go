package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// staticDmgRedirect lowers a printed static damage redirect (Pariah, Veteran
// Bodyguard, Treacherous Link, Harsh Judgment, ...):
//
//	R:Event$ DamageDone | ActiveZones$ Battlefield |
//	  ValidTarget$ You[,Permanent.Other+YouCtrl] or Card.EnchantedBy |
//	  [ValidSource$ Creature.unblocked | Artifact |
//	   Instant.ChosenColor,Sorcery.ChosenColor] |
//	  [IsPresent$ Card.Self+untapped or Card.equipping PresentDefined$ Self] |
//	  [IsCombat$ True] | DamageTarget$ <new recipient> |
//	  ReplaceWith$ DB$ ReplaceEffect | VarName$ Affected | VarValue$ <new
//	  recipient> | VarType$ Card|Player
//
// gorge (replacement_replace.go ReplaceEvent, VarName$ Affected) rewrites the
// held damage event's recipient in place while the original source stays the
// damage source; the kernel reads the same shapes off the Repl's C bits and
// source filter (dirac/repl_staticredirect.go). It reports false for a shape
// that is no redirect so damageRepl's other arms read it.
func (lc *lowerCtx) staticDmgRedirect(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if w == nil || w.API != "ReplaceEffect" || w.Sub != nil ||
		strings.TrimSpace(w.Params["VarName"]) != "Affected" {
		return false, nil
	}
	bits := int16(0)
	switch vt := rp.str("ValidTarget"); vt {
	case "You":
		bits |= kop.RdrToYou
	case "You,Permanent.Other+YouCtrl":
		bits |= kop.RdrToYou | kop.RdrToYouPerms
	case "Card.EnchantedBy":
		bits |= kop.RdrToEnchanted
	default:
		return false, nil
	}
	// the IsPresent$ gates (the untapped gate on Veteran Bodyguard, Martyrs
	// of Korlis and Weathered Bodyguards; Pariah's Shield's equipping one).
	// gorge reads both as a PresentCompare$ GE1 count on the source itself
	// (replacement_match.go IsPresent), so the kernel reads the same state
	// off its own frame.
	ip, hasIP := rp.get("IsPresent")
	pd, hasPD := rp.get("PresentDefined")
	switch {
	case !hasIP && !hasPD:
	case hasIP && strings.TrimSpace(ip) == "Card.Self+untapped" && !hasPD:
		bits |= kop.RdrGateUntapped
	case hasIP && strings.TrimSpace(ip) == "Card.equipping" && hasPD && strings.TrimSpace(pd) == "Self":
		bits |= kop.RdrGateEquipped
	default:
		return true, refuse("repl-hard: StaticDmgRedirect gate %q", ip)
	}
	switch ic := rp.str("IsCombat"); ic {
	case "":
	case "True":
		bits |= kop.RdrCombatOnly
	default:
		return true, refuse("repl-hard: StaticDmgRedirect IsCombat$ %q", ic)
	}
	// the source filter: gorge matches it against the damage source
	// (replacement_damage.go ValidSource$); unblocked is a live combat word,
	// the chosen colour is read off this source's own frame.
	if vs, ok := rp.get("ValidSource"); ok {
		switch vs {
		case "Creature.unblocked":
			f, err := lc.filter("Creature")
			if err != nil {
				return true, err
			}
			rl.F2, bits = f, bits|kop.RdrSrcUnblocked
		case "Artifact":
			f, err := lc.filter(vs)
			if err != nil {
				return true, err
			}
			rl.F2 = f
		case "Instant.ChosenColor,Sorcery.ChosenColor":
			f, err := lc.filter(vs)
			if err != nil {
				return true, err
			}
			rl.F2, bits = f, bits|kop.RdrSrcChosen
		default:
			return true, refuse("repl-hard: StaticDmgRedirect ValidSource$ %q", vs)
		}
	}
	// the new recipient: DamageTarget$ names it and the body's VarValue$
	// repeats it (the same word the kernel's rewrite keys off).
	dt := rp.str("DamageTarget")
	vv := strings.TrimSpace(w.Params["VarValue"])
	switch {
	case dt == vv && dt == "Self":
		bits |= kop.RdrAsSelf
	case dt == vv && (dt == "Enchanted" || dt == "Equipped"):
		bits |= kop.RdrAsAttached
	case dt == vv && dt == "ReplacedSourceController":
		bits |= kop.RdrAsSrcCtrl
	case dt == vv && dt == "ReplacedTargetController":
		bits |= kop.RdrAsTgtCtrl
	default:
		return true, refuse("repl-hard: StaticDmgRedirect DamageTarget$ %q", dt)
	}
	switch w.Params["VarType"] {
	case "Card":
		if bits&kop.RdrAsSelf == 0 && bits&kop.RdrAsAttached == 0 {
			return true, refuse("repl-hard: StaticDmgRedirect VarType$ Card body")
		}
	case "Player":
		if bits&kop.RdrAsSrcCtrl == 0 && bits&kop.RdrAsTgtCtrl == 0 {
			return true, refuse("repl-hard: StaticDmgRedirect VarType$ Player body")
		}
	default:
		return true, refuse("repl-hard: StaticDmgRedirect VarType$ %q", w.Params["VarType"])
	}
	rp.consume("Description", "ValidTarget", "ValidSource", "IsPresent", "PresentDefined", "IsCombat", "DamageTarget")
	rl.Kind = "DamageRedirect"
	rl.Bits = int(bits)
	return true, nil
}

// ensure the C-bit block stays inside kop's int16 Repl.C

// rdrRepl builds the ReplDamageRedirect ability: the C bits carry the
// recipient filter, the new recipient and the gates (kop/staticredirect.go),
// Filter2 the damage source.
func rdrRepl(ka kop.Ability, r lRepl, cb *cardBuilder) (kop.Ability, error) {
	rp := &ka.Repl
	rp.Kind = kop.ReplDamageRedirect
	rp.C = int16(r.Bits)
	if r.F2 != nil {
		f, err := cb.filter(r.F2, kop.ZoneBattlefield)
		if err != nil {
			return ka, err
		}
		rp.Filter2 = f
	}
	return ka, nil
}
