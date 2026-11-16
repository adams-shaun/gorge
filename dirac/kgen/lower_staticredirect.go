package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The Repl.C bits of a static damage redirect (kop.ReplDamageRedirect), as
// the lowering stores them in rl.Bits.
const (
	rdrToYou        = 1 << 0 // ValidTarget$ You: the recipient is this source's controller
	rdrToYouPerms   = 1 << 1 // ...or a permanent they control, this source's not among them (Permanent.Other+YouCtrl)
	rdrToEnchanted  = 1 << 2 // ...or the object this source is attached to (Card.EnchantedBy)
	rdrAsSelf       = 1 << 3 // VarValue$ Self: the damage is dealt to this source instead
	rdrAsAttached   = 1 << 4 // ...to the object it is attached to (Enchanted, Equipped)
	rdrAsSrcCtrl    = 1 << 5 // ...to the damage source's controller (ReplacedSourceController)
	rdrAsTgtCtrl    = 1 << 6 // ...to the current recipient's controller (ReplacedTargetController)
	rdrGateUntapped = 1 << 7 // only while this source is untapped (IsPresent$ Card.Self+untapped)
	rdrGateEquipped = 1 << 8 // only while this source is attached to something (IsPresent$ Card.equipping)
	rdrCombatOnly   = 1 << 9 // combat damage only (IsCombat$ True)
	rdrSrcUnblocked = 1 << 10 // only an unblocked attacking creature (ValidSource$ Creature.unblocked)
	rdrSrcChosen    = 1 << 11 // a spell source of this source's chosen colour (ValidSource$ ...ChosenColor)
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
	bits := 0
	switch vt := rp.str("ValidTarget"); vt {
	case "You":
		bits |= rdrToYou
	case "You,Permanent.Other+YouCtrl":
		bits |= rdrToYou | rdrToYouPerms
	case "Card.EnchantedBy":
		bits |= rdrToEnchanted
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
		bits |= rdrGateUntapped
	case hasIP && strings.TrimSpace(ip) == "Card.equipping" && hasPD && strings.TrimSpace(pd) == "Self":
		bits |= rdrGateEquipped
	default:
		return true, refuse("repl-hard: StaticDmgRedirect gate %q", ip)
	}
	switch ic := rp.str("IsCombat"); ic {
	case "":
	case "True":
		bits |= rdrCombatOnly
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
			rl.F2, bits = f, bits|rdrSrcUnblocked
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
			rl.F2, bits = f, bits|rdrSrcChosen
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
		bits |= rdrAsSelf
	case dt == vv && (dt == "Enchanted" || dt == "Equipped"):
		bits |= rdrAsAttached
	case dt == vv && dt == "ReplacedSourceController":
		bits |= rdrAsSrcCtrl
	case dt == vv && dt == "ReplacedTargetController":
		bits |= rdrAsTgtCtrl
	default:
		return true, refuse("repl-hard: StaticDmgRedirect DamageTarget$ %q", dt)
	}
	switch w.Params["VarType"] {
	case "Card":
		if bits&rdrAsSelf == 0 && bits&rdrAsAttached == 0 {
			return true, refuse("repl-hard: StaticDmgRedirect VarType$ Card body")
		}
	case "Player":
		if bits&rdrAsSrcCtrl == 0 && bits&rdrAsTgtCtrl == 0 {
			return true, refuse("repl-hard: StaticDmgRedirect VarType$ Player body")
		}
	default:
		return true, refuse("repl-hard: StaticDmgRedirect VarType$ %q", w.Params["VarType"])
	}
	rp.consume("Description", "ValidTarget", "ValidSource", "IsPresent", "PresentDefined", "IsCombat", "DamageTarget")
	rl.Kind = "DamageRedirect"
	rl.Bits = bits
	return true, nil
}

// ensure the C-bit block stays inside kop's int16 Repl.C
