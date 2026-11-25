package kgen

import "testing"

// rememberedCardMayPlayCarriers grant "you may play that card for as long as
// it remains exiled" with an Effect whose RememberObjects$ is RememberedCard
// and whose lifetime is ForgetOnMoved$ Exile. gorge's effectRememberedCodes
// maps RememberedCard to Remembered (engine/effects/staticline.go), so each
// lowers to the per-card WhileExiled record the Remembered spelling already
// takes (lower_zone.go partitionMayPlay).
var rememberedCardMayPlayCarriers = []string{
	"Dream-Thief's Bandana", "Edward Kenway", "Gonti, Canny Acquisitor",
	"Heartless Conscription", "Klaw, Master of Sound", "Shadow of the Enemy",
}

// damagedCreatureDelayCarriers register "destroy that creature at end of
// combat" with a DelayedTrigger RememberObjects$ TriggeredTargetLKICopy on a
// per-referent DamageDone line: the delayed program's remembered object is
// the trigger object, the damaged creature (lower_delayed.go
// delayedStepRemember, lower_trigrefs.go readDamageTarget).
var damagedCreatureDelayCarriers = []string{
	"Lowland Basilisk", "Ohran Viper", "Serpentine Basilisk", "Simic Basilisk", "Stone-Tongue Basilisk",
}

func TestRememberedCrossAbility(t *testing.T) {
	for _, name := range append(append([]string(nil), rememberedCardMayPlayCarriers...), damagedCreatureDelayCarriers...) {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
	// the same grant over RememberObjects$ Imprinted is not this shape
	if c := lowerNamedCard(t, "Rassilon, the War President"); len(c.Refusals) == 0 {
		t.Error("Rassilon, the War President lowers; want its Imprinted may-play refused")
	}
}
