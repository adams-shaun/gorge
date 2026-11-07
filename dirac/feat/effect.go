package feat

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Effect classes of EffectSpec.Class.
const (
	EffKill = iota
	EffDamage
	EffCounter
	EffBounce
	EffPump
	EffDebuff
	EffOther
)

// EffectSpec is a pool-load fact about what a card's spell ability does,
// read from gorge's IR. It is an approximation: only the first spell ability
// is read; a SubAbility$ chain and modal modes are not.
type EffectSpec struct {
	Class     uint8 // EffKill .. EffOther
	Amount    int16 // the integer amount; -1 = X; -2 = unknown or absent
	NumAtt    int8  // Pump NumAtt, clamped -9..9
	NumDef    int8  // Pump NumDef, clamped -9..9
	Exile     bool  // a kill that exiles: ignores indestructible
	TgtPlayer bool  // ValidTgts names Player, Opponent or Any
	TgtCreat  bool  // ValidTgts names Creature or Any
	CostX     bool  // the mana cost holds X
}

// StrengthBucket buckets a printed strength: 0 for s <= 3, 1 for 4..7,
// 2 for 8..12, 3 for >= 13.
func StrengthBucket(s int32) uint8 {
	switch {
	case s <= 3:
		return 0
	case s <= 7:
		return 1
	case s <= 12:
		return 2
	}
	return 3
}

const strengthKw = kop.KwFlying | kop.KwTrample | kop.KwMenace | kop.KwDeathtouch |
	kop.KwLifelink | kop.KwFirstStrike | kop.KwDoubleStrike | kop.KwHexproof |
	kop.KwIndestructible | kop.KwWard

// printedStrength scores a card from its printed face: a creature
// max(P,0)+max(T,0)+2*min(kw,3)+mv/2, a planeswalker 2*mv+2, another
// nonland card 2*mv, a land 1.
func printedStrength(sp CardSpec, kw kop.KwMask, mv int32) int32 {
	fold, _ := typeMasks(sp.Types)
	switch {
	case fold&tbCreature != 0:
		n := int32(bits.OnesCount32(uint32(kw & strengthKw)))
		return max(sp.Power, 0) + max(sp.Toughness, 0) + 2*min(n, 3) + mv/2
	case fold&tbLand != 0:
		return 1
	case fold&tbPlaneswalker != 0:
		return 2*mv + 2
	}
	return 2 * mv
}
