package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// blgFeeDynFirstCols is the colour of card c's first mana ability when that is
// a fee-paid dynamic amount of one fixed colour (Cabal Coffers' "{2}, {T}: Add
// {B} for each Swamp", kop.FeeDynManaAb) and prep left it no autopay source
// (pc.manaAb < 0, manaCol kept). Gorge's intrinsic append (rules
// mana_activation.go, pay.ManaAbilitiesProduce) keys on the raw Produced$, so
// that colour's granted basic type adds no intrinsic ability beside it: Coffers
// under Realmwright's Swamp is one census gap, not a gap plus a plain {B}.
func blgFeeDynFirstCols(c *kop.CardDef, pc *pcard) kop.ColorMask {
	if pc.manaAb >= 0 || pc.manaCol == 0 {
		return 0
	}
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana {
			continue
		}
		if a.AFlags&kop.AfLatent == 0 && a.ManaColors == pc.manaCol && kop.FeeDynManaAb(a) {
			return pc.manaCol
		}
		return 0 // only the first mana ability sets pc.manaCol (prep.go)
	}
	return 0
}
