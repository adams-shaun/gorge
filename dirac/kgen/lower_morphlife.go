package kgen

import (
	"strconv"
	"strings"
)

// morphTurnUpCost parses a Morph/Megamorph/Disguise turn-face-up cost: a
// mana cost, or a life payment alone (K:Morph:PayLife<5>, Zombie Cutthroat;
// CR 702.37e, 119.4), whose mana is {0} and whose life rides
// CardDef.MorphLife (dirac/face_morphlife.go). gorge's turn-up prices and
// pays the life with the mana (rules morph_turnup.go: CostPayableSpecial,
// PayManaSpecial). A variable or mixed life cost is an error.
func morphTurnUpCost(cs string) (mc manaCost, life uint8, err error) {
	v, ok := strings.CutPrefix(cs, "PayLife<")
	if !ok {
		mc, err = parseManaCost(cs)
		return mc, 0, err
	}
	v, ok = strings.CutSuffix(v, ">")
	n, aerr := strconv.Atoi(v)
	if !ok || aerr != nil || n < 1 || n > 255 {
		return manaCost{}, 0, refuse("morph life cost %q", cs)
	}
	return manaCost{}, uint8(n), nil
}
