package kgen

import "strings"

// spellsUncounterable lowers a battlefield R:Event$ Counter | Layer$
// CantHappen line with no ValidCard$ (Chimil, the
// Inner Sun, Sphinx of the Final Word).
//
// gorge counterReplacementMatchesAll scans every object's Counter Repls on
// each Counter effect (effCounter -> CounterAllowed); with no ReplaceWith$
// stopping the event is the whole replacement (replacement.go). ValidSA$ is
// counterValidSA read against the spell being countered, YouCtrl relative
// to the source's controller, and ActiveZones$ Battlefield limits it to a
// source on the battlefield. Only the Spell kind lowers, each alternative
// "Spell[.<card types>]+YouCtrl": YouCtrl is structural in the kernel (the
// spell's controller is the source's), the type words become the Filter.
// An Activated or Triggered alternative, a missing YouCtrl or any other
// qualifier refuses.
func (lc *lowerCtx) spellsUncounterable(rp *params, rl *lRepl) error {
	const shape = "Counter replacement shape"
	if rp.str("Layer") != "CantHappen" {
		return refuse(shape)
	}
	var alts []string
	for _, alt := range strings.Split(rp.str("ValidSA"), ",") {
		kind, quals, _ := strings.Cut(strings.TrimSpace(alt), ".")
		if kind != "Spell" || quals == "" {
			return refuse(shape)
		}
		you := false
		expr := "Card"
		for _, q := range strings.Split(quals, "+") {
			switch {
			case q == "YouCtrl" && !you:
				you = true
			case uncounterTypeWords[q]:
				expr += "." + q
			default:
				return refuse(shape)
			}
		}
		if !you {
			return refuse(shape)
		}
		alts = append(alts, expr)
	}
	f, err := lc.filter(strings.Join(alts, ","))
	if err != nil {
		return err
	}
	rl.F, rl.Kind = f, "SpellsCantBeCountered"
	return nil
}

// uncounterTypeWords are the card types a spell qualifier may name.
var uncounterTypeWords = map[string]bool{
	"Land": true, "Creature": true, "Artifact": true, "Enchantment": true,
	"Planeswalker": true, "Instant": true, "Sorcery": true, "Battle": true,
}
