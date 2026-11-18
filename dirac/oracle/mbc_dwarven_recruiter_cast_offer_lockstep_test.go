package oracle

import "testing"

// CR 601.2g, 605.1a: a cast's cost may be paid with a mana ability that taps
// and sacrifices the source for a determinate amount (Lotus Blossom with one
// petal counter). Dwarven Recruiter {2}{R} beside an untapped Mountain, an
// untapped Darkslick Shores and an untapped Lotus Blossom with one petal
// counter: gorge offers the cast (decision 359 of the recorded game, pool
// tail-74, surface full); the kernel offered pass and Lotus Blossom's cast
// only, because its counter-scaled sacrifice-source shape recognized a single
// fixed colour and not a Produced$ Any one.
func TestMBCDwarvenRecruiterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dwarven_recruiter_cast_offer")
}
