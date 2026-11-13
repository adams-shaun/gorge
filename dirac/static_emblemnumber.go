package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The frozen chosen number of an Effect-delivered emblem ReduceCost static
// gorge's effects/effect.go evaluates SetChosenNumber$
// ONCE, as the Effect resolves, against that resolution's context, and the
// registered Mode$ ReduceCost static carries the answer as
// state.ContinuousEffect.ChosenNumber; the cost path reads
// Count$ChosenNumber from that frozen field. Life lost or gained later in
// the turn therefore does not raise the discount. The kernel freezes the
// value in State.g.embN at OpCreateEmblem and reads it back through
// embNumAt under kop.StEmbNum.

// freezeEmbNum evaluates the CreateEmblem instruction's A in the resolving
// Effect's context and freezes it as the emblem's chosen number, clamped to
// 0..255 (gorge's int32 is signed; a byte above every generic cost is an
// exact discount).
//
//go:noinline
func (s *State) freezeEmbNum(k *prep, v kop.ValueRef, ctx evalCtx) uint8 {
	n := s.evalV(k, v, ctx)
	switch {
	case n < 0:
		return 0
	case n > 255:
		return 255
	}
	return uint8(n)
}

// embNumAt is the frozen number of the emblem statSrc.emb names
// (player<<4 | index+1, 0 = not an emblem).
//
//go:noinline
func (s *State) embNumAt(emb uint8) uint8 {
	if emb == 0 {
		return 0
	}
	p, i := emb>>4, int(emb&15)-1
	if p >= MaxSeats || i < 0 || i >= int(s.g.nemb[p]) {
		return 0
	}
	return uint8(s.embNum(p, i))
}

// DebugEmbNumLive is the mutation hook: the read
// re-evaluates the number the emblem's CreateEmblem instruction carried
// against the LIVE state instead of the frozen byte. It must make
// TestEmbnumMutation diverge; the frozen read is the shipped behaviour.
var DebugEmbNumLive bool

// embNumLive re-evaluates the creating ability's value in the live state:
// the emblem record keeps the creating card (State.g.embC), so the
// CreateEmblem instruction with this emblem index is found in that card's
// programs. An instruction not found (or no creating card) falls back to
// the frozen byte.
//
//go:noinline
func (s *State) embNumLive(k *prep, emb uint8) uint8 {
	if emb == 0 {
		return 0
	}
	p, i := emb>>4, int(emb&15)-1
	if p >= MaxSeats || i < 0 || i >= int(s.g.nemb[p]) {
		return 0
	}
	c, idx := s.embCard(p, i), s.embIdx(p, i)-1
	if int(c) >= len(k.t.Cards) {
		return s.embNumAt(emb)
	}
	cd := &k.t.Cards[c]
	for ai := range cd.Abilities {
		prog := cd.Abilities[ai].Prog
		for ii := range prog {
			in := &prog[ii]
			if in.Op == kop.OpCreateEmblem && in.C == int16(idx) {
				return s.freezeEmbNum(k, in.A, evalCtx{ctrl: p})
			}
		}
	}
	return s.embNumAt(emb)
}
