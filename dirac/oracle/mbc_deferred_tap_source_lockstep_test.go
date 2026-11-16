package oracle

import "testing"

// CR 405.2, 602.2b: a mana source whose tap a trigger watches -- p0's Relic
// Bind ("whenever enchanted artifact becomes tapped, choose one") on p1's
// Talisman of Unity -- is deferred for the payment planner
// (payment_plan_interference.go names the watcher), but the activation it
// would fund is a legal play the payer completes in the ordinary manual
// window: the trigger the tap fires is pushed and resolves at the next
// priority round, after the activation. gorge withheld every activation that
// needed the watched source; the kernel's mana-bound afford check offered it
// (tail-08 pairs 18, 51 and 159 as dealt at land 02bb59e13c).
//
// Pair 159, decision 242: p1's Memorial to Unity {2}{G}, {T}, Sacrifice dig
// and the Channel of the Ghost-Lit Nourisher in its hand, both covered only
// through the watched Talisman.
func TestMBCMemorialToUnityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "relic_bind_watch_defers_dig_source")
}

// Pair 18, decision 358: the first equip of the Quietus Spike copy True
// Polymorph made of p0's Snarespinner, {3} covered through the watched
// Honor-Worn Shaku.
func TestMBCTruePolymorphCloneEquipReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "relic_bind_watch_defers_clone_equip")
}

// Pair 51, decision 642: a second equip of True-Faith Censer already attached
// to Steel Wall, {2} covered through the watched source.
func TestMBCTrueFaithCenserSecondEquipReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "relic_bind_watch_defers_second_equip")
}
