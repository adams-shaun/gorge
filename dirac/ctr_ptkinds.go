package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 122.1a: a +X/+Y counter gives its permanent +X/+Y and a -X/-Y counter
// gives it -X/-Y. The +1/+1 and -1/-1 kinds are folded into ptGen's base sum
// and the -2/-1 kind has its own gate (ctr_hone.go prepM2M1); the kinds below
// are the rest of the corpus's P/T counters. gorge reads all of them through
// state.CounterPTDelta (the first pair is power, the second toughness; P is a
// plus, M a minus).

// ptKind is one P/T counter kind and the power and toughness it adds per
// counter.
type ptKind struct {
	kind   kop.CounterKind
	dp, dt int
}

var ptKinds = [...]ptKind{
	{kop.CtrP1P0, +1, 0},
	{kop.CtrP0P1, 0, +1},
	{kop.CtrP1P2, +1, +2},
	{kop.CtrP2P2, +2, +2},
	{kop.CtrM0M1, 0, -1},
	{kop.CtrM0M2, 0, -2},
	{kop.CtrM2M2, -2, -2},
}

// usePTKinds holds for a table some program of which can place one of the
// ptKinds: ptGen and the toughness SBA read the counters only then.
var usePTKinds = newGenUse(func(t *kop.Table) bool {
	for i := range ptKinds {
		if tablePlacesCtr(t, ptKinds[i].kind) {
			return true
		}
	}
	return false
})

// ptKindsDelta is the power and toughness the ptKinds counters on p add. The
// -0/-1 kind is skipped when the table's mana-counter cost already has ptGen
// read it (mana_countercost.go).
func (s *State) ptKindsDelta(k *prep, p *perm) (dp, dt int) {
	for i := range ptKinds {
		pk := &ptKinds[i]
		if pk.kind == kop.CtrM0M1 && k.ctrMana {
			continue
		}
		if n := p.ctrN(s, int(pk.kind)); n != 0 {
			dp += pk.dp * n
			dt += pk.dt * n
		}
	}
	return dp, dt
}

// sbaPTKinds is CR 704.5f for the creatures whose toughness -2/-1 counters or
// the toughness-lowering ptKinds counters (-0/-2, -2/-2, and -0/-1 where mana_countercost.go
// does not already check it) can drop to 0.
func (s *State) sbaPTKinds(k *prep, die, gy *slotMask) {
	if k.gen == nil {
		return
	}
	if k.gen.anyM2M1 {
		s.sbaCtrTough(k, kop.CtrM2M1, die, gy) // -2/-1 counters (ctr_hone.go prepM2M1)
	}
	if !k.gen.uses.has(usePTKinds) {
		return
	}
	for i := range ptKinds {
		if pk := &ptKinds[i]; pk.dt < 0 && !(pk.kind == kop.CtrM0M1 && k.ctrMana) {
			s.sbaCtrTough(k, pk.kind, die, gy)
		}
	}
}
