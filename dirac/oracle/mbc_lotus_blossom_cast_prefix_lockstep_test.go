package oracle

import "testing"

// CR 601.2e/f, 605.1a, 106.4: a play whose payment plan reaches gorge's
// last-resort arm for a Produced$ Any sacrifice source -- Lotus Blossom's
// "{T}, Sacrifice this artifact: Add one mana of any colour for each petal
// counter on it" -- taps and sacrifices the source and floats the chosen
// mana before the next decision, while the kernel's planner (mana.go phase
// 1 pass 2) prices the same source only at payment, after targeting. gorge
// floated two red beside Rabbit Battery's activation target ask (pair 28 of
// tail-74 focus Lotus Blossom, seed 108741, surface full).
func TestMBCLotusBlossomCastPrefixLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_lotus_blossom_cast_prefix")
}
