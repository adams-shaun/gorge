package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The CR 613 layer walk for layer 6 :
// keywords, losses and removals in timestamp order, reordered by gorge's
// dependency rule (engine/rules/chars/derive.go abilityDependencyOrder),
// and the layer-3 name and the loss read from the same walk. Cold code,
// reached from kwGen, lossAt, ovrLoses,
// FeatNameCard and grantedKwGen through one predictable branch: the
// table has a layer-6 shape the fixed-order code does not order
// (fxNeedL6), or the slot is a walk slot (perm.ovr.walk, two override
// effects). A table without either runs the fixed-order code.
//
// The effects of the walk, merged by timestamp (a removal before an
// addition on a tie, as lossAt's "a grant not older survives"):
//   - a one-shot loss of all abilities (perm.ovr ovLoseAll, its pool
//     timestamp);
//   - the override records and override statics of the layer walk
//     (charsFx), a static applying by its layer-4 verdict or, without a
//     layer-4 part, by its filter against the finished layer-5
//     characteristics (lfxLocked): AddKw, RemoveKw, LoseAbilities;
//   - the Continuous statics' keyword grants and RemoveKeyword$ removals
//     (Static.Override), stamped with their source's layerTs (an emblem's:
//     the timestamp recorded when it was created, State.g.embT);
//   - until-end-of-turn grants (perm.eotKw, an Animate's perm.ovr.kw):
//     when the table promotes them (fxNeedPromo: a static removal can be
//     older or newer than them) each grant left an fxKwGrant record with
//     its timestamp; an unrecorded grant is newer than every loss applying
//     (loseAllOnset set the older ones aside), so it applies last.
// The base is the printed keywords, a copy's keyword exceptions (layer 1)
// and the keyword counters (gorge BaseKeywords: a loss or a removal takes
// them too).
//
// gorge's dependency rule (CR 613.8): an effect whose filter reads
// keywords (kgen lowers only withFlying / withoutFlying) depends on every
// other layer-6 effect whose action on the base keywords changes its
// match; the walk emits the timestamp-earliest effect whose dependencies
// are emitted, and a cycle falls back to timestamp order. kgen refuses a
// gated effect that changes flying itself, so a cycle cannot form.

// fxNeed bits of layer 6 (beside static_layerwalk.go's).
const (
	fxNeedL6    uint8 = 1 << 3 // a layer-6 removal or a keyword-gated grant: kwGen walks
	fxNeedPromo uint8 = 1 << 4 // a static removal: until-end-of-turn grants keep timestamps (fxKwGrant)
)

// l6e is one layer-6 effect of the walk.
type l6e struct {
	add, rem kop.KwMask
	ts       uint16
	lose     bool
	gated    bool // its filter reads flying: okF / okN are its match with / without
	okF, okN bool
	kind     uint8 // l6Rec, l6Stat, l6EOT
	reps     uint8 // l6Rec: identical applications
}

const (
	l6Rec  uint8 = iota // an override record (applied by a resolved effect)
	l6Stat              // a static ability
	l6EOT               // an until-end-of-turn grant
)

const maxL6 = 32

// l6walk is the walk's effects in application order (a stack local).
type l6walk struct {
	e       [maxL6]l6e
	n       int
	order   [maxL6]uint8
	applied uint32 // bit per e index: the effect applied
	full    bool
	// cant: the CantHaveKeyword$ prohibitions applying (Override.CantHaveKw
	// of a matched Continuous static), dropped after the walk (gorge
	// chars.derive's final filter, CR 613.1f)
	cant kop.KwMask
}

// l6add merges e into timestamp order (a removal before an addition on
// a tie).
func (w *l6walk) l6add(e l6e) {
	if w.n >= maxL6 {
		w.full = true
		return
	}
	ke := uint32(e.ts)<<1 | l6addOnly(&e)
	j := w.n
	for j > 0 && uint32(w.e[j-1].ts)<<1|l6addOnly(&w.e[j-1]) > ke {
		w.e[j] = w.e[j-1]
		j--
	}
	w.e[j] = e
	w.n++
}

func l6addOnly(e *l6e) uint32 {
	if !e.lose && e.rem == 0 {
		return 1
	}
	return 0
}

// l6ok: e applies to an object with keywords kw.
func l6ok(e *l6e, kw kop.KwMask) bool {
	if !e.gated {
		return true
	}
	if kw&kop.KwFlying != 0 {
		return e.okF
	}
	return e.okN
}

// l6after is kw after e's action.
func l6after(e *l6e, kw kop.KwMask) kop.KwMask {
	if e.lose {
		kw = 0
	}
	return kw&^e.rem | e.add
}

// filterReadsKw reports whether a filter reads keywords.
func filterReadsKw(t *kop.Table, f kop.FilterID) bool {
	for ; f != 0; f = t.Filters[f].Or {
		if t.Filters[f].Flags2&(kop.F2WithFlying|kop.F2WithoutFlying) != 0 {
			return true
		}
	}
	return false
}

// matchKwAssume is matchPermGen with the filter's keyword gate read
// against fly (sl has flying) instead of sl's keywords.
func (s *State) matchKwAssume(k *prep, f kop.FilterID, sl Slot, ctrl uint8, src Slot, fly bool) bool {
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := k.t.Filters[f]
		if fl.Flags2&kop.F2WithFlying != 0 && !fly || fl.Flags2&kop.F2WithoutFlying != 0 && fly {
			continue
		}
		fl.Flags2 &^= kop.F2WithFlying | kop.F2WithoutFlying
		if s.matchPermOne(k, &fl, sl, ctrl, src, layerKw) {
			return true
		}
	}
	return false
}

// l6Walk runs the layer-6 walk on sl: its keywords, whether it lost all
// its abilities and the latest loss's timestamp; w keeps the effects (for
// grantedKwGen's per-effect list).
func (s *State) l6Walk(k *prep, sl Slot, w *l6walk) (kw kop.KwMask, lts uint16, lost bool) {
	p := &s.perms[sl]
	var fx [maxLfx]lfx
	var nf int
	c := s.charsFx(k, sl, upTo5, &fx, &nf)
	if p.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		w.l6add(l6e{lose: true, ts: s.lossFxTs(sl), kind: l6Rec, reps: 1})
	}
	for i := 0; i < nf; i++ {
		e := &fx[i]
		o := e.o
		if o == nil || !o.LoseAbilities && o.AddKw == 0 && o.RemoveKw == 0 {
			continue
		}
		if e.a == nil {
			w.l6add(l6e{add: o.AddKw, rem: o.RemoveKw, lose: o.LoseAbilities, ts: lossTs(o, e.ts), kind: l6Rec, reps: e.reps})
		} else if s.lfxLocked(k, sl, e, &c) {
			ts := e.ts
			if o.SetLandSub {
				ts = 0 // a land-type setter's loss is layer 4's: a granted keyword survives it (static_landtypesetter.go)
			}
			w.l6add(l6e{add: o.AddKw, rem: o.RemoveKw, lose: o.LoseAbilities, ts: ts, kind: l6Stat, reps: 1})
		}
	}
	// the Continuous statics' grants and removals
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.genLost(k, src) {
			continue
		}
		sp := &s.perms[src]
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && (a.Static.Kw != 0 || a.Static.Override > 0) {
				s.l6static(k, sl, w, statSrc{a: a, sl: src, ctrl: sp.ctrl, obj: sp.obj}, s.layerTs(src))
			}
		}
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			for j := range e.Abilities {
				if a := &e.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && (a.Static.Kw != 0 || a.Static.Override > 0) {
					s.l6static(k, sl, w, statSrc{a: a, sl: NoSlot, ctrl: pp, obj: NoObj, emb: pp<<4 | uint8(i+1)}, s.embStamp(pp, i))
				}
			}
		}
	}
	// until-end-of-turn grants; a copy's keyword exceptions are layer 1
	var cpKw kop.KwMask
	if s.fxM.has(sl) {
		cpKw = s.copyExcKw(k, sl) & p.ovr.kw // the exceptions' keywords, settled into ovr.kw at entry
	}
	if eot := (p.eotKw | p.ovr.kw) &^ cpKw; eot != 0 {
		if s.fxM.has(sl) {
			for i := 0; i < int(s.g.nfx); i++ {
				if r := &s.g.fx[i]; r.sl == sl && r.kind == fxKwGrant {
					if g := kwGrantMask(r) & eot; g != 0 {
						w.l6add(l6e{add: g, ts: r.r.ts, kind: l6EOT})
						eot &^= g
					}
				}
			}
		}
		if eot != 0 {
			w.l6add(l6e{add: eot, ts: 0xFFFF, kind: l6EOT})
		}
	}
	if w.full {
		s.capFull(StallTables)
	}
	var base kop.KwMask
	if !s.faceDown(k, sl) {
		base = k.cards[s.cardAt(sl)].kw // CR 708.2: no printed abilities
	}
	base |= cpKw
	base |= ctrKw(s, p)
	// CR 702.168b, 701.58a: a face-down permanent cast with disguise (or a
	// cloaked one) has ward {2} as a base keyword (gorge's BaseKeywords), so
	// a layer-6 lose-all removes it; a manifested one has none (CR 701.40a).
	if s.faceDown(k, sl) && s.faceDownWard(k, sl) {
		base |= kop.KwWard
	}
	if k.gen.anySuspect {
		base |= s.suspectKw(k, sl) // CR 701.60a: ahead of the walk, where the cloak's ward lands
	}
	s.l6order(w, base)
	kw = base
	for i := 0; i < w.n; i++ {
		j := w.order[i]
		e := &w.e[j]
		if !l6ok(e, kw) {
			continue
		}
		w.applied |= 1 << j
		if e.lose {
			lost = true
			lts = max(lts, e.ts)
		}
		kw = l6after(e, kw)
	}
	if !lost && s.nkwx != 0 {
		kw |= s.supKw(sl) // grants an ended loss had suppressed
	}
	return kw &^ w.cant, lts, lost
}

// l6static adds a Continuous static's layer-6 effect on sl to w.
func (s *State) l6static(k *prep, sl Slot, w *l6walk, st statSrc, ts uint16) {
	a := st.a
	if !s.statCond(k, st) {
		return
	}
	e := l6e{add: a.Static.Kw, ts: ts, kind: l6Stat, reps: 1}
	var cant kop.KwMask
	if a.Static.Override > 0 {
		o := &k.t.Overrides[a.Static.Override-1]
		e.rem, cant = o.RemoveKw, o.CantHaveKw // kgen: cant only on an ungated filter
	}
	if filterReadsKw(k.t, a.Static.Filter) {
		e.gated = true
		e.okF = s.matchKwAssume(k, a.Static.Filter, sl, st.ctrl, st.sl, true)
		e.okN = s.matchKwAssume(k, a.Static.Filter, sl, st.ctrl, st.sl, false)
		if !e.okF && !e.okN {
			return
		}
	} else if !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
		return
	}
	w.cant |= cant
	w.l6add(e)
}

// l6order is gorge's abilityDependencyOrder over w's effects (in
// timestamp order): Kahn's algorithm emitting the timestamp-earliest
// effect whose dependencies are emitted, a cycle falling back to the
// earliest remaining.
func (s *State) l6order(w *l6walk, base kop.KwMask) {
	gated := false
	for i := 0; i < w.n; i++ {
		w.order[i] = uint8(i)
		gated = gated || w.e[i].gated
	}
	if !gated {
		return
	}
	var dep [maxL6]uint32
	edges := false
	for g := 0; g < w.n; g++ {
		if !w.e[g].gated {
			continue
		}
		b := l6ok(&w.e[g], base)
		for m := 0; m < w.n; m++ {
			if m != g && l6ok(&w.e[g], l6after(&w.e[m], base)) != b {
				dep[g] |= 1 << m
				edges = true
			}
		}
	}
	if !edges {
		return
	}
	var done uint32
	for i := 0; i < w.n; i++ {
		pick := -1
		for j := 0; j < w.n && pick < 0; j++ {
			if done&(1<<j) == 0 && dep[j]&^done == 0 {
				pick = j
			}
		}
		for j := 0; j < w.n && pick < 0; j++ {
			if done&(1<<j) == 0 {
				pick = j
			}
		}
		done |= 1 << pick
		w.order[i] = uint8(pick)
	}
}

// kw6 is kwGen on the walk (out of line: its walk frame stays off
// kwGen's).
//
//go:noinline
func (s *State) kw6(k *prep, sl Slot) kop.KwMask {
	var w l6walk
	kw, _, _ := s.l6Walk(k, sl, &w)
	return kw
}

// loss6 is lossAt on the walk: the losses only (records, the one-shot
// loss and the override statics by their walk verdict), without the
// keyword grants (which read other permanents' losses).
func (s *State) loss6(k *prep, sl Slot) (ts uint16, lost bool) {
	p := &s.perms[sl]
	if p.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		ts, lost = s.lossFxTs(sl), true
	}
	if !k.gen.anyLoseStat || !s.loseStatOn(k, sl) {
		return s.loss6Rec(k, sl, ts, lost) // no lose-all static reaches sl: the records alone (kperf2)
	}
	var fx [maxLfx]lfx
	var nf int
	c := s.charsFx(k, sl, upTo5, &fx, &nf)
	for i := 0; i < nf; i++ {
		e := &fx[i]
		if e.o == nil || !e.o.LoseAbilities {
			continue
		}
		if e.a == nil || s.lfxLocked(k, sl, e, &c) {
			lost = true
			if !e.o.SetLandSub {
				ts = max(ts, e.ts) // a land-type setter's loss is layer 4's: no timestamp (static_landtypesetter.go)
			}
		}
	}
	return ts, lost
}

// loss6Lost is loss6's verdict alone.
func (s *State) loss6Lost(k *prep, sl Slot) bool {
	_, lost := s.loss6(k, sl)
	return lost
}

// l6On: sl's layer 6 (and layer 3) come from the walk.
func (s *State) l6On(k *prep, sl Slot) bool {
	return k.gen.fxNeed&fxNeedL6 != 0 && (!k.gen.l6RecOnly || s.fxM.has(sl)) || s.perms[sl].ovr.walk&walkOn != 0
}

// walkName is FeatNameCard on the walk: the newest layer-3 rename
// applying to sl, a static matched against the finished layer-4 types
// (gorge chars.Name binds Types' list).
func (s *State) walkName(k *prep, sl Slot) (CardID, bool) {
	var fx [maxLfx]lfx
	var nf int
	c := s.charsFx(k, sl, upTo4, &fx, &nf)
	nc := uint16(0)
	for i := 0; i < nf; i++ {
		e := &fx[i]
		if e.o == nil || e.o.NameCard == 0 {
			continue
		}
		if e.a == nil || s.lfxApplies(k, sl, e, &c) {
			nc = e.o.NameCard
		}
	}
	if nc == 0 {
		return 0, false
	}
	return CardID(nc - 1), true
}

// grantedKw6 is grantedKwGen on the walk: one call per applied record or
// static grant (a record once per application), less what a later effect
// removed.
func (s *State) grantedKw6(k *prep, sl Slot, add func(kop.KwMask)) {
	var w l6walk
	s.l6Walk(k, sl, &w)
	gone := w.cant // removed after the effect at i (a prohibition: after every one)
	var keep [maxL6]kop.KwMask
	for i := w.n - 1; i >= 0; i-- {
		j := w.order[i]
		e := &w.e[j]
		if w.applied&(1<<j) == 0 {
			continue
		}
		keep[j] = e.add &^ gone
		if e.lose {
			gone = ^kop.KwMask(0)
		}
		gone |= e.rem
	}
	for i := 0; i < w.n; i++ {
		j := w.order[i]
		e := &w.e[j]
		if w.applied&(1<<j) == 0 || e.kind == l6EOT || keep[j] == 0 {
			continue
		}
		for r := 0; r < max(int(e.reps), 1); r++ {
			add(keep[j])
		}
	}
}

// kwGrantMask is an fxKwGrant record's keywords (packed into the ovRec's
// idx, fl and src bytes).
func kwGrantMask(r *fxRec) kop.KwMask {
	return kop.KwMask(r.r.idx) | kop.KwMask(r.r.fl)<<8 | kop.KwMask(r.r.src)<<16
}

// promoKw: until-end-of-turn keywords m were granted to sl; a table that
// promotes them records the grant's timestamp.
func (s *State) promoKw(k *prep, sl Slot, m kop.KwMask) {
	if k.gen != nil && k.gen.fxNeed&fxNeedPromo != 0 {
		s.kwGrantFx(sl, m)
	}
}

// kwGrantFx records an until-end-of-turn grant of m to sl at a new
// timestamp; an older record's same keywords give way (only the newest
// grant of a keyword decides whether a removal took it). "Can't be
// blocked" is an effect, not an ability (suppressEOT keeps it): never
// recorded, so it applies last.
//
//go:noinline
func (s *State) kwGrantFx(sl Slot, m kop.KwMask) {
	m &^= kop.KwUnblockable
	if m == 0 {
		return
	}
	ts := s.tick()
	if s.fxM.has(sl) {
		empty := false
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; r.sl == sl && r.kind == fxKwGrant {
				g := kwGrantMask(r) &^ m
				r.r.idx, r.r.fl, r.r.src = uint8(g), uint8(g>>8), CardID(g>>16)
				empty = empty || g == 0
			}
		}
		if empty {
			s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.kind == fxKwGrant && kwGrantMask(r) == 0 })
		}
	}
	if int(s.g.nfx) >= maxFx {
		s.capFull(StallTables)
		return
	}
	s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxKwGrant, r: ovRec{idx: uint8(m), fl: uint8(m >> 8), src: CardID(m >> 16), ts: ts}}
	s.g.nfx++
	s.fxM.set(sl)
	s.touchSl(sl)
}

// prepL6 sets the layer-6 fxNeed bits: a RemoveKeyword$ (on an override
// or a Continuous static), a global override static with a layer-6 part,
// a keyword-gated keyword grant.
func prepL6(t *kop.Table, g *genPrep) {
	globLose := false
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbStatic {
				continue
			}
			var o *kop.Override
			if a.Static.Override > 0 {
				o = &t.Overrides[a.Static.Override-1]
			}
			switch a.Static.Kind {
			case kop.StatOverride:
				if o == nil {
					continue
				}
				if o.RemoveKw != 0 {
					g.fxNeed |= fxNeedL6 | fxNeedPromo
				}
				if !filterAttachedOnly(t, a.Static.Filter) && (o.LoseAbilities || o.AddKw != 0) {
					g.fxNeed |= fxNeedL6
					if o.LoseAbilities {
						g.fxNeed |= fxNeedPromo
						globLose = true
					}
				}
			case kop.StatContinuous:
				if o != nil && o.RemoveKw != 0 {
					g.fxNeed |= fxNeedL6 | fxNeedPromo
				}
				if a.Static.Kw != 0 && filterReadsKw(t, a.Static.Filter) {
					g.fxNeed |= fxNeedL6
				}
			}
		}
	}
	g.l6Stall = globLose && g.fxNeed&fxNeedSubGrant != 0
	if g.fxNeed&fxNeedL6 == 0 && overridesRemoveKw(t) {
		g.fxNeed |= fxNeedL6 | fxNeedPromo // an effect's removal (Debuff)
		g.l6RecOnly = true
	}
}
