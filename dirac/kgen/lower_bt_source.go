package kgen

import "strings"

// btSource is a BecomesTarget ValidSource$ spell form (gorge
// becomesTargetSourceMatches over the targeting stack object).
type btSource struct {
	ok     bool
	whose  uint8  // whoAny / whoYou / whoOpp: the spell's controller
	filter string // the spell's card filter ("" = any spell)
}

// btSpellSource reads ValidSource$ Spell, Spell.YouCtrl, Spell.OppCtrl,
// Spell.<Type>[+<Ctrl>] and the bare type list Instant,Sorcery. Any other
// qualifier on Spell is left to the card filter reader, which refuses what it
// does not know. An ability-shaped source (SpellAbility, Ability, Activated)
// is not one.
func btSpellSource(v string) btSource {
	switch {
	case v == "Spell":
		return btSource{ok: true}
	case strings.HasPrefix(v, "Spell."):
		var out btSource
		var rest []string
		for _, q := range strings.Split(v[len("Spell."):], "+") {
			switch q {
			case "YouCtrl", "OppCtrl":
				if out.whose != whoAny {
					return btSource{}
				}
				out.whose = map[string]uint8{"YouCtrl": whoYou, "OppCtrl": whoOpp}[q]
			case "":
				return btSource{}
			default:
				rest = append(rest, q)
			}
		}
		if len(rest) > 0 {
			out.filter = "Card." + strings.Join(rest, "+")
		}
		out.ok = true
		return out
	case v == "Instant,Sorcery":
		return btSource{ok: true, filter: "Card.Instant,Card.Sorcery"}
	}
	return btSource{}
}

// lowerBTSpellSource lowers a BecomesTarget line whose ValidSource$ is a spell
// form (gorge becomesTargetSourceMatches reads the targeting stack object as
// an ordinary filter whose Spell base is any stack object) over
// ValidTarget$ Card.Self or one filter: the watcher arms with a spell gate,
// the controller in Whose and the type in the card filter (kernel
// btSpellSrcOK).
func (lc *lowerCtx) lowerBTSpellSource(tp *params, tr *lTrigger) (err error) {
	src := btSpellSource(tp.str("ValidSource"))
	switch vt := tp.str("ValidTarget"); {
	case vt == "Card.Self":
		tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
	case strings.Contains(vt, "Card.Self"):
		return refuse("BecomesTarget shape")
	default:
		if tr.F, err = lc.filter(vt); err != nil {
			return err
		}
	}
	if src.filter != "" {
		if tr.SrcF, err = lc.filter(src.filter); err != nil {
			return err
		}
	}
	tr.Whose = src.whose
	tr.Watch = true
	tr.SrcSpell = true
	return nil
}

// isBTSpellSourceLine reports a BecomesTarget line this arm lowers: a spell
// ValidSource$ over a ValidTarget$ that is Card.Self or one filter (no
// alternative list), and no Ward, Valiant or FirstTime.
func isBTSpellSourceLine(tp *params) bool {
	vt := tp.str("ValidTarget")
	return btSpellSource(tp.str("ValidSource")).ok && vt != "" && !strings.Contains(vt, ",") &&
		!tp.is("Ward", "True") && !tp.is("Valiant", "True") && !tp.is("FirstTime", "True")
}
