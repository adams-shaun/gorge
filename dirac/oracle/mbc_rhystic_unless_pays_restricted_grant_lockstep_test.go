package oracle

import "testing"

// p0 casts Rage Thrower paying with all six of its lands, and p1's Rhystic
// Study triggers. As the trigger resolves, p0 is offered its unless tax
// {1}: its pool is empty and every land is tapped, and the only untapped
// mana source is Arcbound Tracker's Galazeth-granted "{T}: Add one mana of
// any color. Spend this mana only to cast an instant or sorcery spell."
// (CR 106.6), which a tax payment does not admit. gorge offers the decline
// only; the kernel offered the pay as well.
func TestMBCRageThrowerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rhystic_unless_pays_restricted_grant")
}
