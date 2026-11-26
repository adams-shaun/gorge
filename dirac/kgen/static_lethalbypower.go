package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// static_lethalbypower.go registers the build arm of a Continuous static's
// LethalDamageByPower$ True (Zilortha, Strength Incarnate): lethal damage to
// the affected creatures is judged by their power (lower_continuous.go sets
// the Kind; the kernel side is kop/lethalbypower.go).
func init() {
	registerStaticBuild("LethalByPower", staticFilterArm(kop.StatLethalByPower))
}
