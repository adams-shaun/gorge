package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Layer 1 copy exceptions (CR 707.9b).
//
// A copy effect's exceptions ("except it's 1/1", "except it has haste",
// "except it's a Nightmare in addition") are part of the copy's copiable
// values. gorge folds them into the copy's layer-1 basis face (events
// copy_except.go foldCopyExcept), so they sit under every later layer and
// a copy of the copy inherits them (CR 707.3). Here they are an exception
// override (a Table.Overrides record named by OpCopyAsTokenEx's C and
// OpBecomeCopy's C>>1) kept as fxCopyExc records in the effect pool, one
// per exception applied: a copy of a copy carries its template's records
// first, then its own, so pool order is the order CR 707.9b applies them.
//
// Readers, each behind the slot's fxM bit and a prep flag (tested inline at
// the call site; the scans are out of line):
//   - P/T (cpxPT): the base P/T after the 7a CDA and before every layer-7b
//     setter, so a one-shot "becomes 4/4 until end of turn" on an
//     Offspring token returns it to 1/1, not to its printed P/T. A set P/T
//     exception also removes a P/T-defining CDA (CR 707.9d), which the
//     order gives for free.
//   - Subtypes (cpxSub): ORed into the printed subtypes (layer 1).
//   - Keywords: settled eagerly into perm.ovr.kw at entry, where every
//     keyword read already looks, and settled again when an until-end-of-
//     turn animation's cleanup clears ovr.kw.
//
// The records retire genState.addSub and the pfCpHaste / pfCpOneOne perm
// bits. Not modelled: a copy of a copy whose template left the battlefield
// before the copy was made inherits nothing (no last known exceptions).

// fxCopyExc records hold the override index+1 in r.idx; r.ts is 0 (layer 1
// is not ordered by timestamp).

// copyExcAdd appends slot sl's exceptions: the template's (at src, when it
// is on the battlefield) and then the op's own override idx (index+1, 0:
// none). The keywords are settled into the permanent's override.
//
//go:noinline
func (s *State) copyExcAdd(k *prep, sl, src Slot, idx uint8) {
	if DebugCopyRidersStats != nil {
		s.copyRidersCount(k, src, idx)
	}
	if src != NoSlot && s.fxM.has(src) {
		for i := 0; i < int(s.g.nfx); i++ {
			if r := s.g.fx[i]; r.sl == src && r.kind == fxCopyExc {
				s.copyExcPush(sl, r.r.idx)
			}
		}
	}
	if idx != 0 {
		s.copyExcPush(sl, idx)
	}
	if s.fxM.has(sl) {
		s.perms[sl].ovr.kw |= s.copyExcKw(k, sl)
	}
}

// entExcOn marks genState.entExc set.
const entExcOn uint32 = 1 << 31

// copyExcPush appends one exception record (the game stalls when the pool
// is full).
func (s *State) copyExcPush(sl Slot, idx uint8) {
	if int(s.g.nfx) >= maxFx {
		s.stall(StallTables)
		return
	}
	s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxCopyExc, r: ovRec{idx: idx}}
	s.g.nfx++
	s.fxM.set(sl)
}

// copyExcKw is the keywords slot sl's copy exceptions grant.
//
//go:noinline
func (s *State) copyExcKw(k *prep, sl Slot) (m kop.KwMask) {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			m |= k.t.Overrides[r.r.idx-1].AddKw
		}
	}
	return
}

// copyExcKwRestore returns the keyword exceptions of the copies in m that an
// ended until-end-of-turn effect had set aside (loseAllOnset zeroes
// perm.ovr.kw, where they are settled, while the permanent loses its
// abilities): a copy's exceptions are layer 1 (CR 707.9b), so once no loss
// stands (CR 613.1f) they apply again.
func (s *State) copyExcKwRestore(k *prep, m slotMask) {
	if k.gen == nil {
		return
	}
	for ; m.any(); m = m.rest() {
		sl := m.first()
		if !s.occ.has(sl) || !s.fxM.has(sl) || s.genLost(k, sl) {
			continue
		}
		if p := &s.perms[sl]; p.ovr.kw|s.copyExcKw(k, sl) != p.ovr.kw {
			p.ovr.kw |= s.copyExcKw(k, sl)
			s.touchSl(sl)
		}
	}
}

// copyExcKwReps adds to rep the keywords slot sl's copy exceptions grant more
// than once (FeatKwReps): a copy of a copy carries its template's "except it
// has haste" beside its own.
func (s *State) copyExcKwReps(k *prep, sl Slot, rep *FeatKwReps) {
	var seen kop.KwMask
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			m := k.t.Overrides[r.r.idx-1].AddKw
			rep.add(seen, m)
			seen |= m
		}
	}
}

// copyExcPT is slot sl's set-P/T copy exception (the last one applied).
//
//go:noinline
func (s *State) copyExcPT(k *prep, sl Slot) (p, t int, ok bool) {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			if o := &k.t.Overrides[r.r.idx-1]; o.HasPT {
				p, t, ok = int(o.SetP), int(o.SetT), true
			}
		}
	}
	return
}

// copyExcSubs is the subtypes slot sl's copy exceptions add.
//
//go:noinline
func (s *State) copyExcSubs(k *prep, sl Slot) (m kop.SubMask) {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			m = m.Or(k.t.Overrides[r.r.idx-1].AddSub)
		}
	}
	return
}

// DebugCopyChangelingOff is the mutation switch: a copy's changeling
// exception is ignored, so the token keeps only the copied creature's types.
var DebugCopyChangelingOff bool

// copyExcAll reports whether slot sl's copy exceptions give it changeling
// (Dominion Infiltration's "except it has changeling", CR 707.9b): every
// creature type, a copiable value read where a printed changeling's
// kop.CardDef.AllCreatureTypes is (the layer-4 CDA seed, CR 613.3).
//
//go:noinline
func (s *State) copyExcAll(k *prep, sl Slot) bool {
	if DebugCopyChangelingOff {
		return false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc && k.t.Overrides[r.r.idx-1].AllCreatureTypes {
			return true
		}
	}
	return false
}

// copyExcCol is base (the card's colours) through slot sl's colour copy
// exceptions in order: a set replaces (embalm's white), an
// AddColors adds (gorge applies the set, then the add).
//
//go:noinline
func (s *State) copyExcCol(k *prep, sl Slot, base kop.ColorMask) (c kop.ColorMask) {
	c = base
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			o := &k.t.Overrides[r.r.idx-1]
			if o.HasColors {
				c = o.SetColors
			}
			if o.AddColors != 0 {
				c = c&^kop.ColC | o.AddColors
			}
		}
	}
	return
}

// cpxNoCost reports whether slot sl's copy exceptions remove its mana cost
// (embalm's "with no mana cost": mana value 0).
func (s *State) cpxNoCost(k *prep, sl Slot) bool {
	return k.gen != nil && k.gen.cpxNoCost && s.fxM.has(sl) && s.copyExcNoCost(k, sl)
}

//go:noinline
func (s *State) copyExcNoCost(k *prep, sl Slot) bool {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc && k.t.Overrides[r.r.idx-1].NoCost {
			return true
		}
	}
	return false
}

// copyExcHash mixes slot sl's copy exception records into h (0 when it has
// none: the position hashes then read as before).
//
//go:noinline
func (s *State) copyExcHash(sl Slot) (h uint64) {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			h = h<<8 | uint64(r.r.idx)
		}
	}
	return
}

// copyExcFlags reports whether some copy op of table t has a set-P/T or an
// added-subtype exception (the prep flags that gate the readers).
// copyExcOf is the copy exception override instruction in names (index+1), 0
// when it names none.
func copyExcOf(in *kop.Instr) int16 {
	switch in.Op {
	case kop.OpCopyAsTokenEx:
		return in.C
	case kop.OpExt:
		if x := kop.ExtOp(in.A.N); x == kop.ExtCopyAllEach || x == kop.ExtCopyExiledWith || x == kop.ExtCloneCopy || x == kop.ExtCopyPicked {
			return in.C
		} else if x == kop.ExtCopyNamed {
			return int16(uint16(in.C) >> kop.CopyNamedExcShift)
		}
	case kop.OpBecomeCopy:
		return in.C >> 1
	}
	return 0
}

func copyExcFlags(t *kop.Table) (pt, sub, col, nocost, ty, rmLeg bool) {
	see := func(x int16) {
		if x <= 0 || int(x) > len(t.Overrides) {
			return
		}
		o := &t.Overrides[x-1]
		pt = pt || o.HasPT
		sub = sub || o.AddSub != (kop.SubMask{}) || o.AllCreatureTypes
		col = col || o.HasColors || o.AddColors != 0
		ty = ty || o.AddTypes != 0 || o.RemoveLegendary
		rmLeg = rmLeg || o.RemoveLegendary
		nocost = nocost || o.NoCost
	}
	scan := func(abs []kop.Ability) {
		for i := range abs {
			for j := range abs[i].Prog {
				see(copyExcOf(&abs[i].Prog[j]))
			}
		}
	}
	for i := range t.Cards {
		scan(t.Cards[i].Abilities)
	}
	for i := range t.Emblems {
		scan(t.Emblems[i].Abilities)
	}
	return
}
