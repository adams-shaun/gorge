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

// btSelfFirstTime reads FirstTime$ True on the ungated Card.Self self line
// ("becomes the target of a spell or ability for the first time each turn",
// Kira, Great Glass-Spinner, Angelic Cub; gorge becomesTargetMatches'
// firstTimeTargetedThisTurn: no EARLIER TargetsChosen event this turn named
// the object). The line's TfSelf shape queues on every targeting of its
// object, so the first targeting always queues and spends the kernel's
// per-source Tf2FirstTime latch (trig_counterfirst.go, keyed on the trigger
// object's version): the once-a-turn cadence gorge's per-object log scan
// gives. Only this ungated self shape may carry it: with a source gate the
// line does not queue on a targeting gorge counts (its own controller's),
// which a queue-side latch cannot see.
func btSelfFirstTime(tp *params, tr *lTrigger) error {
	v, ok := tp.get("FirstTime")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("BecomesTarget FirstTime$ %q", v)
	}
	tr.First = true
	return nil
}

// btTargetList lowers a BecomesTarget watcher's ValidTarget$ alternatives:
// gorge becomesTargetMatches matches the whole ValidTarget$ spec against each
// chosen target, so a comma list of permanent filters is the OR of the
// alternatives (the filter's Clauses), one trigger per matching target. A
// list naming a player refuses (the kernel's targetedOne walk sees object
// targets only, dirac/cast.go: a lowered player half would silently never
// fire), and so does a Card.Self alternative (the watcher shape). A single
// alternative lowers exactly as one filter always did.
func (lc *lowerCtx) btTargetList(alts []string) (*lFilter, error) {
	f, err := lc.filter(strings.Join(alts, ","))
	if err != nil {
		return nil, err
	}
	if len(alts) > 1 {
		if f.Players != 0 {
			return nil, refuse("BecomesTarget list with a player alternative")
		}
		for _, c := range f.Clauses {
			if c.Flags&fSelf != 0 {
				return nil, refuse("BecomesTarget shape")
			}
		}
	}
	return f, nil
}

// lowerBTSpellSource lowers a BecomesTarget line whose ValidSource$ is a spell
// form (gorge becomesTargetSourceMatches reads the targeting stack object as
// an ordinary filter whose Spell base is any stack object) over
// ValidTarget$ Card.Self, one filter or a comma list of filters: the watcher
// arms with a spell gate, the controller in Whose and the type in the card
// filter (kernel btSpellSrcOK).
func (lc *lowerCtx) lowerBTSpellSource(tp *params, tr *lTrigger) (err error) {
	src := btSpellSource(tp.str("ValidSource"))
	switch vt := tp.str("ValidTarget"); {
	case vt == "Card.Self":
		tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
	case strings.Contains(vt, "Card.Self"):
		return refuse("BecomesTarget shape")
	default:
		if tr.F, err = lc.btTargetList(strings.Split(vt, ",")); err != nil {
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
// ValidSource$ over a ValidTarget$ that is Card.Self or permanent filters
// (one alternative or a comma list), and no Ward, Valiant or FirstTime.
func isBTSpellSourceLine(tp *params) bool {
	vt := tp.str("ValidTarget")
	return btSpellSource(tp.str("ValidSource")).ok && vt != "" &&
		!(strings.Contains(vt, ",") && strings.Contains(vt, "Card.Self")) &&
		!tp.is("Ward", "True") && !tp.is("Valiant", "True") && !tp.is("FirstTime", "True")
}
