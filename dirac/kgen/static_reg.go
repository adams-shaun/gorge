package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// static_reg.go holds the registry behind (*cardBuilder).static's mode
// dispatch. The build arms were one growing switch in build_ab.go: every new
// Mode$ added an arm to the same 700-line function, so parallel seats collided
// in it. A new arm is now a static_<mode>.go file and one registerStaticBuild
// line, mirroring the parse-mode registry (staticModes in static_mode.go).
//
// staticBuilds is lookup only: it is never ranged, so registration order
// never reaches an output table.
var staticBuilds = map[string]staticBuild{}

// staticBuild is one registered static arm's body: it fills the shared
// prologue's kop.Static (Cond2 and OvGate are already set) and returns the
// subtypes it adds, like the switch arm it replaces.
type staticBuild func(cb *cardBuilder, s lStatic, st *kop.Static) (add []uint8, err error)

// registerStaticBuild registers the build arm of Mode$ mode; a second
// registration panics.
func registerStaticBuild(mode string, f staticBuild) {
	if _, dup := staticBuilds[mode]; dup {
		panic("kgen: static build arm " + mode + " registered twice")
	}
	staticBuilds[mode] = f
}
