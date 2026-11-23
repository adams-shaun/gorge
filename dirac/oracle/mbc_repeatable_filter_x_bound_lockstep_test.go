package oracle

import "testing"

// CR 601.2b-h, 605.3b: Blue Sun's Twilight ({X}{U}{U}, X-bound target) is payable at
// X=0 through Llanowar Envoy's repeatable "{1}{G}: Add one mana of any color", activated
// twice off three Forests and an Abundant Countryside, and its Jasconian Isle (mana value
// 0) is a legal target. gorge offered the cast; the kernel's X-bound sweep counted the
// repeatable filter as one any-colour unit, read {U}{U} unpayable and left the cast out
// (decision 458 of 458, tail-28-c000 pair 34).
func TestMBCBlueSunSTwilightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blue_suns_twilight_repeatable_filter_x_bound")
}
