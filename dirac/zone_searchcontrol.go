package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Opposition Agent (CR 723.2; kgen lower_searchcontrol.go):
//
//   - kop.StatSearchControl, "you control your opponents while they're
//     searching their libraries": gorge searchControlRedirect poses a
//     library search's pick (ResumeKind "search", the kernel's raGenSearch
//     over an OpSearchEx) to the static's controller when an opponent of that
//     controller would answer it. The pick's options are the searched
//     library's cards as before; only the answering seat moves. The
//     election and confirm asks a search may pose before the pick are not
//     "search" decisions in gorge and keep their seat.
//   - kop.ReplFoundSearchExile, "while an opponent is searching their
//     library, they exile each card they find. You may play those cards for
//     as long as they remain exiled, and you may spend mana as though it were
//     mana of any color to cast them": gorge replacementMatches applies the
//     Moved line to each move a library search emits (BeginLibrarySearch's
//     scope) of a card from the library the line's controller does not own,
//     while the searcher is not that controller. The card is exiled instead
//     and a permanent while-exiled permission record with the any-colour
//     rider is the controller's (gorge's Duration$ Permanent Effect with
//     ForgetOnMoved$ Exile: the record ends when the card leaves exile, its
//     version moving).

// useSearchControl / useFoundSearchExile arm the two reads for a table
// holding the kind.
var (
	useSearchControl    = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatSearchControl) })
	useFoundSearchExile = newGenUse(func(t *kop.Table) bool { return tableHasReplKind(t, kop.ReplFoundSearchExile) })
)

// DebugSearchControlOff and DebugFoundSearchExileOff skip the two reads (the
// mutations: the searcher answers their own pick; the found card goes where
// the search sends it).
var DebugSearchControlOff, DebugFoundSearchExileOff bool

// SearchControlStats counts the search picks a StatSearchControl static
// redirected, the found cards ReplFoundSearchExile exiled and the order asks
// posed among several lines; the lockstep test asserts them non-zero.
type SearchControlStats struct{ Redirected, Exiled, Asked int64 }

// DebugSearchControlStats, when set, receives the counts.
var DebugSearchControlStats *SearchControlStats

// searchPickChooser is who answers the library search pick q would answer:
// q, or the controller of the first live StatSearchControl static (static
// walk order, gorge's activeStatics order) that q is an opponent of.
func (s *State) searchPickChooser(k *prep, q uint8) uint8 {
	if g := k.gen; g == nil || !g.uses.has(useSearchControl) || DebugSearchControlOff {
		return q
	}
	if pr := s.prog(k); int(s.res.pc) >= len(pr) || pr[s.res.pc].Op != kop.OpSearchEx {
		return q
	}
	to := q
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatSearchControl || st.ctrl == q || !s.statCond(k, st) {
			return true
		}
		to = st.ctrl
		return false
	})
	if to != q && DebugSearchControlStats != nil {
		DebugSearchControlStats.Redirected++
	}
	return to
}

// foundSearchExile applies the live ReplFoundSearchExile lines to look card i
// (o) about to leave the library in the library search in progress, and
// reports whether it took o: one line exiles o and its controller gets the
// permission record; two or more pose gorge's CR 616.1 order ask to o's owner
// (raFoundSearchOrder), the look cut to the cards from o on. The ask parks on
// s.g.ro's header as the life-gain order ask does (repl_lifeask.go): n and
// cand the lines' hosts, used the ones applied, p the asked owner.
func (s *State) foundSearchExile(k *prep, o Obj, i int) bool {
	if g := k.gen; g == nil || !g.uses.has(useFoundSearchExile) || DebugFoundSearchExileOff || s.objs[o].zone != kop.ZoneLibrary {
		return false
	}
	if pr := s.prog(k); !s.res.active || int(s.res.pc) >= len(pr) || pr[s.res.pc].Op != kop.OpSearchEx {
		return false
	}
	cand, n, ok := s.foundSearchScan(k, s.g.pickP, s.objs[o].owner())
	switch {
	case !ok:
		return true // a loud stall
	case n == 0:
		return false
	case n == 1 || s.g.ro.n != 0:
		// s.g.ro.n != 0: the header is another ask's; the lines all exile
		// the card for the same controller, so only the asks' count differs
		if n > 1 {
			s.stall(StallLowering)
			return true
		}
		s.foundSearchApply(k, o, s.perms[s.objs[cand[0]].slot].ctrl)
	default:
		s.g.nlook = uint8(copy(s.resArm().look[:], s.resArm().look[i:s.g.nlook]))
		s.g.ro.replHdr = replHdr{n: uint8(n), p: s.objs[o].owner(), cand: cand}
		s.res.ask, s.res.askP = raFoundSearchOrder, s.objs[o].owner()
		if DebugSearchControlStats != nil {
			DebugSearchControlStats.Asked++
		}
	}
	return true
}

// foundSearchApply exiles o and grants ctrl the permanent while-exiled,
// any-colour permission record over it.
func (s *State) foundSearchApply(k *prep, o Obj, ctrl uint8) {
	from := s.detach(o)
	s.placeFromK(k, o, kop.ZoneExile, false, from, false, false)
	if DebugSearchControlStats != nil {
		DebugSearchControlStats.Exiled++
	}
	if s.objs[o].zone != kop.ZoneExile {
		return
	}
	if s.mayFull() {
		s.capFull(StallMayPlay)
		return
	}
	*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: ctrl, until: kop.DurPermanent, kind: kop.MayPlayIgnoreColor, turn: mayWord(s.turn)}
	s.g.nmay++
}

// foundSearchScan lists the hosts of the live ReplFoundSearchExile lines
// (battlefield slot order) that apply to a card owned by owner found by
// searcher: a controller that is neither the searcher (gorge: the line is
// inert on its controller's own search) nor the owner (Card.OppOwn). ok is
// false after a loud stall: more lines than the order header holds.
func (s *State) foundSearchScan(k *prep, searcher, owner uint8) (cand [4]Obj, n int, ok bool) {
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		ctrl := s.perms[sl].ctrl
		if ctrl == searcher || ctrl == owner || s.genLost(k, sl) || s.faceDown(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplFoundSearchExile {
				if n == len(cand) {
					s.stall(StallLowering)
					return cand, 0, false
				}
				cand[n] = s.perms[sl].obj
				n++
			}
		}
	}
	return cand, n, true
}

// foundSearchOrderOptions poses the order ask: the lines not yet applied, one
// per candidate, by scan index (gorge's KReplacement order ask).
//
//kmanifest:askopts raFoundSearchOrder
func (s *State) foundSearchOrderOptions(_ *prep, d *Decision) {
	ro := &s.g.ro
	d.Kind, d.Sub, d.Player = DkChoose, ChGenReplOrder, ro.p
	for i := 0; i < int(ro.n); i++ {
		if ro.used&(1<<i) == 0 {
			d.add(MakeAction(AkChoose, TCode(ro.cand[i]), 0, uint8(i)))
		}
	}
}

// foundSearchOrderAnswer marks the picked line applied and asks again while
// two or more remain (CR 616.1e: each line still applies to the modified
// move, and gorge re-collects after each pick); then the card is exiled, the
// header released, and the search's remaining moves and tail run.
//
//kmanifest:askapply raFoundSearchOrder
func (s *State) foundSearchOrderAnswer(k *prep, a Action) {
	ro := &s.g.ro
	i := int(a.Mode())
	if i >= int(ro.n) || ro.used&(1<<i) != 0 || s.g.nlook == 0 {
		s.stall(StallLowering) // a hand-built answer, or the candidates changed under the ask
		return
	}
	ro.used |= 1 << i
	if int(ro.n)-bits.OnesCount8(ro.used) >= 2 {
		return // asked again
	}
	ctrl := s.perms[s.objs[ro.cand[i]].slot].ctrl
	s.g.ro = replOrd{}
	s.res.ask = raNone
	o := s.resArm().look[0]
	s.foundSearchApply(k, o, ctrl)
	s.g.nlook = uint8(copy(s.resArm().look[:], s.resArm().look[1:s.g.nlook]))
	s.finishSearchNow(k)
	if s.res.ask != raNone {
		return
	}
	s.continueRes(k, 0)
}
