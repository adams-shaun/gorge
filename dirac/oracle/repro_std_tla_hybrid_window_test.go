package oracle

import "testing"

// Abandon Attachments is a hybrid {1}{U/R} Lesson: gorge's cast planner
// refuses a hybrid cost, so the cast is a plain option paid through the manual
// CR 601.2g window. On a board with White Lotus Hideout (whose restricted
// any-colour mana is the first source able to pay the announced {U}) and a
// Serpent's Pass, the window's owedColoursFirst/owedAbilitiesFirst ordering
// picks the Hideout's restricted ability, not the Pass. The kernel must follow
// the same first-useful-source walk.
func TestStdTlaHybridWindowRestrictedAnyReproLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tla", []stdRepro{
		{"hybrid_window_restricted_any_colour", true, false}, // CR 601.2g: the manual window's first useful source is the Hideout's restricted any-colour ability
	})
}
