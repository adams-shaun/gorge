package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The cross-transition cache of derived types (typesGen;
// docs/design/dirac-incremental-characteristics.md).
//
// A permanent's derived types are asked about 190 times per posed
// transition on the Commander bench, and the answer almost never changes:
// each "does static X's filter match slot Y" derives Y's types through the
// CR 613 layer walk again. The cache keeps the answer per slot across
// walks, state-based passes and applies, for the State that owns it.
//
// It lives in the ForkPool, never State, and holds no pointers, and it runs
// only on a table whose type statics read objects' own fields alone
// (tyObjTable: no condition but a constant count of battlefield objects, no
// gate, plain filters; no Ring emblem). Any other table computes every ask.
// An entry answers only when all of these hold:
//   - the asking State owns the cache (a fork never takes it over), no
//     filter search (inFilt) or census window runs, and no in-place probe
//     holds it (tyCacheHold: pcMemoHold's sites, lfxCond's re-entry guard,
//     askColValue's chosen-colour probe);
//   - the context has not moved since the entry was written: the line of
//     play, the emblem counts, the control masks (a probe withholds slots
//     from them in place), the statics in force, and every write to a type
//     source (a permanent among them whose card prints a type effect) —
//     on a table whose statics count the board (tyBoard), every write;
//   - the slot holds the same object, and that object has not been written
//     since (ChangedSince: the State's own per-object write stamps).
//
// A whole-State overwrite (CopyFrom, InitNStart) drops every entry, and
// refreshTy drops them when it moves a slot's walk flag (an unmarked write
// walkSlot reads). Anything else recomputes, so a miss is never wrong, only
// slower. DebugTyCacheShadow recomputes each hit and panics on a difference.

type tyCacheEnt struct {
	key uint32 // the context number it was written under; 0: none
	t0  uint32 // the touch count it was written at
	obj Obj    // the slot's object then
	t   kop.TypeMask
}

type tyCache struct {
	owner uintptr // the State the entries belong to (identity, never dereferenced); 0: none
	hold  uint8
	ctxN  uint32 // the current context's number
	// the context: the touch count it was last brought up to, the line, the
	// emblem counts and the control masks
	oTouch uint32
	oLine  uint64
	oEmb   [MaxSeats]uint8
	oCtrl  [MaxSeats]slotMask
	oStat  slotMask // statM when srcM was taken
	srcM   slotMask // the battlefield sources of a type effect (layer sources)
	exM    slotMask // the slots whose own statics read more than objects (prep.tyEx): never cached
	ent    [MaxPerm]tyCacheEnt
	// over the pool's life (the tests read them)
	hits, miss uint32
}

// DebugTyCacheOff turns the cache off.
var DebugTyCacheOff bool

// DebugTyCacheShadow makes every hit recompute and panic on a difference;
// DebugTyCacheHits and DebugTyCacheBad count the hits it checked and the
// stale ones (a caller that recovers panics reads the count).
var (
	DebugTyCacheShadow bool
	DebugTyCacheHits   int
	DebugTyCacheMisses int
	DebugTyCacheBad    int
)

// debugTyCacheNoInval drops the context check, and DebugTyCacheNoEx the
// per-source exclusion: the mutations the shadow check must catch.
var debugTyCacheNoInval, DebugTyCacheNoEx bool

// prepTyCache reports whether the cache pays on table p: some slot's types
// can come from the layer walk or an override static. A table with neither
// pays one flag test per ask.
func prepTyCache(p *prep) bool {
	g := p.gen
	return g != nil && (g.fxNeed != 0 || g.anyOvrStat)
}

// next starts a new context: no entry written before answers after.
func (m *tyCache) next() {
	if m.ctxN++; m.ctxN == 0 {
		clear(m.ent[:])
		m.ctxN = 1
	}
}

// tyCacheDrop forgets every entry of s's (a whole-State overwrite).
func (s *State) tyCacheDrop() {
	if p := s.pool; p != nil && p.tyc.owner == s.addr() {
		p.tyc.next()
	}
}

func (s *State) tyCacheHold() {
	if p := s.pool; p != nil {
		p.tyc.hold++
	}
}

func (s *State) tyCacheRelease() {
	if p := s.pool; p != nil && p.tyc.hold > 0 {
		p.tyc.hold--
	}
}

func (s *State) typesCached(k *prep, sl Slot) kop.TypeMask {
	p := s.pool
	if p == nil {
		return s.typesCalc(k, sl) // no pool yet: the cache never allocates one (its first fork or priority walk does)
	}
	m := &p.tyc
	if DebugTyCacheOff || m.hold != 0 || p.inFilt != 0 || p.census {
		return s.typesCalc(k, sl)
	}
	return s.typesCachedObj(k, sl, m)
}

// prepTyObj marks the tables the cache runs on (prepare).
func prepTyObj(p *prep, t *kop.Table) {
	if p.tyCache {
		p.tyObj, p.tyBoard, p.tyEx = tyObjTable(t, p.gen)
		p.tyCache = p.tyObj // a table whose type statics read more than objects computes every ask
		if p.tyObj && p.gen.anyGrant {
			obj, board, ex := grObjTable(t, p.tyEx)
			p.grCache = obj && !(board && ex != nil) // static_grantcache.go
			p.tyBoard = p.tyBoard || board
			p.tyEx = ex
		}
	}
}

// tyCacheReset is InitNStart's: a new game, and no hold a panic left behind.
func (s *State) tyCacheReset() {
	if p := s.pool; p != nil {
		p.tyc.hold = 0
		p.tyc.next()
	}
}

// typesCachedObj answers from the cache where an entry stands.
func (s *State) typesCachedObj(k *prep, sl Slot, m *tyCache) kop.TypeMask {
	if m.owner != s.addr() {
		if s.pool.depth != 0 {
			return s.typesCalc(k, sl)
		}
		m.owner = s.addr()
		m.next()
		m.oTouch, m.oLine, m.oEmb, m.oCtrl = s.touchN, s.line, s.g.nemb, s.ctrlM
		s.tySrcs(k, m)
	} else if m.oTouch != s.touchN || m.oLine != s.line || m.oEmb != s.g.nemb || m.oCtrl != s.ctrlM {
		s.tyRevalidate(k, m)
	}
	if m.exM.has(sl) {
		return s.typesCalc(k, sl) // its own statics read more than objects
	}
	e := &m.ent[sl]
	obj := s.perms[sl].obj
	if e.key == m.ctxN && e.obj == obj && (k.tyBoard || !s.ChangedSince(obj, e.t0) || debugTyCacheNoInval) {
		m.hits++
		if DebugTyCacheShadow {
			DebugTyCacheHits++
			if calc := s.typesCalc(k, sl); calc != e.t {
				DebugTyCacheBad++
				panic("dirac: typesGen cache answered stale types")
			}
		}
		return e.t
	}
	m.miss++
	if DebugTyCacheShadow {
		DebugTyCacheMisses++
	}
	key, t0 := m.ctxN, s.touchN
	t := s.typesCalc(k, sl)
	if m.ctxN == key && s.touchN == t0 && s.ctrlM == m.oCtrl {
		*e = tyCacheEnt{key: key, t0: t0, obj: obj, t: t}
	}
	return t
}

// tyRevalidate brings the object-only context up to the State: a new line,
// emblem or control mask, a write to a type source, a source entering or
// leaving, or (on a table whose type statics count the board, tyBoard) any
// write starts a new context; a write to any other object leaves the
// entries standing, each checking its own object's stamp (ChangedSince).
func (s *State) tyRevalidate(k *prep, m *tyCache) {
	glob := m.oLine != s.line || m.oEmb != s.g.nemb || m.oCtrl != s.ctrlM || k.tyBoard || s.touchN-m.oTouch >= 256 || m.oStat != s.statM
	if !glob {
		for sm := m.srcM; sm.any(); sm = sm.rest() {
			if s.ChangedSince(s.perms[sm.first()].obj, m.oTouch) {
				glob = true
				break
			}
		}
	}
	if glob {
		if !debugTyCacheNoInval {
			m.next()
		}
		m.oLine, m.oEmb, m.oCtrl = s.line, s.g.nemb, s.ctrlM
		if m.oStat != s.statM {
			s.tySrcs(k, m)
		}
	}
	m.oTouch = s.touchN
}

// tySrcs takes the battlefield's type sources: the permanents among the
// statics in force whose card prints a type effect (every one on a table
// that builds piles, whose under cards' statics count).
func (s *State) tySrcs(k *prep, m *tyCache) {
	m.oStat = s.statM
	m.srcM, m.exM = slotMask{}, slotMask{}
	for sm := s.statM; sm.any(); sm = sm.rest() {
		sl := sm.first()
		if c := s.cardAt(sl); k.piles || k.gen.has(c, gfOvrStat) || k.gen.has(c, gfSubGrant) || k.gen.has(s.perms[sl].card, gfOvrStat) || k.gen.has(s.perms[sl].card, gfSubGrant) {
			m.srcM = m.srcM.with(sl)
		}
		// a pile's under cards' statics count too: on a pile table every
		// static source is excluded once the table has an excluded card
		if k.tyEx != nil && !DebugTyCacheNoEx && (k.piles || k.tyEx[s.cardAt(sl)] || k.tyEx[s.perms[sl].card]) {
			m.exM = m.exM.with(sl)
		}
	}
}

// tyObjTable reports whether every static of table t that can change a
// type (an override static, a Continuous subtype grant) reads only objects'
// own fields: no condition but a constant count of battlefield objects
// matching a plain filter, no gate, and filters of plain object flags,
// types, colours, subtypes and counter thresholds. The Ring emblem reads a
// seat's bearer, so a table with it is never object-only.
func tyObjTable(t *kop.Table, g *genPrep) (obj, board bool, ex []bool) {
	if g == nil || g.ringOn() {
		return false, false, nil
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbStatic {
				continue
			}
			sa := &a.Static
			if !(sa.Kind == kop.StatOverride && sa.Override != 0 || sa.Kind == kop.StatContinuous && (sa.AddSub.Any() || sa.AddChosen)) {
				continue
			}
			if sa.Cond != 0 || sa.OvGate != 0 || !tyObjCond(t, sa.Cond2, 0) || !tyObjFilter(t, sa.Filter) {
				if !tySelfOnly(t, i, sa) {
					return false, false, nil
				}
				if ex == nil {
					ex = make([]bool, len(t.Cards))
				}
				ex[i] = true // its source computes every ask (tySelfOnly)
				continue
			}
			if sa.Cond2 != 0 {
				board = true // a count of the board's objects: any write may change it
			}
		}
	}
	if board && ex != nil {
		return false, false, nil // a board count can read an excluded source's derived types
	}
	return true, board, ex
}

// tySelfOnly reports whether static sa of card c, though it reads more than
// objects, can change only its own source's characteristics: its filter is
// the source alone (FSelf, no alternative), it removes no abilities (so it
// never switches the source's other statics on or off), and the card grants
// no static (GrantSt), so no other permanent can carry it. Such a source is
// excluded from the caches per slot (tyCache.exM): every other slot's types
// read the source only through its own fields (controller, attachment,
// chosen value), which a write marks.
func tySelfOnly(t *kop.Table, c int, sa *kop.Static) bool {
	if sa.Filter == 0 {
		return false
	}
	fl := &t.Filters[sa.Filter]
	if fl.Flags&kop.FSelf == 0 || fl.Or != 0 {
		return false
	}
	if sa.Kind == kop.StatOverride && t.Overrides[sa.Override-1].LoseAbilities {
		return false
	}
	for j := range t.Cards[c].Abilities {
		if a := &t.Cards[c].Abilities[j]; a.Kind == kop.AbStatic && a.Static.GrantSt != 0 {
			return false
		}
	}
	return true
}

func tyObjCond(t *kop.Table, c kop.CondID, depth int) bool {
	if c == 0 {
		return true
	}
	if depth > 4 {
		return false
	}
	cd := &t.Conds[c]
	switch cd.Kind {
	case kop.CondPresent:
		return cd.A.K == kop.VConst && cd.Ref == 0 && cd.Zone == 0 && !cd.Neg && tyObjFilter(t, cd.F)
	case kop.CondAnd:
		return !cd.Neg && tyObjCond(t, cd.Sub[0], depth+1) && tyObjCond(t, cd.Sub[1], depth+1)
	}
	return false
}

func tyObjFilter(t *kop.Table, f kop.FilterID) bool {
	const plain = kop.FOther | kop.FNontoken | kop.FToken | kop.FTapped | kop.FUntapped | kop.FAttacking | kop.FNotAttacking |
		kop.FSelf | kop.FAttachedBySrc | kop.FNotAttachedBySrc
	for ; f != 0; f = t.Filters[f].Or {
		fl := &t.Filters[f]
		if fl.CtrlTgt != 0 || fl.Flags&^plain != 0 || fl.Flags2 != 0 || fl.Ctrl > kop.CtrlOpp || fl.Owner > kop.CtrlOpp || fl.Zone != kop.ZoneNone {
			return false
		}
		switch fl.Pred {
		case kop.PredNone, kop.PredHasCounter, kop.PredCountersGE, kop.PredHasCounters, kop.PredNoCounters:
		default:
			return false
		}
	}
	return true
}
