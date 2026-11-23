package oracle

import "testing"

// CR 601.2g with CR 605.3c: Urza, Lord High Artificer's repeatable "Tap an
// untapped artifact you control: Add {U}" is one legal activation per untapped
// artifact, so Tragic Lesson, exiled by Urza's {5} ability and playable this
// turn, is castable beside Thorn of Amethyst's tax and the artifacts Urza can
// tap. The payment-plan census priced the ability as a single {U}, so the
// kernel offered the cast and gorge withheld it (decision 378 of the
// tail-64-c001 pair 93 game the evidence loop parted on). The same cause as
// tapanother_census_one_unit; fixed by 380060b39b.
func TestMBCTragicLessonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapanother_census_one_unit_cast_offer")
}
