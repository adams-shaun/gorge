package kgen

import (
	"fmt"
	"strings"
)

func init() { kwHandlers["Mutate"] = (*lowerCtx).lowerMutateKw }

// lowerMutateKw lowers K:Mutate:<cost> (CR 702.140a; gorge rules/mutate.go
// mutateCost and mutateTargetSA; kernel kw_mutate.go's AkMutate). The cost is
// a plain mana cost, hybrid pips included (gorge announces them as for any
// printed cost); an {X}, a ":" or "<" rider and any non-mana part refuse. The
// card must be a creature, and without another alternative hand cost beside
// it. The mutating cast targets a non-Human creature its controller owns.
func (lc *lowerCtx) lowerMutateKw(node, param string) {
	if err := lc.mutateKeyword(param); err != nil {
		lc.refuseNode(node, "%v", err)
	}
}

// mutateKeyword is lowerMutateKw's body, modelled on bestowKeyword
// (lower_bestow.go).
func (lc *lowerCtx) mutateKeyword(param string) error {
	mc, err := parseManaCost(param)
	if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.XX || mc.XXX || mc.None {
		return fmt.Errorf("mutate cost %q", param)
	}
	if lc.out.Types&tCreature == 0 {
		return fmt.Errorf("mutate on a non-creature")
	}
	if lc.out.Bestow != nil || lc.out.Warp != nil || lc.out.Sneak != nil || lc.out.Plot != nil || lc.out.Mutate != nil {
		return fmt.Errorf("mutate beside another alternative cost")
	}
	f, err := lc.filter("Creature.nonHuman+YouOwn")
	if err != nil {
		return err
	}
	lc.out.Mutate = &mc
	lc.out.MutateT = &lTarget{F: f, Zone: zBattlefield, Mode: -1, Min: cst(1), Max: cst(1)}
	return nil
}
