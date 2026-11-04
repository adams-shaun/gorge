package oracle

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// playRepeatDup reports whether option i of a Play ask names a card an
// earlier option already offered (seen, filled by the caller as it offers).
// An AllowRepeats$ Play (Infinite Diversity, "copy that card X times, you may
// cast the copies") lists its one card once per copy; the copies are
// interchangeable, so a pick is offered once, at the first entry not yet
// taken (admissible has dropped those), and the kernel's ChGenCastFree offers
// the card once the same way (dirac/cast_freerepeat.go). Every other
// multi-pick ask is untouched.
func playRepeatDup(d *decision.Decision, seen map[state.ObjID]bool, i int) bool {
	if d.Kind != decision.KModes || d.ResumeKind != "play" {
		return false
	}
	o := &d.Options[i]
	return o.Obj != 0 && seen[o.Obj]
}
