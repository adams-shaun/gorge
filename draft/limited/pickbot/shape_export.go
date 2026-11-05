package pickbot

import "github.com/adams-shaun/gorge/engine/cards"

// classNames spells classOf's card classes for callers outside the builder.
var classNames = [numClasses]string{
	classOther:        "other",
	classCreature:     "creature",
	classInstant:      "instant",
	classSorcery:      "sorcery",
	classEnchantment:  "enchantment",
	classArtifact:     "artifact",
	classPlaneswalker: "planeswalker",
	classBattle:       "battle",
}

// ClassName names the spell type classOf assigns a face: creature, instant,
// sorcery, enchantment, artifact, planeswalker, battle or other.
func ClassName(f *cards.Face) string {
	return classNames[classOf(f)]
}

// CurveBucket maps a mana value to the builder's curve bucket: <=1, 2, 3, 4,
// 5, 6+.
func CurveBucket(mv int) int {
	return curveBucket(mv)
}

// LandColours returns the OR of the colours a land's mana abilities can make,
// the same reading Balanced uses to choose its nonbasic lands.
func LandColours(f *cards.Face) uint8 {
	return producedColours(f)
}
