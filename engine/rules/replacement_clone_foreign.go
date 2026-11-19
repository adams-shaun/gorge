package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// seedCloneEntry records the as-enters Clone inputs of an ETBReplacement:Copy
// body m applying to the entry ev (o is the replacement's host).
//
// The carrier's OWN entry (Spark Double) reads the copy election recorded on
// it before the move. A FOREIGN entry (Infinite Reflection: "nontoken
// creatures you control enter as a copy of enchanted creature", CR
// 614.1c/707.2) is no election: the host is another permanent, so the body
// resolves as an ordinary Clone -- Defined$ Enchanted read from the host, the
// entering object the become operand. A foreign body outside that shape keeps
// the election inputs, whose absent choice is the loud unimplemented-API
// fallback.
func seedCloneEntry(ctx *effects.Ctx, o *state.Object, m replMatch, ev events.Event) {
	ctx.CloneEnter.Become = ev.Obj
	ctx.CloneEnter.BecomeValid = true
	if m.id != ev.Obj && effects.CloneForeignEntryOffered(m.repl.With) {
		return
	}
	ctx.CloneEnter.ETB = true
	ctx.CloneEnter.ChoiceValid = o.ETBCloneChoiceValid
	ctx.CloneEnter.Choice = o.ETBCloneChoice
}
