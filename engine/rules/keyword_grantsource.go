package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// KeywordGrantSource is the source of the first active layer-6 effect that
// grants object id the keyword line (CR 613.1f), 0 when none does. The first
// in active()'s order is the oldest by timestamp: the grantor a granted
// keyword's activated ability is attributed to when two sources grant the same
// line (the offer collapses the second's identical ability into the first's).
// It is a read-only scan for the dirac oracle, which tells a granted-keyword
// activation apart by its grantor; the engine itself offers the ability
// without one (legal_walk_battlefield.go's granted-keyword block).
func (e *Engine) KeywordGrantSource(id state.ObjID, line string) state.ObjID {
	o := e.G.Obj(id)
	if o == nil {
		return 0
	}
	line = strings.TrimSpace(line)
	statics := e.active()
	for i := range statics {
		ce := &statics[i]
		if len(ce.AddKeywords) == 0 || !e.matchesSpecFrom(ce.Affects, id, ce.Controller, ce.Source) || e.grantLostTo(ce, o) {
			continue
		}
		for _, kw := range ce.AddKeywords {
			if strings.EqualFold(strings.TrimSpace(kw), line) {
				return ce.Source
			}
		}
	}
	return 0
}
