package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// untapStepGen is the untap step's turn-based action on a table with a
// ReplOthersDoesntUntap or ReplAnyUntap card (gorge turn.go finishUntapStep): the active
// player's permanents in battlefield order, each losing summoning sickness;
// a tapped one with a stun counter loses it instead (CR 122.1d); otherwise
// its Untap event meets the "doesn't untap" replacements (its own printed
// one, and every other permanent's whose filter matches it, as gorge's scan
// collects them: each player's battlefield in entry order, player 0 first).
// None: it untaps. One or more: it stays tapped, with no CR 616.1 order ask,
// since "doesn't untap" is no replacement (CR 502.3, 614.1). Untaps triggers
// fire for the permanents untapped at the end, in battlefield order.
//
//go:noinline
func (s *State) untapStepGen(k *prep) {
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(s.active, s.ctrlM[s.active], ord[:], 0)
	from := 0
	var up slotMask
	if s.g.untapOn {
		// resumed after a keep-tapped ask (repl_untapchoice.go): the scan
		// goes on from the asked permanent
		from, up = int(s.g.untapNext), s.g.untapUp
	} else {
		// CR 502.3: gorge folds the sickness clearing into the StepChange event
		// (apply_turn.go), before finishUntapStep scans.
		for _, sl := range ord[:n] {
			if p := &s.perms[sl]; p.flags&pfSick != 0 {
				p.flags &^= pfSick
				s.touchSl(sl)
			}
		}
	}
	for i := from; i < n; i++ {
		sl := ord[i]
		p := &s.perms[sl]
		f := p.flags
		if p.flags&pfSkipUntap != 0 {
			p.flags &^= pfSkipUntap // CR 611.2b one-shot (turn.go's rule)
			s.touchSl(sl)
			continue
		}
		if p.flags&pfTapped != 0 && k.gen.anySkipUntap && s.skipsUntap(k, sl) {
			// gorge's Effect replacement (Layer$ CantHappen): it stays tapped
		} else if p.flags&pfTapped != 0 {
			if k.cards[s.cardAt(sl)].mayNotUntap {
				// CR 502.3: asked before the untap event meets any replacement
				// (gorge finishUntapStep), and once per permanent
				if s.g.untapAns == 0 {
					s.untapAsk(i, sl, up)
					return
				}
				keep := s.g.untapAns == 2
				s.g.untapAns = 0
				if keep {
					continue
				}
			}
			c := s.untapCands(k, sl)
			if s.g.stalled != 0 {
				return // more than the ask holds: the game stops here, not at the next ask
			}
			s.g.ro.replHdr = replHdr{} // "doesn't untap" is no replacement (CR 502.3, 614.1), so no order ask at any count
			switch {
			case c == 0 && p.ctrLow(int(kop.CtrStun)) > 0:
				p.seeCtr(s)
				p.subCtrN(s, int(kop.CtrStun), 1) // CR 122.1d: only a permanent that would untap spends one
				s.touchSl(sl)
				continue
			case c == 0:
				p.flags &^= pfTapped
				s.untapEnds(k, p.obj) // the lock and the host-tapped grants end at the first untap (static_animatewhiletapped.go)
				up = up.with(sl)
			}
		}
		if p.flags != f {
			s.touchSl(sl)
		}
	}
	s.g.untapOn = false
	s.fireUntapsOrdered(k, up, &ord, n)
	if k.gen.anyUntapOther {
		s.untapOthers(k)
	}
	if k.gen.anySkipUntap {
		s.expireSkipUntap() // CR 611.3b: those effects end with this untap step
	}
	s.step = kop.StepUpkeep
}

// untapCands counts the "doesn't untap" replacements matching tapped
// permanent sl and records them as s.g.ro's candidates (at most four), in
// gorge's scan order.
func (s *State) untapCands(k *prep, sl Slot) int {
	ro := &s.g.ro
	ro.replHdr = replHdr{}
	c := 0
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		var bf [MaxPerm]Slot
		for _, b := range bf[:s.battlefieldOrder(pp, s.statM.and(s.ctrlM[pp]).with(sl).and(s.ctrlM[pp]), bf[:], 0)] {
			if s.genLost(k, b) {
				continue
			}
			cd := &k.t.Cards[s.cardAt(b)]
			for j := range cd.Abilities {
				a := &cd.Abilities[j]
				if a.Kind != kop.AbReplacement {
					continue
				}
				hit := a.Repl.Kind == kop.ReplDoesntUntap && b == sl ||
					(a.Repl.Kind == kop.ReplOthersDoesntUntap || a.Repl.Kind == kop.ReplAnyUntap) &&
						s.matchPermGen(k, a.Repl.Filter, sl, s.perms[b].ctrl, b, layerFull)
				if hit {
					if c == len(ro.cand) {
						s.capFull(StallReplCands) // loud: never truncate the candidates
						return 0
					}
					ro.cand[c] = s.perms[b].obj
					c++
				}
			}
		}
	}
	ro.n = uint8(c)
	return c
}

// fireUntapsOrdered fires Untaps triggers for the permanents in up, in the
// battlefield order ord.
func (s *State) fireUntapsOrdered(k *prep, up slotMask, ord *[MaxPerm]Slot, n int) {
	if !up.any() || !s.subs[kop.TrigUntaps].any() {
		return
	}
	for _, sl := range ord[:n] {
		if up.has(sl) && s.perms[sl].flags&pfTapped == 0 {
			s.fireObjGen(k, kop.TrigUntaps, sl, s.perms[sl].obj, s.perms[sl].ctrl, 0)
		}
	}
}
