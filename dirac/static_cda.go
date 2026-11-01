package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The zone-aware characteristic reader. Cold code: the filter arms in
// matchCardOne only call here behind one predictable branch.

// cardCDAPT is card c's power/toughness from its characteristic-defining
// ability (CR 604.3), which works in every zone (CR 208.2a): object o
// (NoObj if unknown), owned by owner (its controller on the stack). On the
// battlefield it is the permanent's base P/T; elsewhere the CDA's value
// evaluated now with the owner as its controller.
func (s *State) cardCDAPT(k *prep, c CardID, owner uint8, o Obj) (int, int) {
	if o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
		return s.basePTGen(k, s.objs[o].slot)
	}
	a := &k.t.Cards[c].Abilities[k.gen.cdaAb(c)]
	ctx := evalCtx{src: o, srcSl: NoSlot, ctrl: owner}
	return s.evalV(k, a.Static.VA, ctx), s.evalV(k, a.Static.VB, ctx)
}

// stackXMV is what object o's {X} symbols (nx of them) add to its mana
// value: on the stack the value chosen for X (CR 202.3e, gorge),
// elsewhere 0.
func (s *State) stackXMV(o Obj, nx uint8) int {
	if s.objs[o].zone != kop.ZoneStack {
		return 0
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
			return int(e.x) * int(nx)
		}
	}
	return 0
}
