package oracle

import "testing"

// CR 107.4f with CR 609.4b: Gonti, Lord of Luxury exiles Thundering Tanadon
// ({4}{G/P}{G/P}) with a may-cast permission whose mana may be of any type.
// Its controller holds four lands and enough life, so the Phyrexian pips are
// payable with life and the cast is offered. The rider folds a plain hybrid
// pip into generic mana, but a Phyrexian pip is not a hybrid one: gorge's
// planner keeps it (pay.PlanManaUnderRider), while dirac once folded the
// Phyrexian marker into one more generic and priced the pips as {2/G}, so it
// left the cast out (decision 667, tail-28 pair 4, seed 601411).
func TestMBCThunderingTanadonMayPlayReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_anytype_phyrexian_pips")
}
