package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// spellsUncounterable lowers a battlefield R:Event$ Counter | Layer$
// CantHappen line with no ValidCard$ (Chimil, the Inner Sun, Sphinx of the
// Final Word, Allosaurus Shepherd, Root Sliver).
//
// gorge counterReplacementMatchesAll scans every object's Counter Repls on
// each Counter effect (effCounter -> CounterAllowed); with no ReplaceWith$
// stopping the event is the whole replacement (replacement.go). ValidSA$ is
// counterValidSA read against the spell being countered: the Spell kind only,
// its qualifiers the controller-relative words (YouCtrl and wasCastByYou are
// one read, the spell's controller is the replacement source's) plus the
// ordinary object-filter grammar gorge's counterSpellQualifiers passes the
// rest through — so the card types, a colour, a subtype, a power compare and
// hasKeywordFlash all lower here as the filter they build, evaluated by the
// kernel's spell-card match (uncounterMatch). An alternative with neither
// controller word (Leyline of Lifeforce, Root Sliver) matches spells of every
// controller, so the whole line lowers with the any-controller read
// (kop.ReplUncAnyCtrl) and the kernel widens its scan to the battlefield;
// alternatives that disagree on the scope refuse, fail-closed. Only the Spell
// kind lowers; an Activated, Triggered or SpellAbility alternative refuses.
// Secondary$ True (Cunning Nightbonder's paired line) is read by nothing in
// gorge's replacement path and consumes.
func (lc *lowerCtx) spellsUncounterable(rp *params, rl *lRepl) error {
	const shape = "Counter replacement shape"
	if rp.str("Layer") != "CantHappen" {
		return refuse(shape)
	}
	rp.consume("Secondary")
	youAll, youAny := true, false
	var alts []string
	for _, alt := range strings.Split(rp.str("ValidSA"), ",") {
		kind, quals, _ := strings.Cut(strings.TrimSpace(alt), ".")
		if kind != "Spell" {
			return refuse(shape)
		}
		you := false
		var quals2 []string
		for _, q := range strings.Split(quals, "+") {
			switch {
			case (q == "YouCtrl" || q == "wasCastByYou") && !you:
				// both are the spell's-controller-is-the-source's read
				you = true
			case q != "":
				quals2 = append(quals2, q)
			}
		}
		expr := "Card"
		if len(quals2) != 0 {
			// gorge counterSpellQualifiers passes the ordinary qualifiers
			// through as one "Card."+join spec
			expr += "." + strings.Join(quals2, "+")
		}
		if you {
			youAny = true
		} else {
			youAll = false
		}
		alts = append(alts, expr)
	}
	if youAny && !youAll {
		return refuse("Counter ValidSA scope mix")
	}
	f, err := lc.filter(strings.Join(alts, ","))
	if err != nil {
		return err
	}
	rl.F, rl.Kind = f, "SpellsCantBeCountered"
	if !youAll {
		rl.Bits = kop.ReplUncAnyCtrl
	}
	return nil
}
