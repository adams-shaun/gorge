package kgen

import "testing"

// effectContainersLowered are the gap-effect-containers carriers this slice
// lowers so far: the no-ValidBlocker$ CantBlockBy static over the chain's own
// remembered targets (lower_cantblockplain.go) and the Effect-delivered
// CantRegenerate static with no DealDamage carrying the mark
// (lower_noregen.go's noRegenMark, kernel OpMarkNoRegen) beside the
// damage-linked spellings the existing fold-ins now take (Jaya Ballard's
// RememberObjects$ Remembered, Engulfing Flames' no-own-target Targeted).
//
// The ValidBlocker$ filter form (kernel ExHaste C bit 8: Kick in the Door,
// whose ForgetOnMoved$ Battlefield matches the record's version lapse) and the curse
// form that names the one blocker (C bit 9: Duct Crawler, Screeching Griffin,
// Spin Engine, Kozilek's Pathfinder, Shrewd Hatchling, Auriok Siege Sled,
// Burning-Tree Bloodscale) ride the same record store.
var effectContainersLowered = []string{
	"Engulfing Flames", "Furnace Brood", "Ghostform", "Gravebind",
	"Hurr Jackal", "Jaya Ballard, Task Mage", "Orcish Healer", "Runed Arch",
	"Kick in the Door",
	"Duct Crawler", "Screeching Griffin", "Spin Engine", "Kozilek's Pathfinder",
	"Shrewd Hatchling", "Auriok Siege Sled", "Burning-Tree Bloodscale",
}

func TestEffectContainersSlice(t *testing.T) {
	for _, name := range effectContainersLowered {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// TestEffectContainersCantBlockRefusals pins the two CantBlockBy spellings
// with no ForgetOnMoved$ over the ability's own targets (Cephalid Pathmage,
// Open into Wonder): gorge's Remembered set keeps the object id across a
// move (CR 400.7: a new object), so a blinked target is restricted again on
// return while the kernel's keyword grant lapses. gorge leads; the refusal
// stays until that is settled.
func TestEffectContainersCantBlockRefusals(t *testing.T) {
	// Tower of Coireall, Doorman, Joven's Tools and Firefright Mage carry the
	// same gap on the ValidBlocker$ filter form.
	for _, name := range []string{"Cephalid Pathmage", "Open into Wonder",
		"Tower of Coireall", "Doorman", "Joven's Tools", "Firefright Mage"} {
		if c := lowerNamedCard(t, name); len(c.Refusals) == 0 {
			t.Errorf("%s lowers; want its no-ForgetOnMoved$ refusal", name)
		}
	}
}
