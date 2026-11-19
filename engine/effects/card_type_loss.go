package effects

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
)

// artifactTypeWords and enchantmentTypeWords are the CR 205.3g and 205.3h
// subtype lists plus the words non-tournament cards print beside those types.
var (
	artifactTypeWords = []string{
		"Attraction", "Blood", "Bobblehead", "Book", "Clue", "Communicator", "Contraption",
		"Equipment", "Food", "Fortification", "Gold", "Hat", "Heartwood",
		"Incubator", "Infinity", "Junk", "Key", "Lander", "Map", "Mutagen",
		"Powerstone", "Spacecraft", "Stone", "Treasure", "Vehicle", "Vibranium",
	}
	enchantmentTypeWords = []string{
		"Aura", "Background", "Cartouche", "Case", "Class", "Curse", "Entropy",
		"Plan", "Role", "Room", "Rune", "Saga", "Shard", "Shrine",
	}
)

// ArtifactTypeWord reports whether t is an artifact subtype.
func ArtifactTypeWord(t string) bool { return slices.Contains(artifactTypeWords, t) }

// EnchantmentTypeWord reports whether t is an enchantment subtype.
func EnchantmentTypeWord(t string) bool { return slices.Contains(enchantmentTypeWords, t) }

// SubtypesKept is which of an object's subtypes survive a type-strip effect
// (CR 205.1a). The creature, land, artifact and enchantment sets are the
// word-list vocabularies; the planeswalker set's words are the walker's own
// name (CR 205.3j), read from the corpus vocabulary on the strip side. No
// RemoveCardTypes$ carrier adds a planeswalker, battle, instant or sorcery
// type, so SubtypesKeptBy never keeps a walker name word -- only a strip
// naming the Planeswalker card type directly (a RemoveType$) does.
type SubtypesKept struct {
	creature, land, artifact, enchantment, planeswalker bool
}

// SubtypesKeptBy reads a type-removing effect's added types: a subtype stays
// when it is correlated with a card type the effect adds, unless the effect
// also adds a subtype from that set, which replaces the set.
func SubtypesKeptBy(added []string) SubtypesKept {
	k := correlatedSet(added)
	for _, t := range added {
		k.creature = k.creature && !CreatureTypeWords(t)
		k.land = k.land && !slices.Contains(chooseLandTypes, t)
		k.artifact = k.artifact && !ArtifactTypeWord(t)
		k.enchantment = k.enchantment && !EnchantmentTypeWord(t)
		k.planeswalker = k.planeswalker && !slices.Contains(choosePlaneswalkerTypes, t)
	}
	return k
}

// SubtypesCorrelatedWith reads the card type words named: which of the five
// subtype sets are correlated with them (CR 205.1a). The RemoveType$ strip's
// subtype follow-through (rules/chars) and SubtypesKeptBy share the one
// dispatch, so the correlated-set vocabulary lives in a single place.
func SubtypesCorrelatedWith(words []string) SubtypesKept {
	return correlatedSet(words)
}

// correlatedSet is the one card-type-word dispatch behind both readers. The
// dispatch goes through the catalog type mask (cards.TypeMaskOfName), so the
// word list it switches on is the one table that owns the card-type words.
func correlatedSet(words []string) SubtypesKept {
	var k SubtypesKept
	for _, t := range words {
		switch cards.TypeMaskOfName(t) {
		case cards.TypeCreature:
			k.creature = true
		case cards.TypeLand:
			k.land = true
		case cards.TypeArtifact:
			k.artifact = true
		case cards.TypeEnchantment:
			k.enchantment = true
		case cards.TypePlaneswalker:
			k.planeswalker = true
		}
	}
	return k
}

// Creature reports whether creature subtypes survive, the all-creature-types
// marker with them.
func (k SubtypesKept) Creature() bool { return k.creature }

// Land reports whether land subtypes participate.
func (k SubtypesKept) Land() bool { return k.land }

// Artifact reports whether artifact subtypes participate.
func (k SubtypesKept) Artifact() bool { return k.artifact }

// Enchantment reports whether enchantment subtypes participate.
func (k SubtypesKept) Enchantment() bool { return k.enchantment }

// Planeswalker reports whether planeswalker-name subtypes participate.
func (k SubtypesKept) Planeswalker() bool { return k.planeswalker }

// Has reports whether the subtype word survives.
func (k SubtypesKept) Has(word string) bool {
	return k.creature && CreatureTypeWords(word) ||
		k.land && slices.Contains(chooseLandTypes, word) ||
		k.artifact && ArtifactTypeWord(word) ||
		k.enchantment && EnchantmentTypeWord(word) ||
		k.planeswalker && slices.Contains(choosePlaneswalkerTypes, word)
}
