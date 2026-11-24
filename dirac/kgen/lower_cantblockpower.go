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
func (lc *lowerCtx) cantBlockValidBlocker(vb string) (int, *lFilter, error) {
	vb = strings.TrimSpace(vb)
	if vb == "Creature.withoutHaste" {
		return 0, nil, nil
	}
	if c, err := lc.cantBlockPowerC(vb); err == nil {
		return c, nil, nil
	}
	f, err := lc.cantBlockBlockerFilter(vb)
	return 0, f, err
}

// cantBlockBlockerFilter is the general ValidBlocker$ form (Tower of Coireall
// "Creature.Wall", Joven's Tools "Creature.nonWall", Firefright Mage
// "Creature.nonArtifact+nonRed", Doorman "Creature.powerLE2,Creature.Wall"):
// the blockers the filter matches can't block. Every alternative must read
// only the blocker's own types, subtypes, colours and power/toughness-style
// compare: a controller, owner, name or SVar-bounded clause relates to the
// effect's source or players, which the record does not keep.
func (lc *lowerCtx) cantBlockBlockerFilter(vb string) (*lFilter, error) {
	f, err := lc.filter(vb)
	if err != nil {
		return nil, err
	}
	if f.Players != 0 || f.Each || f.PickSumMax != nil || f.PickDistinctPower || len(f.Clauses) == 0 {
		return nil, refuse("Effect CantBlockBy ValidBlocker %q", vb)
	}
	for _, c := range f.Clauses {
		if c.Types&tCreature == 0 {
			return nil, refuse("Effect CantBlockBy ValidBlocker %q", vb)
		}
		plain := lClause{Types: c.Types, NotTypes: c.NotTypes, Subtype: c.Subtype, NotSubtype: c.NotSubtype,
			Colors: c.Colors, NotColors: c.NotColors}
		if c.CmpStat == cmpPower {
			plain.CmpStat, plain.CmpOp, plain.CmpN = c.CmpStat, c.CmpOp, c.CmpN
		}
		if c != plain {
			return nil, refuse("Effect CantBlockBy ValidBlocker %q", vb)
		}
	}
	return f, nil
}
