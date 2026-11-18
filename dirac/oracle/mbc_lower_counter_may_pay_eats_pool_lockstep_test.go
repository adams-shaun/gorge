package oracle

import "testing"

// A lowered cast's witness that sacrifices a mana source (Dire Mimic, the
// {B} of Baleful Eidolon's bestowed {4}{B}) stacks a sacrifice trigger
// (Tablet of Epityr's "you may pay {1}. If you do, gain 1 life."). gorge's
// lowering waits for it to resolve (payexec: the play waits for the stack),
// the botpolicy answer plays the may as a do, and the {1} eats the pool's
// {C}, leaving the {4}{B} one short -- so the completion probe dead-ends and
// gorge never offers the bestowed cast (decision 588 of pool tail-72 pair
// 183: the kernel's option list held cast(#40,-,6), the bestowed cast, which
// gorge priced at all only as a dead-ending script).
func TestMBCBalefulEidolonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lower_counter_may_pay_eats_pool")
}
