package kgen

import (
	"slices"
	"strings"
	"testing"
)

// The staticzones multi-zone MayPlay slice (ticket gap-static-zones, the
// staticzones umbrella's MayPlay round): the ten paper-legal cards whose
// MayPlay static names more than one AffectedZone$.
//
//   - a non-granting free cast (MayPlayDontGrantZonePermissions$ True with
//     MayPlayWithoutManaCost$ True): Dracogenesis. A non-granting permission
//     opens no zone, but gorge rules/mayplay.go gates the free ride by
//     affectedZoneOK, so the full five-zone list is ignored and lowers to
//     Weftwalking's shape (StFreeOnly over the hand, which the kernel reads
//     from every zone). A partial list (One with the Multiverse's
//     Library,Hand) refuses: the kernel's free ride would reach a graveyard
//     or exile cast the list withholds, and Static.Zone holds one zone.
//   - a non-granting alternative cost (the {0}, the pay-life and the
//     pay-energy MayPlayAltManaCost$ beside MayPlayDontGrantZonePermissions$
//     True): As Foretold, Demon of Fate's Design, Nissa, Worldsoul Speaker.
//     The kernel's StMayZeroAlt marks the static a non-granting alternative
//     (cast_mayzeroalt.go), which the hand walk lists beside the plain cast
//     (gorge legal_walk_hand.go alternativeCosts) and the may-play walk
//     offers beside a printed-cost permission.
//   - a two-zone self grant: Squee, the Immortal's Exile,Graveyard with
//     EffectZone$ Exile,Graveyard: one static with both function flags, so
//     the self walk reaches it from either zone (both class gates set).
//
// Aluren (any player), Primal Prayers (cast as though it had flash),
// Conspiracy Unraveler (an evidence alternative the kernel has no cost for)
// and Durnan (an Effect-delivered permission with an Undaunted rider) keep
// their refusals -- every one off the "MayPlay zone" class, at the named
// shape that now withholds them.
var mayPlayFormsCarriers = []struct {
	name string
	want string // non-empty: the card keeps this named refusal
}{
	{name: "Aluren", want: `MayPlayPlayer$ "Player"`},
	{name: "As Foretold"},
	{name: "Conspiracy Unraveler", want: `MayPlayAltManaCost "CollectEvidence<10>"`},
	{name: "Demon of Fate's Design"},
	{name: "Dracogenesis"},
	{name: "Durnan of the Yawning Portal", want: `MayPlay zones "Exile,Stack"`},
	{name: "Nissa, Worldsoul Speaker"},
	{name: "One with the Multiverse", want: `MayPlay zones "Library,Hand"`},
	{name: "Primal Prayers", want: "MayPlayWithFlash"},
	{name: "Squee, the Immortal"},
}

func TestStaticZonesMayPlayForms(t *testing.T) {
	for _, r := range mayPlayFormsCarriers {
		c := lowerNamedCard(t, r.name)
		if r.want == "" {
			if len(c.Refusals) != 0 {
				t.Errorf("%s refusals = %v, want none", r.name, c.Refusals)
			}
			continue
		}
		if len(c.Refusals) == 0 {
			t.Errorf("%s lowers; want the refusal %q", r.name, r.want)
			continue
		}
		hit := false
		for _, rf := range c.Refusals {
			if strings.Contains(rf.Reason, `MayPlay zone "`) {
				t.Errorf("%s refusal %q still sits in the MayPlay zone class", r.name, rf.Reason)
			}
			hit = hit || strings.Contains(rf.Reason, r.want)
		}
		if !hit {
			t.Errorf("%s refusals %v: none carries %q", r.name, c.Refusals, r.want)
		}
	}
}

// TestStaticZonesMayPlayShapes lowers the slice's shapes with no paper
// carrier in the corpus: the non-granting free cast over the full zone list,
// the {0} alternative over the full list, the non-granting pay-life and
// pay-energy alternatives, and the two-zone self grant. Each must lower to
// the kernel flags its walk reads.
func TestStaticZonesMayPlayShapes(t *testing.T) {
	free := lowerScriptCard(t, "Free List Probe", `Name:Free List Probe
ManaCost:2 G
Types:Enchantment
S:Mode$ Continuous | Affected$ Card.Enchantment+YouCtrl+nonLand | MayPlay$ True | MayPlayWithoutManaCost$ True | MayPlayDontGrantZonePermissions$ True | AffectedZone$ Hand,Graveyard,Exile,Library,Command | Description$ You may cast enchantment spells without paying their mana costs.
Oracle:probe
`)
	if rf := free.Refusals; len(rf) != 0 {
		t.Fatalf("the free-list probe refused: %v", rf)
	}
	found := false
	for _, st := range free.Statics {
		if st.Kind == "MayPlayFrom" && st.Zone == int(zHand) && slices.Contains(st.Flags, "FreeOnly") && slices.Contains(st.Flags, "WithoutManaCost") {
			found = true
		}
	}
	if !found {
		t.Error("the free-list probe lowers to no non-granting free hand static")
	}

	zero := lowerScriptCard(t, "Zero Alt List Probe", `Name:Zero Alt List Probe
ManaCost:2 U
Types:Enchantment
S:Mode$ Continuous | Affected$ Card.nonLand+cmcLE2 | MayPlay$ True | MayPlayAltManaCost$ 0 | MayPlayLimit$ 1 | MayPlayDontGrantZonePermissions$ True | AffectedZone$ Hand,Graveyard,Library,Exile,Command | Description$ Once each turn, you may pay {0} rather than pay the mana cost for a spell you cast with mana value 2 or less.
Oracle:probe
`)
	if rf := zero.Refusals; len(rf) != 0 {
		t.Fatalf("the zero-alt probe refused: %v", rf)
	}
	found = false
	for _, st := range zero.Statics {
		if st.Kind == "MayPlayFrom" && slices.Contains(st.Flags, "MayZeroAlt") && slices.Contains(st.Flags, "OncePerTurn") {
			found = true
		}
	}
	if !found {
		t.Error("the zero-alt probe lowers to no non-granting alternative static")
	}

	life := lowerScriptCard(t, "Life Alt List Probe", `Name:Life Alt List Probe
ManaCost:3 B
Types:Enchantment Creature Demon
PT:4/4
S:Mode$ Continuous | Condition$ PlayerTurn | Affected$ Card.YouCtrl+nonLand | MayPlayLimit$ 1 | MayPlay$ True | MayPlayAltManaCost$ PayLife<ConvertedManaCost> | MayPlayDontGrantZonePermissions$ True | AffectedZone$ Hand,Graveyard,Exile,Library,Command | Description$ Once during each of your turns, you may cast a spell for life equal to its mana value instead of mana.
Oracle:probe
`)
	if rf := life.Refusals; len(rf) != 0 {
		t.Fatalf("the life-alt probe refused: %v", rf)
	}
	found = false
	for _, st := range life.Statics {
		if st.Kind == "MayPlayFrom" && slices.Contains(st.Flags, "MayZeroAlt") && slices.Contains(st.Flags, "MayPayLife") {
			found = true
		}
	}
	if !found {
		t.Error("the life-alt probe lowers to no non-granting pay-life static")
	}

	energy := lowerScriptCard(t, "Energy Alt List Probe", `Name:Energy Alt List Probe
ManaCost:3 G
Types:Legendary Creature Elf Druid
PT:3/3
S:Mode$ Continuous | Affected$ Card.YouCtrl+Permanent+nonLand | MayPlay$ True | MayPlayAltManaCost$ PayEnergy<8> | MayPlayDontGrantZonePermissions$ True | AffectedZone$ Hand,Graveyard,Exile,Library,Command | Description$ Pay eight energy instead of mana for your permanent spells.
Oracle:probe
`)
	if rf := energy.Refusals; len(rf) != 0 {
		t.Fatalf("the energy-alt probe refused: %v", rf)
	}
	found = false
	for _, st := range energy.Statics {
		if st.Kind == "MayPlayFrom" && slices.Contains(st.Flags, "MayZeroAlt") && slices.Contains(st.Flags, "MayPayEnergy") && st.Amount.Kind == vConst && st.Amount.N == 8 {
			found = true
		}
	}
	if !found {
		t.Error("the energy-alt probe lowers to no non-granting pay-energy static")
	}

	self := lowerScriptCard(t, "Self Two Zone Probe", `Name:Self Two Zone Probe
ManaCost:1 R R
Types:Legendary Creature Goblin
PT:2/1
S:Mode$ Continuous | Affected$ Card.Self | MayPlay$ True | AffectedZone$ Exile,Graveyard | EffectZone$ Exile,Graveyard | Description$ Cast CARDNAME from exile or from your graveyard.
Oracle:probe
`)
	if rf := self.Refusals; len(rf) != 0 {
		t.Fatalf("the two-zone self probe refused: %v", rf)
	}
	// one static per zone, each with its own FunctionsIn flag
	// (mayplay_selfzonepair_test.go): none carries both
	var selfGy, selfEx bool
	for _, st := range self.Statics {
		if st.Kind != "MayPlayFrom" {
			continue
		}
		if slices.Contains(st.Flags, "FunctionsInGraveyard") && slices.Contains(st.Flags, "FunctionsInExile") {
			t.Error("the two-zone self probe lowers to one static with both function flags")
		}
		selfGy = selfGy || slices.Contains(st.Flags, "FunctionsInGraveyard")
		selfEx = selfEx || slices.Contains(st.Flags, "FunctionsInExile")
	}
	if !selfGy || !selfEx {
		t.Errorf("the two-zone self probe lowers to no per-zone statics (graveyard %v, exile %v)", selfGy, selfEx)
	}

	// the non-granting free shape's Affected$ may carry the TopLibrary word
	// (One with the Multiverse's second line): the library-top read is dead
	// in a non-granting static and is stripped -- a filter word read in a static
	// matcher refuses (filter_spill.go fwAllowed) -- and an arm that carried
	// only it is dropped, which refuses when no arm survives
	toplib := lowerScriptCard(t, "Top Library Free Probe", `Name:Top Library Free Probe
ManaCost:2 U
Types:Enchantment
S:Mode$ Continuous | Affected$ Card.nonLand+TopLibrary+YouCtrl,Card.nonLand+YouOwn | MayPlay$ True | MayPlayWithoutManaCost$ True | MayPlayLimit$ 1 | MayPlayDontGrantZonePermissions$ True | AffectedZone$ Hand,Graveyard,Library,Exile,Command | Condition$ PlayerTurn | Description$ Once during each of your turns, you may cast a spell without paying its mana cost.
Oracle:probe
`)
	if rf := toplib.Refusals; len(rf) != 0 {
		t.Fatalf("the TopLibrary free probe refused: %v", rf)
	}
	// a partial zone list refuses for the free shape: the kernel's StFreeOnly
	// ride reaches every zone, so the list's missing zones would over-read
	partial := lowerScriptCard(t, "Partial List Free Probe", `Name:Partial List Free Probe
ManaCost:2 U
Types:Enchantment
S:Mode$ Continuous | Affected$ Card.nonLand+YouOwn | MayPlay$ True | MayPlayWithoutManaCost$ True | MayPlayDontGrantZonePermissions$ True | AffectedZone$ Library,Hand | Description$ You may cast spells without paying their mana costs.
Oracle:probe
`)
	hit := false
	for _, rf := range partial.Refusals {
		hit = hit || strings.Contains(rf.Reason, `MayPlay zones "Library,Hand"`)
	}
	if !hit {
		t.Errorf("the partial-list free probe refusals %v: none names the zone list", partial.Refusals)
	}
	only := lowerScriptCard(t, "Top Library Only Probe", `Name:Top Library Only Probe
ManaCost:2 U
Types:Enchantment
S:Mode$ Continuous | Affected$ Card.TopLibrary | MayPlay$ True | MayPlayWithoutManaCost$ True | MayPlayDontGrantZonePermissions$ True | AffectedZone$ Library,Hand | Description$ Once each turn, you may cast the top card of your library without paying its mana cost.
Oracle:probe
`)
	if len(only.Refusals) == 0 {
		t.Fatalf("the TopLibrary-only probe lowers; want a refusal")
	}
	hit = false
	for _, rf := range only.Refusals {
		hit = hit || strings.Contains(rf.Reason, "names no card off the library top")
	}
	if !hit {
		t.Errorf("the TopLibrary-only probe refusals %v: none names the dropped arm", only.Refusals)
	}
}
