package pickbot

import "github.com/adams-shaun/gorge/engine/cards"

// cardClass is the single spell type a card fills for the balanced builder.
// A card with several types takes the first that holds, so an artifact
// creature fills a creature slot, an enchantment artifact fills the rarer
// enchantment slot, and a Spacecraft or Vehicle is a noncreature artifact.
type cardClass uint8

const (
	// classOther is the zero value: a card with no facts, or one whose
	// types the builder does not track.
	classOther cardClass = iota
	classCreature
	classInstant
	classSorcery
	classEnchantment
	classArtifact
	classPlaneswalker
	classBattle

	numClasses = 8
)

// classOf returns the class a face fills, or classOther when none applies.
func classOf(f *cards.Face) cardClass {
	if f == nil {
		return classOther
	}
	switch {
	case f.IsCreature():
		return classCreature
	case f.IsInstant():
		return classInstant
	case f.IsSorcery():
		return classSorcery
	case f.IsPlaneswalker():
		return classPlaneswalker
	case f.IsBattle():
		return classBattle
	case f.IsEnchantment():
		return classEnchantment
	case f.IsArtifact():
		return classArtifact
	}
	return classOther
}

// curveBucket maps a mana value to one of six curve buckets: <=1, 2, 3, 4,
// 5, and 6+.
func curveBucket(mv int) int {
	switch {
	case mv <= 1:
		return 0
	case mv >= 6:
		return 5
	default:
		return mv - 1
	}
}

// DeckShape is the balanced builder's target: a curve, a creature band and a
// minimum count of each spell type. A zero Shape means DefaultDeckShape.
type DeckShape struct {
	// MainSize is how many non-land cards the builder plays. 23 is the
	// limited standard: 17 lands in a 40-card deck (= Tuning.MainSize).
	MainSize int
	// CurveMin and CurveMax bound the cards per curve bucket (<=1, 2, 3,
	// 4, 5, 6+). The bands sit around the measured means and cut the
	// tails; 6+ max 2 tightens Best23's 3 because a 17-land deck casts a
	// sixth-mana card on turn 6 only about half the time. The minima sum
	// to 13 <= 23 <= the maxima's 28.
	CurveMin [6]int
	CurveMax [6]int
	// CreatureMin and CreatureMax are the creature band. About 60% of
	// TRK's nonland commons are creatures, so 14 is the floor most pools
	// meet; 18 leaves 5 slots for the type quotas.
	CreatureMin int
	CreatureMax int
	// TypeMin is the minimum count of each class. Two of each of instant,
	// sorcery, enchantment and artifact is "more than just 1 of each type
	// if available"; 4 x 2 = 8 plus CreatureMin 14 = 22 <= 23, so the
	// quotas and the creature floor never conflict. Planeswalker and
	// battle are rare types, where one is a real deck's norm.
	TypeMin [numClasses]int
	// QuotaFloor is the lowest rating a quota card may have. -10 on the
	// builder's centred rating scale ((gih_wr - 0.55) * 100) is a 0.45 GIH
	// card (or -3 heuristic points), so a quota never forces an
	// unplayable card in.
	QuotaFloor float64
	// SplashMax caps a splash at three cards: the conventional ceiling,
	// past which the mana is unreliable.
	SplashMax int
	// SplashSources is the fewest sources (duals that make it plus
	// basics) the splash colour gets, the usual minimum for a one-pip
	// splash card.
	SplashSources int
	// DualMax caps the nonbasic lands that make only the deck's colours:
	// every common dual in FDN and TRK enters tapped, and more than four
	// taplands slow the curve.
	DualMax int
	// ThinMainSize is the spell count of a deck whose in-pair plus splash
	// cards fall short of MainSize: 22 spells and 18 lands, because an
	// eighteenth land beats a card with no sources. Zero or at least
	// MainSize turns the rule off.
	ThinMainSize int
}

// DefaultDeckShape is the balanced builder's default target.
var DefaultDeckShape = DeckShape{
	MainSize:    23,
	CurveMin:    [6]int{0, 5, 4, 3, 1, 0},
	CurveMax:    [6]int{3, 8, 7, 5, 3, 2},
	CreatureMin: 14,
	CreatureMax: 18,
	TypeMin: [numClasses]int{
		classOther:        0,
		classCreature:     0,
		classInstant:      2,
		classSorcery:      2,
		classEnchantment:  2,
		classArtifact:     2,
		classPlaneswalker: 1,
		classBattle:       1,
	},
	QuotaFloor:    -10,
	SplashMax:     3,
	SplashSources: 3,
	DualMax:       4,
	ThinMainSize:  22,
}
