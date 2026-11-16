package oracle

import "testing"

// CR 106.6: a spend restriction bars only the payments its restriction names,
// so Karn, Legacy Reforged's "{C} ... can't be spent to cast nonartifact
// spells" still pays an unless tax. With Kataki, War's Wage on the other side
// granting every artifact "sacrifice this unless you pay {1}" upkeep
// triggers, Karn's own trigger had just floated one restricted {C}
// (PersistentMana$ True) beside eleven tapped lands when its granted trigger
// resolved: gorge offered the pay and the decline, the kernel only the
// decline, because its offer gate (canPayGen) prices the pool and the tap
// window and never reads the floating restricted batches (State.rb) the
// payment admits (gorge's UnlessCostPayable reads AvailableFor's
// PurposeOther view). The recording is the first divergence of dr-mbc-abab
// pair 56: decision 457, gorge leading, on the autopay surface.
func TestMBCIdolOfOblivionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_offer_admits_floating_restricted_mana")
}
