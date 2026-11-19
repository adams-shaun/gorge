package main

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// etbCopyShapeGate is cards.Registry.ShapeGate for the coverage report: one
// "shape:ETBReplacement Clone" entry per face whose ETBReplacement:Copy DB$
// Clone body neither CloneETBOffered (the entering carrier's own copy
// election) nor CloneForeignEntryOffered (another entering object's copy,
// Infinite Reflection) admits. Such a card is counted by primitive
// ("api:Clone" is registered) but, cast, takes the loud unimplemented-API
// fallback and enters as itself (usually a 0/0 that dies), so it is not
// playable as printed. It lives here, not in effects, because the
// replacement Keyword$ read is report-only: no Register'd primitive reaches it.
func etbCopyShapeGate(c *cards.Card) []string {
	var out []string
	for _, f := range c.Faces {
		for i := range f.Repls {
			r := &f.Repls[i]
			if r.ParamStr(cards.PKKeyword) != "ETBReplacement" || r.With == nil || r.With.API != "Clone" {
				continue
			}
			if !effects.CloneETBOffered(r.With, f.SVars) && !effects.CloneForeignEntryOffered(r.With) {
				out = append(out, "shape:ETBReplacement Clone")
				break
			}
		}
	}
	return out
}
