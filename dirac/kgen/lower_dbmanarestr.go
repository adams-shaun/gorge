package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A resolving DB$ Mana with RestrictValid$ Spell.<spec>
// (Abstract Paintmage, Ashling Rekindled's back face, Desolation of Smaug;
// CR 106.6, 605.1a). gorge's effMana adds the units as ManaAdd events carrying
// the restriction, one restriction for every unit of a colour choice
// (askManaChoice picks first); the kernel floats them as restricted mana
// (State.rb) through AddMana F and ExtManaChoice F.

// dbManaActOnly: rv is exactly the undotted Activated (no Activated.<spec>,
// no other alternative).
func dbManaActOnly(rv string) bool { return strings.TrimSpace(rv) == "Activated" }

// dbManaRestrict reads a resolving DB$ Mana's RestrictValid$ (nil: none). Only
// Spell.<spec> alternatives lower; an Activated, Static or non-spell one keeps
// refusing by name.
func (lc *lowerCtx) dbManaRestrict(ps *params) (*lFilter, error) {
	rv, ok := ps.get("RestrictValid")
	if !ok {
		return nil, nil
	}
	var m lMana
	if err := lc.manaRestrict(rv, &m); err != nil {
		return nil, err
	}
	if m.Restrict == nil && !m.NonSpell && m.RestrictAb != nil && dbManaActOnly(rv) {
		// the undotted Activated: every activated-ability payment, no spell
		// and no special action (kop.SpmActivated)
		return &lFilter{Clauses: []lClause{{Special: kop.SpmActivated}}}, nil
	}
	if m.Restrict != nil && m.RestrictAb == nil && m.NonSpell && strings.TrimSpace(rv) == "CantCastNonArtifactSpells" {
		// artifact spells (the clause manaRestrict read) and every payment that
		// is no spell (kop.SpmNonSpell); manaEffect lowers it beside
		// PersistentMana$ True only
		cl := append(append([]lClause(nil), m.Restrict.Clauses...), lClause{Special: kop.SpmNonSpell})
		return &lFilter{Clauses: cl}, nil
	}
	if m.Restrict == nil || m.RestrictAb != nil || m.NonSpell {
		return nil, refuse("DB$ Mana RestrictValid$ %q", rv)
	}
	return m.Restrict, nil
}

// restrictNonSpell: f is the artifact-spells-and-non-spell-payments restriction
// dbManaRestrict lowers CantCastNonArtifactSpells to.
func restrictNonSpell(f *lFilter) bool {
	return f != nil && len(f.Clauses) > 1 && f.Clauses[len(f.Clauses)-1].Special == kop.SpmNonSpell
}

// restrictActOnly: f is the activated-only marker dbManaRestrict lowers an
// undotted Activated to.
func restrictActOnly(f *lFilter) bool {
	return f != nil && len(f.Clauses) == 1 && f.Clauses[0].Special == kop.SpmActivated
}
