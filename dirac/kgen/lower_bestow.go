package kgen

import (
	"fmt"
	"strings"
)

// bestowKeyword lowers K:Bestow:<cost> (CR 702.103a; gorge rules/bestow.go).
// The cost is a plain mana cost, one {X} allowed (gorge announces it through
// the ordinary X machinery; Nyxborn Hydra's X G G); a ":" rider (Forge
// metadata), a hybrid symbol and any non-mana part refuse. The card must be
// an Enchantment Creature printed without a spell ability or Aura subtype,
// and without another alternative hand cost beside it.
func (lc *lowerCtx) bestowKeyword(param string) error {
	mc, err := parseManaCost(param)
	if err != nil || param == "" || strings.Contains(param, ":") || mc.XX || mc.None || len(mc.Hybrid) != 0 {
		return fmt.Errorf("bestow cost %q", param)
	}
	if lc.out.Types&tCreature == 0 || lc.out.Types&tEnchantment == 0 {
		return fmt.Errorf("bestow on a non-Enchantment-Creature")
	}
	if lc.out.Warp != nil || lc.out.Sneak != nil || lc.out.Plot != nil || lc.out.Bestow != nil {
		return fmt.Errorf("bestow beside another alternative cost")
	}
	f, err := lc.filter("Creature")
	if err != nil {
		return err
	}
	lc.out.Bestow = &mc
	lc.out.BestowT = &lTarget{F: f, Zone: zBattlefield, Mode: -1, Min: cst(1), Max: cst(1)}
	return nil
}
