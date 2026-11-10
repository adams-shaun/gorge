package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// one put per counter kind at entry.
//
// CR 614.1c / 122.6: every counter of one kind an object enters with is put
// by ONE event. gorge's entry plan (engine/rules/entry_counters.go
// entryGrantPlan) folds the entrant's own enters-with bodies, a lent
// (foreign) body and an Effect-created body (Torgal, Wildgrowth Archaic) into
// one grant per kind, and folds that into the requested base grant, so an
// additive "that many plus one" replacement (Hardened Scales, Kami of
// Whispered Hopes, Ozolith) applies once to the sum: Archaic's own
// converge counter and the earlier Archaic's effect counter enter as
// (1+1)+1 = 3, not (1+1)+(1+1) = 4. A doubler is unchanged (2a+2b = 2(a+b)).
//
// The kernel used to put each source through the counter replacements on its
// own. Now, while the entering permanent's window is open (enterGen, from the
// printed constant to the end of the enters and others-enter programs), a
// +1/+1 or -1/-1 amount from any of those sources is only added to the sum in
// s.g.enterCtr.n (kind in .kind; .tap marks the window open, .ctrl that two or
// more sources merged), and enterGen's exit applies the replacements once to
// the sum. A source of another kind, or one outside the window (a program
// resumed after its ask), keeps the separate put.
//
// Not modelled: gorge keeps an entrant's own SECOND body of one kind a
// separate put when there is no requested base to fold into. No lowered card
// has two own bodies of one kind, so they sum here too.

// DebugEntryCtrSplit is TestEntryctrMutation's switch: the old separate puts.
var DebugEntryCtrSplit bool

// EntryMergeStats counts the lockstep test's coverage: entries whose counters
// of one kind came from two or more sources, and the ones among those where
// an additive modifier changed the sum.
var EntryMergeStats struct{ Merged, Modified atomic.Int64 }

// prepEntryMerge sets genPrep.anyEntryMerge: the table has an AddCounter
// replacement and some source of entry counters besides an entrant's own
// body (an Effect-borne body, a foreign program, a requested count).
func prepEntryMerge(t *kop.Table, g *genPrep) {
	if g.anyRepl&(rbDblCtr|rbCtrOn|rbDblCtrYou|rbPlusCtrEOT) == 0 {
		return
	}
	src := g.anyEffEtbCtr
	for i := range t.Cards {
		if t.Cards[i].Impending > 0 {
			src = true
		}
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			for _, in := range a.Prog {
				switch in.Op {
				case kop.OpReturnEx, kop.OpChangeZoneAllEx, kop.OpChooseMove, kop.OpDigCounters, kop.OpIncubate:
					src = true
				case kop.OpExt:
					// Only the arm (a positive amount) is a source; the
					// trailing clear is B=0 and must not enable the window.
					if in.Ext() == kop.ExtSearchCounters && in.B.K == kop.VConst && in.B.N > 0 && entryMergeKind(int(in.C)) {
						src = true // a library search's +1/+1 or -1/-1 rider
					}
				case kop.OpEnterWithCounters:
					if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplOthersEnterProgram {
						src = true
					}
				}
			}
		}
	}
	g.anyEntryMerge = src
}

// entryMergeKind: the kinds whose entry amounts merge.
func entryMergeKind(kind int) bool {
	return kind == int(kop.CtrP1P1) || kind == int(kop.CtrM1M1)
}

// entryAcc adds n counters of kind to the open window's sum and reports
// whether it did; false leaves the caller's own separate put.
func (s *State) entryAcc(kind, n int) bool {
	e := &s.g.enterCtr
	if DebugEntryCtrSplit || !e.tap || n <= 0 || !entryMergeKind(kind) {
		return false
	}
	if e.n != 0 {
		if int(e.kind) != kind || int(e.n)+n > 0xff {
			return false
		}
		e.ctrl = true
	} else if n > 0xff {
		return false
	}
	e.n, e.kind = e.n+uint8(n), kop.CounterKind(kind)
	return true
}

// entryWinOpen opens the entering permanent's window (face 0): its printed
// enters-with constant and the requested count (s.g.enterCtr) start the sum,
// each separately put if its kind does not merge. enterGen calls it where it
// used to place the requested count.
func (s *State) entryWinOpen(k *prep, sl Slot, p *perm) {
	e := &s.g.enterCtr
	rn, rk := int(e.n), int(e.kind)
	e.n, e.tap, e.ctrl = 0, true, false
	if rn > 0 {
		if !s.entryAcc(rk, rn) {
			s.placeEntryCounters(k, sl, rk, rn, p.ctrl)
		}
		if k2 := int(e.kind2) - 1; k2 >= 0 { // the pair's second kind (returnEntryCount)
			if !s.entryAcc(k2, rn) {
				s.placeEntryCounters(k, sl, k2, rn, p.ctrl)
			}
		}
	}
	e.kind2 = 0
	if pc := &k.cards[p.card]; pc.enterCtr != 0 {
		ck := int(pc.enterCK)
		if !s.entryAcc(ck, int(pc.enterCtr)) {
			s.placeEntryCounters(k, sl, ck, int(pc.enterCtr), p.ctrl)
		}
	}
}

// entryWinClose closes the window: the sum goes through the counter
// replacements once.
func (s *State) entryWinClose(k *prep, sl Slot, p *perm) {
	e := &s.g.enterCtr
	if !e.tap {
		return
	}
	n, kind, merged := int(e.n), int(e.kind), e.ctrl
	e.n, e.tap, e.ctrl = 0, false, false
	if n == 0 {
		return
	}
	v := s.placeEntryCounters(k, sl, kind, n, p.ctrl)
	if merged {
		EntryMergeStats.Merged.Add(1)
		if v > 0 && v != n && v != 2*n { // an additive modifier acted on the sum (a pure doubling would not tell the merge from the split)
			EntryMergeStats.Modified.Add(1)
		}
	}
}

// The pending effect count of a spell (spc bits 11-15, ctr_effectentry.go)
// is a code: 0 nothing, 1-21 one effect of that many counters, 22-31 two
// effects (a <= b <= 4) in etbPairs order.
var etbPairs = [10][2]uint8{{1, 1}, {1, 2}, {1, 3}, {1, 4}, {2, 2}, {2, 3}, {2, 4}, {3, 3}, {3, 4}, {4, 4}}

// etbCodeParts is the counts of the two effects of code (b = 0: one or none).
func etbCodeParts(code int) (a, b int) {
	if code >= 22 {
		pr := etbPairs[code-22]
		return int(pr[0]), int(pr[1])
	}
	return code, 0
}

// etbCodeAdd is code with one more effect of n > 0 counters; false when the
// code cannot hold it.
func etbCodeAdd(code, n int) (int, bool) {
	a, b := etbCodeParts(code)
	switch {
	case b > 0:
		return 0, false
	case a == 0:
		return n, n <= 21
	}
	if a > n {
		a, n = n, a
	}
	for i, pr := range etbPairs {
		if int(pr[0]) == a && int(pr[1]) == n {
			return 22 + i, true
		}
	}
	return 0, false
}

// entryPlan reports whether gorge stages the entry of the permanent at sl for
// counters (entryBodyCandidates, entryCounterGrants), by shape: the entrant's
// own enters-with body, an impending count, a requested count, or a lent
// enters-with body on ANY other permanent. Only then does it fold an
// Effect-borne body into the entry's put.
func (s *State) entryPlan(k *prep, sl Slot, p *perm) bool {
	c := p.card
	if k.cards[c].enterCtr != 0 || k.t.Cards[c].Impending > 0 || s.g.enterCtr.n > 0 {
		return true
	}
	g := k.gen
	if g.has(c, gfEnterRepl) && repProgCounters(&k.t.Cards[c], kop.ReplEntersProgram) {
		return true
	}
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if src == sl || s.genLost(k, src) {
			continue
		}
		if scid := s.cardAt(src); g.has(scid, gfOthersRep) && repProgCounters(&k.t.Cards[scid], kop.ReplOthersEnterProgram) {
			return true
		}
	}
	return false
}

// repProgCounters: card cd has a replacement of kind rk whose program holds an
// OpEnterWithCounters.
func repProgCounters(cd *kop.CardDef, rk kop.ReplKind) bool {
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != rk {
			continue
		}
		for _, in := range a.Prog {
			if in.Op == kop.OpEnterWithCounters {
				return true
			}
		}
	}
	return false
}

// a token copy of a clone-like card
// is asked the clone election and the answer is applied as the token enters
// (rules/replacement.go applyReplacementsDispatch, the IsToken arm). The copy
// keeps its copy exceptions on top (the copy token's own, CR 707.9b).

// DebugCloneTokenNoAsk is TestClonetokMutation's switch: the old "a token
// copy asks nothing and enters as itself".
var DebugCloneTokenNoAsk bool

// prepBecomeCopy sets genPrep.anyBecomeCopy when some enters program is an
// as-enters copy.
func prepBecomeCopy(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram && len(a.Prog) > 0 && a.Prog[0].Op == kop.OpBecomeCopy {
				g.anyBecomeCopy = true
			}
		}
	}
}

// cloneTokenAsk is akClone when the copiable card c of a token about to be
// minted by in carries the as-enters copy line, else 0. A token copy of a
// permanent that is itself a copy would be minted on that permanent's printed
// card while it waits for the answer (copyTokenBasis): not modelled, it
// stalls (StallCopyAllAsk). A copy-all sweep asks its source's slot itself
// (sweepRun, copy_allpark.go), so a source that is not a copy poses the
// election instead of stalling.
//
//go:noinline
func (s *State) cloneTokenAsk(k *prep, in *kop.Instr, c CardID) uint8 {
	if DebugCloneTokenNoAsk {
		return 0
	}
	kind := s.cloneAskKind(k, c)
	if kind != 0 && in != nil {
		if src := s.objRefObj(in.Obj); src != NoObj && s.objs[src].zone == kop.ZoneBattlefield && s.perms[s.objs[src].slot].ovr.flags&ovCopy != 0 {
			s.capFull(StallCopyAllAsk)
			return 0
		}
	}
	return kind
}

// cloneTokenElected is cloneTokenBasis for a token that elected a copy as it
// entered: gorge mints it on the copied permanent's PRINTED card
// (copyTokenBasis) and the election is applied over that basis, so the
// token's card is tmpl's printed card and its copy stays the elected template.
//
//go:noinline
func (s *State) cloneTokenElected(sl Slot, t Obj, tmpl Slot) {
	p := &s.perms[sl]
	p.card = s.perms[tmpl].card
	s.objs[t].card = p.card
}

// returnEntryCount decodes an OpReturnEx's counter operand (a is its A, v the
// evaluated amount): the per-kind count and the second kind's entry-counter
// byte (kind+1, 0 for none). A constant carries the pair as count | (second
// kind + 1)<<8, read as unsigned (kop.RetCounterPairShift; WithCountersType$ with
// two kinds, gorge entryWithCounters); any other operand is a plain count.
func returnEntryCount(v int, a kop.ValueRef) (n int, kind2 uint8) {
	if a.K != kop.VConst {
		return v, 0
	}
	w := int(uint16(a.N))
	return w & (1<<kop.RetCounterPairShift - 1), uint8(w >> kop.RetCounterPairShift)
}
