package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The SpellCast ValidSA$ filter words (dirac/filter_spellability.go): the
// cast spell chose modes, has exactly one target, or paid teamwork. Their
// manifest context is "spell": a SpellCast trigger's filter is built for the
// stack, and a battlefield or card matcher refuses them (gorge reads the same
// fields on any object, which the kernel keeps for a spell's entry only).

func init() {
	fwName[kop.FwModal] = "FwModal"
	fwName[kop.FwSingleTarget] = "FwSingleTarget"
	fwName[kop.FwTeamwork] = "FwTeamwork"

	// Modal: gorge's `Modal` predicate, len(ChosenModes) > 0.
	fwArms["Modal"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwModal})
	}
	// singleTarget: gorge's `singleTarget`, len(Targets)+len(SubTargets) == 1.
	fwArms["singleTarget"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwSingleTarget})
	}
	// Teamwork is not an arm: the filter's Teamwork flag (fTeamwork, Card.Self
	// on a Teamwork carrier) answers it first, and reads the host's own
	// provenance. validSAHead's Spell.Teamwork takes teamworkClause instead.
}

// teamworkClause is the one-clause filter "the spell paid teamwork": the bare
// Card filter plus the FwTeamwork literal.
func (lc *lowerCtx) teamworkClause() (*lFilter, error) {
	f, err := lc.filter("Card")
	if err != nil {
		return nil, err
	}
	if len(f.Clauses) != 1 {
		return nil, refuse("spell-ability filter ValidSA %q", "Spell.Teamwork")
	}
	if err := f.Clauses[0].fwLit(lSpillLit{W: kop.FwTeamwork}); err != nil {
		return nil, err
	}
	return f, nil
}

// spellWordClauses maps each SpellCast ValidSA$ head the three words answer
// to the Card filter spec it lowers as (gorge ValidSAMatches: a single-field
// clause is a plain spec filter over the cast spell, and the base "Spell" is
// no type gate). Spell.Teamwork is "" : teamworkClause builds it.
var spellWordClauses = map[string]string{
	"Spell.Modal":        "Card.Modal",
	"Spell.singleTarget": "Card.singleTarget",
	"Instant.singleTarget,Sorcery.singleTarget": "Instant.singleTarget,Sorcery.singleTarget",
	"Spell.Teamwork": "",
}

// spellWordHead lowers one of spellWordClauses' heads into tr.F. The filter
// must be the trigger's only one (a ValidCard$ beside it stays refused: the
// two would have to be ANDed).
func (lc *lowerCtx) spellWordHead(v string, tr *lTrigger) error {
	if tr.F != nil || tr.OrTargets {
		return refuse("spell-ability filter ValidSA %q beside a ValidCard$", v)
	}
	var err error
	if spec := spellWordClauses[v]; spec == "" {
		tr.F, err = lc.teamworkClause()
	} else {
		tr.F, err = lc.filter(spec)
	}
	return err
}

// validSAWhy is the reason tag of a ValidSA$ head no word answers: the cast's
// mana provenance (no kernel reader), or a word gorge itself does not read
// (the compiled matcher fails closed on Kicked, Warp and numTargets, so the
// trigger never fires there and a kernel word would diverge). The tag is
// stripped from the refusal class (refusalClasses).
func validSAWhy(v string) string {
	for _, p := range [...]string{"Spell.ManaSpentBy", "Spell.ManaSpent ", "Spell.ManaFromCreature"} {
		if strings.HasPrefix(v, p) {
			return " [mana-provenance]"
		}
	}
	for _, p := range [...]string{"Spell.Kicked", "Spell.Warp", "Spell.numTargets"} {
		if strings.HasPrefix(v, p) {
			return " [gorge-unmodelled]"
		}
	}
	return ""
}
