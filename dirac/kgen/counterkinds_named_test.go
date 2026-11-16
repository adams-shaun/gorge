package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The named counter kinds (KI, PRESSURE, BOUNTY, ...) have no rules meaning
// (CR 122.1): a card whose only blocker was such a kind lowers once the kind
// is registered in counterKinds. The Arcane-filter Baku cards and Omo are left out: lowerCard over a
// bare pool does not resolve the "Spirit,Arcane" SpellCast filter head or
// AddType AllNonBasicLandType (the full build resolves the first).
func TestNamedCounterKindsLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{
		"A-Spell Satchel",
		"Aligned Heart",
		"Arixmethes, Slumbering Isle",
		"Assemble the Legion",
		"Aurification",
		"Azor's Elocutors",
		"Bloodletter Quill",
		"Bloodthirsty Ogre",
		"Bogardan Phoenix",
		"Boon of the Spirit Realm",
		"Bounty Hunter",
		"Bounty of the Luxa",
		"Briber's Purse",
		"Cephalid Vandal",
		"Charging Cinderhorn",
		"Charitable Levy",
		"Chevill, Bane of Monsters",
		"Chitterspitter",
		"Communal Brewing",
		"Coral Reef",
		"Crescendo of War",
		"Crowded Crypt",
		"Dawn of a New Age",
		"Deadly Designs",
		"Dragon's Hoard",
		"Etrata, the Silencer",
		"Exploding Barrel",
		"Festering Wound",
		"Five-Alarm Fire",
		"Genestealer Patriarch",
		"Grimoire of the Dead",
		"Grolnok, the Omnivore",
		"Gwafa Hazid, Profiteer",
		"Helix Pinnacle",
		"Hellion Crucible",
		"Hoofprints of the Stag",
		"Icatian Javelineers",
		"Icatian Moneychanger",
		"Iceberg",
		"Instrument of the Bards",
		"Lattice Library",
		"Lotus Blossom",
		"Ludevic's Test Subject",
		"Luxa River Shrine",
		"Magma Mine",
		"Midnight Clock",
		"Moth Herb Elixir",
		"Oblivion Stone",
		"Ominous Seas",
		"Oracle's Vault",
		"Osai Vultures",
		"Palantír of Orthanc",
		"Pious Kitsune",
		"Prism Array",
		"Prize Pig",
		"Quarantine Field",
		"Rakdos Riteknife",
		"Replicating Ring",
		"Roc Hatchling",
		"Scavenging Ghoul",
		"Serrated Arrows",
		"Sludge Monster",
		"Smokestack",
		"Smoldering Egg",
		"Soulcatchers' Aerie",
		"Spell Satchel",
		"Sphere of Annihilation",
		"Sporogenesis",
		"Sunset Pyramid",
		"Termination Facilitator",
		"The Darkhold",
		"The Magic Mirror",
		"Thing in the Ice",
		"Toxrill, the Corrosive",
		"Triad of Fates",
		"Triassic Egg",
		"Ursine Fylgja",
		"Venarian Gold",
		"Voodoo Doll",
		"Weatherlight Compleated",
		"Wedding Announcement",
		"Withering Hex",
		"Wrath of Sod",
	}
	if len(names) != 83 {
		t.Fatalf("card list has %d names, want 83", len(names))
	}
	for _, name := range names {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Errorf("%s not in the corpus", name)
			continue
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
	}
}
