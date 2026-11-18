package oracle

import "testing"

// CR 702.16b with CR 702.96a: an overloaded Eldritch Immunity gives every
// creature its controller has protection from each color, so Seven-Tail
// Mentor's white death trigger ("put a +1/+1 counter on target creature or
// Vehicle you control") may target only the Squirrel created after the
// overloaded spell resolved. The kernel offered every creature, as if the
// each-variant's protection grant had never applied (decision 791 of 791,
// tail-55-c000 pair 2: Angelic Sleuth, Duskmantle Operative, Glory Bearers,
// Snubhorn Sentry and two Squirrels offered beside the new Squirrel).
func TestMBCAngelicSleuthReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "overload_pump_protection_blocks_trigger_target")
}
