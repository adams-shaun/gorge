package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// StackTriggerContext is the firing event's referents a triggered ability on
// the stack resolves with (the TriggerContext resolveTop seeds its Ctx from),
// the zero value for a spell or an ability with none. A read for a differential
// harness (dirac/oracle atomdiff's set atoms); it changes nothing.
func StackTriggerContext(e *Engine, id state.ObjID) effects.TriggerContext {
	return e.triggerContexts[id]
}
