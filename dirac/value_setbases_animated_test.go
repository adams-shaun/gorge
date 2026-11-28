package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSetBattlefieldAnimatedLand: "Valid Creature.YouCtrl" (SetBattlefield ∩
// a Creature+you-control filter) names a land of the controller's that an
// Animate record made a creature until end of turn, and only while it is one
// (CR 613.1d: a type-changing effect applies in layer 4, so the filter reads
// the effective type, not the printed Land). This is the TestAtomDiffSetBases
// red of whole-oracle run 46156795316: o73 was Soulstone Sanctuary, animated
// by its own ability; the kernel named it and gorge's battlefieldValidTargets
// sweep read the printed face until #6054 bound the layer tables.
func TestSetBattlefieldAnimatedLand(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Filters = append(slices.Clip(tbl.Filters), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	youCreature := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Overrides = append(slices.Clip(tbl.Overrides), kop.Override{HasPT: true, SetP: 4, SetT: 3, AddTypes: kop.TCreature})
	animate := len(tbl.Overrides) - 1
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s := sc.s
	bear := sc.perm(vanilla.GrizzlyBears, 0)
	land := sc.perm(vanilla.Plains, 0)
	lions := sc.perm(vanilla.SavannahLions, 1)
	if s.objs[land].card != vanilla.Plains || tbl.Cards[vanilla.Plains].Types&kop.TCreature != 0 || !sc.onBF(land) {
		t.Fatal("precondition: the controller's Plains is on the battlefield and is not printed a creature")
	}
	eval := func() []TCode {
		e := stackEntry{kind: seTrigger, obj: bear, srcVer: s.objs[bear].ver, card: vanilla.GrizzlyBears, ctrl: 0, trigObj: NoObj}
		for i := range e.tgt {
			e.tgt[i] = NoTarget
		}
		s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
		v := s.evalSetExpr(sc.k, kop.SetExpr{setOp1(kop.SetBattlefield, 0), setOp1(kop.SetAndFilter, uint16(youCreature))}, setCtx{ev: s.ctxEntry(&s.res.e)})
		var list [MaxObj + MaxSeats]TCode
		var vers [MaxObj + MaxSeats]uint8
		n := s.setMembers(&v, 0, &list, &vers)
		return slices.Clone(list[:n])
	}
	B, L := TCode(bear), TCode(land)
	setWant(t, "Creature.YouCtrl before the animation", eval(), B)

	applyTestOverride(t, sc, land, animate)
	if !s.isCreature(sc.k, sc.slot(land)) {
		t.Fatal("precondition: the animated Plains is a creature")
	}
	setWant(t, "Creature.YouCtrl with the land animated", eval(), B, L)
	if slices.Contains(eval(), TCode(lions)) {
		t.Error("the opponent's creature joined Creature.YouCtrl")
	}
}
