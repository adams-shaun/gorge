package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The cross-transition cache of granted-ability lists (grantedAbilitiesOwn
// without an own callback), beside the derived-types cache
// (static_typecache.go) and under the same per-object key.
//
// A slot's list reads the slot (its records, timestamp and loss), the
// statics in force and their sources (global, attached and Continuous
// grants), and, through the grants' filters, the slot's derived types. So an
// entry answers only while the types cache's context stands (tyCache.ctxN),
// no object among the statics in force was written and none entered or left
// (grEpoch), the slot holds the same object, unwritten since the entry
// (ChangedSince), and the outer grantSrc is the one the list was walked
// under (a callback reads it; each item restores its own). On a table whose
// type or grant statics count the board (tyBoard), any write ends every
// entry. The cache runs only on a table whose grant statics read objects
// alone (grObjTable), and the list lives in a pooled arena: a list longer
// than grMax, or one the arena cannot hold, is not remembered. The arena
// is emptied, with every entry, when it fills.
//
// DebugTyCacheShadow walks each hit again and panics when the lists differ,
// or when a callback wrote the game mid-list.

// grMax is the longest list one entry remembers.
const grMax = 32

// grArenaN is the items the arena holds.
const grArenaN = 2048

type grEnt struct {
	ep    uint32 // grCache.ep it was written under; 0: none
	t0    uint32 // the touch count then
	obj   Obj
	outer Obj    // s.g.grantSrc when the list was walked
	off   uint16 // the list in the arena
	n     uint8
}

// grItem is one remembered grant: f(c, ab, n) with s.g.grantSrc src.
type grItem struct {
	c   CardID
	src Obj
	n   uint16
	ab  uint8
}

type grCache struct {
	ep     uint32 // the list epoch
	ctx    uint32 // the types cache's context number it was taken under
	gTouch uint32 // the touch count the epoch was brought up to
	gStat  slotMask
	an     uint16 // arena items used
	ent    [MaxPerm]grEnt
	arena  [grArenaN]grItem
	// over the pool's life (the tests read them)
	hits, miss uint32
}

func (g *grCache) next() {
	if g.ep++; g.ep == 0 {
		clear(g.ent[:])
		g.ep = 1
	}
	g.an = 0
}

// grantedCached is grantedAbilitiesOwn without an own callback, through the
// cache where an entry stands.
func (s *State) grantedCached(k *prep, sl Slot, f func(c CardID, ab uint8, n int)) {
	p := s.pool
	if p == nil {
		p = new(ForkPool)
		s.pool = p
	}
	m := &p.tyc
	if DebugTyCacheOff || m.hold != 0 || p.inFilt != 0 || p.census {
		s.grantedAbilitiesWalk(k, sl, nil, f)
		return
	}
	if m.owner != s.addr() {
		if p.depth != 0 {
			s.grantedAbilitiesWalk(k, sl, nil, f)
			return
		}
		s.typesCachedObj(k, sl, m) // takes the cache over (a types ask's start)
	} else if m.oTouch != s.touchN || m.oLine != s.line || m.oEmb != s.g.nemb || m.oCtrl != s.ctrlM {
		s.tyRevalidate(k, m)
	}
	g := &p.grc
	if g.ctx != m.ctxN {
		g.ctx = m.ctxN
		g.next()
	}
	if s.touchN != g.gTouch {
		bump := k.tyBoard || s.touchN-g.gTouch >= 256 || g.gStat != s.statM
		for sm := s.statM; !bump && sm.any(); sm = sm.rest() {
			bump = s.ChangedSince(s.perms[sm.first()].obj, g.gTouch)
		}
		if bump && !debugTyCacheNoInval {
			g.next()
		}
		g.gTouch, g.gStat = s.touchN, s.statM
	}
	obj := s.perms[sl].obj
	outer := s.g.grantSrc
	if e := &g.ent[sl]; e.ep == g.ep && e.obj == obj && e.outer == outer && (!s.ChangedSince(obj, e.t0) || debugTyCacheNoInval) {
		g.hits++
		items := g.arena[e.off : e.off+uint16(e.n)]
		if DebugTyCacheShadow {
			DebugTyCacheHits++
			s.grantedShadow(k, sl, items)
		}
		t0 := s.touchN
		for _, it := range items {
			s.g.grantSrc = it.src
			f(it.c, it.ab, int(it.n))
			if DebugTyCacheShadow && s.touchN != t0 {
				DebugTyCacheBad++
				panic("dirac: a grantedAbilities callback wrote the game mid-list")
			}
		}
		s.g.grantSrc = outer
		return
	}
	g.miss++
	if DebugTyCacheShadow {
		DebugTyCacheMisses++
	}
	var buf [grMax]grItem
	n, over := 0, false
	ep, t0 := g.ep, s.touchN
	s.grantedAbilitiesWalk(k, sl, nil, func(c CardID, ab uint8, cnt int) {
		if n < grMax && cnt >= 0 && cnt <= 0xffff {
			buf[n] = grItem{c: c, src: s.g.grantSrc, n: uint16(cnt), ab: ab}
			n++
		} else {
			over = true
		}
		f(c, ab, cnt)
	})
	if over || g.ep != ep || s.touchN != t0 || m.ctxN != g.ctx || s.g.grantSrc != outer {
		return // the walk or its callbacks wrote: not remembered
	}
	if int(g.an)+n > grArenaN {
		g.next() // the arena is full: start over
		return
	}
	off := g.an
	copy(g.arena[off:], buf[:n])
	g.an += uint16(n)
	g.ent[sl] = grEnt{ep: g.ep, t0: t0, obj: obj, outer: outer, off: off, n: uint8(n)}
}

// grantedShadow walks sl's grants again and panics when they differ from want.
func (s *State) grantedShadow(k *prep, sl Slot, want []grItem) {
	outer := s.g.grantSrc
	i, bad := 0, false
	s.grantedAbilitiesWalk(k, sl, nil, func(c CardID, ab uint8, cnt int) {
		if i >= len(want) || want[i] != (grItem{c: c, src: s.g.grantSrc, n: uint16(cnt), ab: ab}) {
			bad = true
		}
		i++
	})
	s.g.grantSrc = outer
	if bad || i != len(want) {
		DebugTyCacheBad++
		panic("dirac: grantedAbilities cache answered a stale list")
	}
}

// grObjTable reports whether every static of table t that grants an
// ability reads only objects' own fields (tyObjFilter, tyObjCond), and
// whether one counts the board.
func grObjTable(t *kop.Table) (obj, board bool) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbStatic {
				continue
			}
			sa := &a.Static
			if sa.Grant == 0 && sa.GrantSt == 0 {
				continue
			}
			if sa.Cond != 0 || sa.OvGate != 0 || !tyObjCond(t, sa.Cond2, 0) || !tyObjFilter(t, sa.Filter) {
				return false, false
			}
			if sa.Cond2 != 0 {
				board = true
			}
		}
	}
	return true, board
}
