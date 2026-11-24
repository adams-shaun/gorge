package kgen

import "testing"

// delayedAtEOTCarriers are the cards the gap-delayed-ateot slice lowers: an
// AtEOT$ rider (Destroy, SacrificeCtrl, Hand, ExileCombat, YourExile,
// YourSacrificeUpkeep) over a pump, a token, a copy, a targeted return, a
// one-card pick or a one-card search; a delayed trigger spelled
// Phase$ End Of Turn; and a delayed "when it dies under your control this
// turn".
var delayedAtEOTCarriers = []string{
	"Desperate Measures",
	"Footsteps of the Goryo",
	"Force of Rage",
	"Geist of Saint Traft",
	"Greasefang, Okiba Boss",
	"Ilharg, the Raze-Boar",
	"Invocation of Saint Traft",
	"Kari Zev, Skyship Raider",
	"Liesa, Forgotten Archangel",
	"Nahiri, the Harbinger",
	"Planebound Accomplice",
	"Purphoros, Bronze-Blooded",
	"Sentinel of the Pearl Trident",
	"Stone Idol Trap",
	"Thunderkin Awakener",
	"Wine of Blood and Iron",
	"Astral Slide",
	"Avacyn, Angel of Horror",
	"Celestial Sword",
	"Crazed Armodon",
	"Ghoulish Impetus",
	"Glyph of Destruction",
	"Goblin Ski Patrol",
	"Mogg Cannon",
}

func TestDelayedAtEOTSlice(t *testing.T) {
	for _, name := range delayedAtEOTCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// The forms the slice leaves refused, each for a reason this change measured:
// gorge's Defined$ DelayTriggerRemembered finds only the remembered object
// itself (CR 400.7), so the leave-return of Graceful Reprieve and Adarkar
// Valkyrie moves nothing in gorge; gorge copies nothing for a Mirror Mockery
// whose attacker already left; the delayed main-phase mana, the cleanup
// sacrifice and the remaining ChangesZone shapes have no kernel program yet.
func TestDelayedAtEOTSliceRefusals(t *testing.T) {
	for _, name := range []string{"Graceful Reprieve", "Adarkar Valkyrie", "Mirror Mockery", "Mana Drain", "Cunning"} {
		if c := lowerNamedCard(t, name); len(c.Refusals) == 0 {
			t.Errorf("%s lowers; the slice leaves it refused", name)
		}
	}
}
