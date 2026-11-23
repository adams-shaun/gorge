package oracle

import "testing"

// CR 605.1a/106.12a: p1 casts Wild Hunger ({2}{G}) beside Mirari's Wake with
// Forest, Plains, Jungle Basin ({T}: {C}{G}, an all-at-once source whose
// reflected extra is a pick of its two types) and more lands untapped. gorge's
// planner names no one type over the several-type production, so it defers
// Jungle Basin and pays with Forest and Plains (two doubled taps, a {G} left
// floating); the kernel's plan must leave the source out too while the other
// sources pay, not tap it alone for {C}{G}{G}.
func TestMBCReflectedPickSourceDeferredInPlanLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "reflected_pick_source_deferred_in_plan")
}
