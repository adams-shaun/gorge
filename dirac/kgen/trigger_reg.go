package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// trigger_reg.go holds the registry behind (*lowerCtx).lowerTriggered's mode
// dispatch. The arms were one growing switch in lower_trigger.go: every trigger
// change edited the same 1700-line function, so parallel seats collided in it.
// A moved arm is a trig_<mode>.go file and one triggerMode line; the arms still
// in the switch move out the same way, one at a time. Mirrors static_reg.go.
//
// triggerLowers is lookup only: it is never ranged, so registration order never
// reaches an output table.
var triggerLowers = map[string]trigLower{}

// trigArm is what lowerTriggered's shared prologue hands a registered arm: the
// trigger line, its zone override, the consuming parameter reader, the trigger
// record being filled and the filter reader bound to them. giftSelf is the arm's
// one result besides tr, read by the shared epilogue.
type trigArm struct {
	t        cards.Trigger
	zone     string
	tp       *params
	tr       *lTrigger
	readF    func(k string) (*lFilter, error)
	giftSelf bool
}

// trigLower is one registered Mode$ arm's body, the switch arm it replaces.
type trigLower func(lc *lowerCtx, a *trigArm) error

// triggerMode registers the lowering of trigger Mode$ mode; a second
// registration panics.
func triggerMode(mode string, f trigLower) {
	if _, dup := triggerLowers[mode]; dup {
		panic("kgen: trigger mode " + mode + " registered twice")
	}
	triggerLowers[mode] = f
}
