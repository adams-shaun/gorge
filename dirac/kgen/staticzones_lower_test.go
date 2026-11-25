package kgen

import (
	"strings"
	"testing"
)

// staticZonesCarriers are the slice's sample: the paper-legal cards whose
// only kgen refusals sit in the gap-static-zones classes this round lowers --
// a Condition$ the shared gate grammar covers (Monarch, and Delirium,
// Metalcraft, Hellbent on the combat-restriction modes), a player static
// (shroud, the additive maximum hand size), a ReduceCost of activated
// abilities with no AffectedZone$ and one gated on the ability's target, the
// hand-default MayPlay zone, and the library-top look beside and
// beside-without a MayPlay grant. They must lower with no refusal (ticket
// gap-static-zones; the classes still refused keep their named reasons in
// TestStaticZonesRefusals).
var staticZonesCarriers = []string{
	"Bladegraft Aspirant", "Case of the Locked Hothouse", "Dawnglade Regent",
	"Dragon's Rage Channeler", "Field of Dreams", "Geist of the Lonely Vigil",
	"Ivory Mask", "Lantern of Insight", "Power Artifact",
	"Queen Mother Ramonda", "Slithering Shade", "Solitary Confinement",
	"Spire Serpent", "Thought Devourer", "Thought Eater", "Thought Nibbler",
	"True Believer", "Trusted Advisor", "Wizened Snitches",
}

func TestStaticZonesSlice(t *testing.T) {
	for _, name := range staticZonesCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// staticZonesRefusals are the slice's other carriers: the same classes, in
// shapes the kernel does not model yet (a multi-zone MayPlay with the command
// zone, a stack grant of Replicate, Prowl, Freerunning or
// Deathtouch, an off-zone grant of miracle, the vote
// counters, K'rrik's pay-life-instead-of-mana) or that gorge itself fails
// closed on -- FatefulHour, ExtraTurn and Night, whose gate has no arm, so
// the whole static is inert there (Gavony Ironwright, Medomai the Ageless,
// Moonrager's Slash: reported, not copied). Each must keep its named
// refusal, so the class stays counted until its own ticket lands. The
// graveyard Unearth grant (Dregscape Sliver) is not pinned here: main lowers
// it (lower_offzone.go offZoneUnearth), so the row would go stale on rebase.
var staticZonesRefusals = []struct{ card, want string }{
	{"Aluren", `MayPlay zone "Hand,Graveyard,Library,Exile,Command"`},
	{"Aminatou, Veil Piercer", `AffectedZone "Hand" without MayPlay`},
	{"Ballot Broker", "player static"},
	{"Djinn Illuminatus", `AffectedZone Stack grant of "Replicate:CardManaCost"`},
	{"Embalmer's Tools", `ReduceCost AffectedZone "": "Creature.YouOwn+inZoneGraveyard" can match a non-battlefield activation`},
	{"Gavony Ironwright", `static Condition "FatefulHour" (gorge fails closed`},
	{"Hurkyl's Final Meditation", `static Condition "NotPlayerTurn"`},
	{"K'rrik, Son of Yawgmoth", "player static"},
	{"Lens of Clarity", "MayLookAt shape"},
	{"Medomai the Ageless", `static Condition "ExtraTurn" (gorge fails closed`},
	{"Moonrager's Slash", `static Condition "Night" (gorge fails closed`},
	{"Qasali Ambusher", `MayPlayDontGrantZonePermissions$ "True" on zone ""`},
	{"Teferi, Mage of Zhalfir", `AffectedZone "Hand,Graveyard,Exile,Library,Command,Stack" without MayPlay`},
	{"Thraben Doomsayer", `static Condition "FatefulHour" (gorge fails closed`},
	{"Village Survivors", `static Condition "FatefulHour" (gorge fails closed`},
	{"Yixlid Jailer", `AffectedZone "Graveyard" without MayPlay`},
}

func TestStaticZonesRefusals(t *testing.T) {
	for _, r := range staticZonesRefusals {
		c := lowerNamedCard(t, r.card)
		if len(c.Refusals) == 0 {
			t.Errorf("%s lowers; want the refusal %q", r.card, r.want)
			continue
		}
		hit := false
		for _, rf := range c.Refusals {
			hit = hit || strings.Contains(rf.Reason, r.want)
		}
		if !hit {
			t.Errorf("%s refusals %v: none carries %q", r.card, c.Refusals, r.want)
		}
	}
}

// TestStaticZonesShapes lowers the slice's shapes with no paper carrier in
// the corpus: the hand-default MayPlay zone (gorge mayPlayGrant reads an
// absent AffectedZone$ as the hand; the corpus's only MayPlay$ True line
// without AffectedZone$, Qasali Ambusher, carries riders that refuse), and
// the CastWithFlash self grant's Condition$ Ferocious gate (gorge
// staticTimingGate's condFerocious arm; no corpus static carries
// Condition$ Ferocious on any mode). Each must lower.
func TestStaticZonesShapes(t *testing.T) {
	handDefault := lowerScriptCard(t, "Hand Default Probe", `Name:Hand Default Probe
ManaCost:2 U
Types:Enchantment
S:Mode$ Continuous | CharacteristicDefining$ True | MayPlay$ True | Description$ You may play cards as though they were in your hand.
Oracle:probe
`)
	if rf := handDefault.Refusals; len(rf) != 0 {
		t.Fatalf("the hand-default MayPlay probe refused: %v", rf)
	}
	found := false
	for _, st := range handDefault.Statics {
		if st.Kind == "MayPlayFrom" && st.Zone == int(zHand) {
			found = true
		}
	}
	if !found {
		t.Error("the hand-default MayPlay probe lowers to no hand-zone MayPlayFrom static")
	}

	ferocious := lowerScriptCard(t, "Ferocious Self Flash Probe", `Name:Ferocious Self Flash Probe
ManaCost:3 G
Types:Creature Beast
PT:2/2
S:Mode$ CastWithFlash | ValidCard$ Card.Self | ValidSA$ Spell | EffectZone$ All | Caster$ You | Condition$ Ferocious | Description$ You may cast this from your hand as though it had flash if you control a creature with power 4 or greater.
Oracle:probe
`)
	if rf := ferocious.Refusals; len(rf) != 0 {
		t.Fatalf("the Ferocious self-flash probe refused: %v", rf)
	}
	condOK := false
	for _, st := range ferocious.Statics {
		condOK = condOK || st.Kind == "CastWithFlash"
	}
	if !condOK {
		t.Error("the Ferocious self-flash probe lowers to no CastWithFlash static")
	}

	// the same grant's plain IsPresent$ gate keeps lowering (the Condition$
	// key is the Ferocious spelling, not the gate itself; a powerGE4 read is
	// outside selfFlashPresent's simple spec, so the plain gate uses a type
	// spec)
	bare := lowerScriptCard(t, "Self Flash Zone Probe", `Name:Self Flash Zone Probe
ManaCost:3 G
Types:Creature Beast
PT:2/2
S:Mode$ CastWithFlash | ValidCard$ Card.Self | ValidSA$ Spell | EffectZone$ All | Caster$ You | IsPresent$ Creature.YouCtrl | Description$ You may cast this from your hand as though it had flash if you control a creature.
Oracle:probe
`)
	if rf := bare.Refusals; len(rf) != 0 {
		t.Fatalf("the plain self-flash probe refused: %v", rf)
	}
}
