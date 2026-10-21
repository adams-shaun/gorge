package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// emitSacrifices emits one sacrifice ACTION's Sacrifice events inside one
// Mode$ SacrificedOnce batch (CR 603.2c). The rules-side cost sites use it so
// a payment that sacrifices several permanents queues the "one or more"
// watcher once; every effect-side sacrifice already enters BatchDepartures,
// whose bracket nests with this one. A single id is a batch of one, exactly
// the unbracketed reading.
func (e *Engine) emitSacrifices(ids []state.ObjID) {
	if len(ids) == 0 {
		return
	}
	e.beginSacrificeBatch()
	for _, id := range ids {
		e.emit(events.Sacrifice(id))
	}
	e.endSacrificeBatch()
}
