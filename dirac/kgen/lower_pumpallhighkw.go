package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kwGrantScanned are the keyword bits a kernel prepass finds only in an
// OpGrantKeyword (grantsKw, the stall classes, the flashback-grant planes):
// an ExtGrantKwAll carrying one would go unseen, so those masks refuse.
const kwGrantScanned = kop.KwWither | kop.KwFlashbackOwnCost | kop.KwPreventCombatDamageFrom

// pumpAllHighKw retargets a PumpAll whose KW$ mask has a bit above 15
// (OpPumpAll's C holds only the low 16) to ExtGrantKwAll, which carries the
// full mask: gorge effPumpAll grants every keyword to each matching permanent
// until end of turn (Dirge of Dread's fear, Tortoise Formation's shroud,
// Goblin War Cry's can't block). Only the keyword-only shape lowers: with a
// NumAtt$/NumDef$ pump the grant would split from the pump into a second
// sweep, and with a ValidTgts$ player scope the ext has no scoped walk; both
// refuse, as does a mask naming a kwGrantScanned bit. A mask within bit 15
// is left to OpPumpAll.
func pumpAllHighKw(in *lInstr, pumps bool) error {
	if in.CS == "" {
		return nil
	}
	m, err := kwMask(strings.Split(in.CS, "&"))
	if err != nil {
		return err
	}
	switch {
	case m>>16 == 0:
		return nil
	case pumps:
		return refuse("PumpAll keyword above bit 15 beside NumAtt$/NumDef$")
	case in.Obj.Kind != rNone:
		return refuse("PumpAll keyword above bit 15 with a ValidTgts$ player scope")
	case m&kwGrantScanned != 0:
		return refuse("PumpAll KW$ %q: a keyword the kernel prepass reads only off OpGrantKeyword", in.CS)
	}
	in.Op = "GrantKwAll"
	in.Note = "keywords above bit 15: the full mask in C | B.N<<16"
	return nil
}

// kwMaskOperands splits the KW$ list cs into OpGrantKeyword's operand pair:
// C the low 16 bits, B.N the high 16 (ExtGrantKwAll reads them the same way).
func kwMaskOperands(cs string) (int16, kop.ValueRef, error) {
	m, err := kwMask(strings.Split(cs, "&"))
	if err != nil {
		return 0, kop.ValueRef{}, err
	}
	return int16(uint16(m)), kop.ValueRef{K: kop.VConst, N: int16(uint16(m >> 16))}, nil
}
