package oracle

import "testing"

// CR 601.2b, 107.4f, 605.1a: Mutagenic Growth's {G/P} pip is announced as {G}
// or two life. The caster's only untapped green source is Gilanra, Caller of
// Wirewood, a TriggersWhenSpent$ source the payment planner defers (its mana is
// not in the planner's census); gorge's pip ask still offers {G} because the
// mana window can tap Gilanra, and the kernel's pip ask omitted it, leaving
// two life as the only face (decision 266 of dr-mbc-baab pair 117).
func TestMBCPorcelainLegionnaireReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_pip_deferred_source")
}
