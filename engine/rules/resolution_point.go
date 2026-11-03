// resolution_point.go holds the resume point: the small record a rules-side
// payment window carries to re-enter a resolution's body once the window has
// settled (resumeResolution, engine/rules/resolution.go).

package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// resumePoint names the body to re-enter: which stack object's resolution,
// the ability to run and the kind of window that settled before it. Plain
// data (the *cards.SA is shared immutable card data).
type resumePoint struct {
	// kind is "optional" (an optional trigger's yes) or "effect_paid" (a
	// triggered Cost$ window paid).
	kind string      `clone:"deep"`
	obj  state.ObjID `clone:"deep"`
	sa   *cards.SA   `clone:"share"`
	// tapPaidX is the count the triggered-cost window's dynamic tapXType<X/
	// Spec> election paid; winPaidX is the X the window's X fold announced or
	// fixed. Each seeds Ctx.X on the re-entry (the trigger object was never
	// paid an X). Zero elsewhere.
	tapPaidX int32 `clone:"deep"`
	winPaidX int32 `clone:"deep"`
	// winXBound marks winPaidX as bound even at zero: a window that announced
	// X = 0 paid nothing, whatever X the triggering permanent carries.
	winXBound bool `clone:"deep"`
}
