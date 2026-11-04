package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugTrigLKIOff turns matchTrigLKI off, a departed trigger object read as
// the card where it now is: the lockstep mutation tests set it.
var DebugTrigLKIOff bool

// matchTrigLKI matches the resolving trigger's object against f as it last
// existed on the battlefield, once it has moved since the entry's snapshot
// (settleFollowed retakes an entry trigger's snapshot as the object leaves):
// "if it has greater power than Ensign Kim" read off a creature that died in
// response is its last known power, every continuous effect applied (CR 603.4,
// 603.10, 608.2h; gorge evalRefProperty's Valid over the trigger's LKI). As in
// matchTargetLKI, power, toughness, tapped and attacking come from the
// snapshot and the rest from the card. ok is false when the object has not
// moved or no snapshot names it.
//
//go:noinline
func (s *State) matchTrigLKI(k *prep, f kop.FilterID, o Obj, ctx evalCtx) (m, ok bool) {
	if o == NoObj || ctx.e == nil || ctx.e.li == 0 || DebugTrigLKIOff {
		return false, false
	}
	l := s.lk(ctx.e.li)
	ob := &s.objs[o]
	if l.obj != o || ob.ver == l.ver {
		return false, false
	}
	tli := s.g.tli
	s.g.tli = ctx.e.li | tliOn
	m = s.matchCardGen(k, f, ob.card, ob.owner(), ctx.ctrl, o)
	s.g.tli = tli
	return m, true
}
