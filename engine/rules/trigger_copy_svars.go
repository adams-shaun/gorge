package rules

import (
	"github.com/adams-shaun/gorge/engine/state"
)

// keepCopiedTriggerSVars binds a printed trigger of a copy-effect permanent to
// the copied face's script table (CR 707.2, 113.7a). The ability keeps its own
// text once on the stack, so a cost that removes the source (Naga Fleshcrafter
// copying Felothar, Dawn of the Abzan and sacrificing itself to the
// ImmediateTrigger cost) must not make the Execute$ SVar resolve against the
// source's printed face, which no longer defines it.
func (e *Engine) keepCopiedTriggerSVars(id state.ObjID, pt pendingTrigger) {
	if pt.TriggerSVars != nil || pt.Trigger.Mode != "" {
		return
	}
	src := e.G.Obj(pt.Source)
	if src == nil || src.CopyFace == nil || len(src.CopyFace.SVars) == 0 {
		return
	}
	pt.TriggerSVars = src.CopyFace.SVars
	e.recordTriggerLine(id, pt)
}
