package kgen

import "strings"

// "can't be blocked by creatures with power 2 or less /
// 3 or greater this turn" (Cavern Stomper, Cheeky House-Mouse): an Effect
// CantBlockBy whose ValidBlocker$ is exactly Creature.power<op><N>. gorge
// matches ValidBlocker$ against the would-be blocker at block declaration, so
// the kernel's ExHaste record (dirac/static_cantblockexcept.go) carries the
// comparison and reads the blocker's current power.
//
// cantBlockPowerC parses expr through the pool's filter and returns the
// ExHaste C bits (bit 0 = the power form, bits 1-3 the comparison, bits 4-7
// the constant); any other filter shape refuses by name.
func (lc *lowerCtx) cantBlockPowerC(expr string) (int, error) {
	f, err := lc.filter(expr)
	if err != nil {
		return 0, err
	}
	if f.Players != 0 || len(f.Clauses) != 1 {
		return 0, refuse("Effect CantBlockBy ValidBlocker %q", expr)
	}
	c := f.Clauses[0]
	plain := lClause{Types: tCreature, CmpStat: cmpPower, CmpOp: c.CmpOp, CmpN: c.CmpN}
	if c != plain || c.CmpOp < opGE || c.CmpOp > opNE || c.CmpN < 0 || c.CmpN > 15 {
		return 0, refuse("Effect CantBlockBy ValidBlocker %q", expr)
	}
	return 1 | int(c.CmpOp)<<1 | c.CmpN<<4, nil
}

// cantBlockValidBlocker reads an Effect CantBlockBy ValidBlocker$: the C bits
// of the ExHaste instr (0 for Creature.withoutHaste, Agility Bobblehead).
func (lc *lowerCtx) cantBlockValidBlocker(vb string) (int, error) {
	if strings.TrimSpace(vb) == "Creature.withoutHaste" {
		return 0, nil
	}
	return lc.cantBlockPowerC(strings.TrimSpace(vb))
}
