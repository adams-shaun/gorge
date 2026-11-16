package oracle

import "testing"

// CR 601.2g-h with CR 605.3b: Brink of Disaster on p1's Crumbling Vestige makes
// the Vestige's {C} tap a Taps-deferred source in gorge's planner (the aura's
// destroy trigger is an unvetted body), so gorge's census lists no unit for it
// and, with exactly seven mana sources, offers no cast of Memnarch ({7}). The
// kernel's gap script modelled only any-colour and one-of-WUBRG productions,
// never a plain {C} tap, so its second planning pass priced the Vestige as an
// ordinary unit and offered the cast (tail-46 pair 264, decision 733).
func TestMBCMemnarchReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapsdefer_colourless_gap")
}
