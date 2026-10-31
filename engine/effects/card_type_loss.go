package effects

import "slices"

// artifactTypeWords and enchantmentTypeWords are the CR 205.3g and 205.3h
// subtype lists plus the words non-tournament cards print beside those types.
var (
	artifactTypeWords = []string{
		"Attraction", "Blood", "Bobblehead", "Book", "Clue", "Contraption",
		"Equipment", "Food", "Fortification", "Gold", "Hat", "Heartwood",
		"Incubator", "Infinity", "Junk", "Key", "Lander", "Map", "Mutagen",
		"Powerstone", "Spacecraft", "Stone", "Treasure", "Vehicle", "Vibranium",
	}
	enchantmentTypeWords = []string{
		"Aura", "Background", "Cartouche", "Case", "Class", "Curse", "Entropy",
		"Plan", "Role", "Room", "Rune", "Saga", "Shard", "Shrine",
	}
)

// SubtypesKept is which of an object's subtypes survive an effect that removes
// its card types (CR 205.1a). No such effect adds a planeswalker, battle,
// instant or sorcery type, so those types' subtypes never survive.
type SubtypesKept struct {
	creature, land, artifact, enchantment bool
}

// SubtypesKeptBy reads a type-removing effect's added types: a subtype stays
// when it is correlated with a card type the effect adds, unless the effect
// also adds a subtype from that set, which replaces the set.
func SubtypesKeptBy(added []string) SubtypesKept {
	var k SubtypesKept
	for _, t := range added {
		switch t {
		case "Creature":
			k.creature = true
		case "Land":
			k.land = true
		case "Artifact":
			k.artifact = true
		case "Enchantment":
			k.enchantment = true
		}
	}
	for _, t := range added {
		k.creature = k.creature && !CreatureTypeWords(t)
		k.land = k.land && !slices.Contains(chooseLandTypes, t)
		k.artifact = k.artifact && !slices.Contains(artifactTypeWords, t)
		k.enchantment = k.enchantment && !slices.Contains(enchantmentTypeWords, t)
	}
	return k
}

// Creature reports whether creature subtypes survive, the all-creature-types
// marker with them.
func (k SubtypesKept) Creature() bool { return k.creature }

// Has reports whether the subtype word survives.
func (k SubtypesKept) Has(word string) bool {
	return k.creature && CreatureTypeWords(word) ||
		k.land && slices.Contains(chooseLandTypes, word) ||
		k.artifact && slices.Contains(artifactTypeWords, word) ||
		k.enchantment && slices.Contains(enchantmentTypeWords, word)
}
