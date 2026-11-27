package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// static_reg.go holds the registry behind (*cardBuilder).static's mode
// dispatch. The build arms were one growing switch in build_ab.go: every new
// Mode$ added an arm to the same 700-line function, so parallel seats collided
// in it. Every arm is registered now; an unknown Mode$ is the lookup's
// miss. A new arm is a static_<mode>.go file and one registerStaticBuild
// line, mirroring the parse-mode registry (staticModes in static_mode.go).
//
// staticBuilds is lookup only: it is never ranged, so registration order
// never reaches an output table.
var staticBuilds = map[string]staticArm{}

// staticArm is a registered arm as static() runs it: the whole ability, so
// an arm that also sets the ability's Cost (EscapeGrant, AlternativeCost)
// reaches it. registerStaticBuild wraps a staticBuild into one.
type staticArm func(cb *cardBuilder, s lStatic, ka *kop.Ability) (add []uint8, err error)

// staticCostBuild is a staticBuild that also fills the ability's Cost.
type staticCostBuild func(cb *cardBuilder, s lStatic, st *kop.Static, cost *kop.Cost) (add []uint8, err error)

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
	staticBuilds[mode] = func(cb *cardBuilder, s lStatic, ka *kop.Ability) ([]uint8, error) {
		return f(cb, s, &ka.Static)
	}
}

// registerStaticCostBuild registers the build arm of Mode$ mode that fills
// the ability's Cost as well; a second registration panics.
func registerStaticCostBuild(mode string, f staticCostBuild) {
	if _, dup := staticBuilds[mode]; dup {
		panic("kgen: static build arm " + mode + " registered twice")
	}
	staticBuilds[mode] = func(cb *cardBuilder, s lStatic, ka *kop.Ability) ([]uint8, error) {
		return f(cb, s, &ka.Static, &ka.Cost)
	}
}

// staticFilterArm is the arm of a mode that is only its kind and the
// affected permanents' filter (read on the battlefield).
func staticFilterArm(kind kop.StaticKind) staticBuild {
	return func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kind
		var err error
		st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield)
		return nil, err
	}
}
