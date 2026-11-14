package oracle

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// ReproEntries turns a Repro deck (card names, one per copy) into the deck
// lines set.MatchupN takes. A repro writes a card by its front face (cardName)
// and a pool's table names a card with more than one face "Front // Back"
// (Imodane's Recruiter // Train Troops), so a name the table lacks is looked up
// as the registry's card spells it in full; a name neither form finds is left
// as written, for the table to refuse by name.
func ReproEntries(reg *cards.Registry, set cardset.Set, names []string) []cardset.Entry {
	es := make([]cardset.Entry, len(names))
	for i, n := range names {
		es[i] = cardset.Entry{Name: tableName(reg, set, n), Count: 1}
	}
	return es
}

// tableName is name as set's table spells it.
func tableName(reg *cards.Registry, set cardset.Set, name string) string {
	if _, ok := set.Lookup(name); ok {
		return name
	}
	c, ok := reg.Lookup(name)
	if !ok || len(c.Faces) < 2 {
		return name
	}
	parts := make([]string, 0, len(c.Faces))
	for _, f := range c.Faces {
		if f == nil {
			return name
		}
		parts = append(parts, f.Name)
	}
	if full := strings.Join(parts, " // "); full != name {
		if _, ok := set.Lookup(full); ok {
			return full
		}
	}
	return name
}
