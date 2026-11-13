package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Will of the council over a card ballot (CR 701.38a; gorge effCardVote,
// engine/effects/vote_effect.go). The ballot is every battlefield permanent
// matching the op's filter from the resolving controller's view. Starting
// with that controller, each player still in the game votes for one of them
// (gorge askCardVote: one KChoose per voter, Min = Max = 1, option kind
// "vote_card"). Every permanent with the most votes or tied for most is
// remembered, in ballot order (seat 0's battlefield first, entry order); with
// C bit 0 they are then exiled together (Council's Judgment's sub-ability,
// which gorge moves as one simultaneous departure batch).
//
// The votes cast so far live in res.arr[:res.left] (one Obj per voter), so
// the tally needs no scratch of its own: it is counted once every vote is in.
// The ballot is recomputed for each ask; nothing changes between the asks.

// raVote is the vote ask. It sits past the ask families that offset from
// raGenManifestDread (+1..+3, +40..+45, +50, +60, +61, +70..+77).
const raVote = raGenManifestDread + 78

// voteBallot is the vote's candidates: the battlefield slots matching in.F
// for the resolving controller.
func (s *State) voteBallot(k *prep, in *kop.Instr) slotMask {
	e := &s.res.e
	src := s.objRef(k, kop.RefSelf)
	var m slotMask
	for occ := s.occ; occ.any(); occ = occ.rest() {
		if sl := occ.first(); s.matchPerm(k, in.F, sl, e.ctrl, src) {
			m = m.with(sl)
		}
	}
	return m
}

// voteNext poses the vote of the next player in turn order from the
// controller who has not voted and is still in the game; false when every
// voter is done.
func (s *State) voteNext() bool {
	for i := s.res.chosen; i < 2; i++ {
		p := s.res.e.ctrl ^ i
		if !s.pl[p].lost {
			s.res.chosen = i + 1
			s.res.ask, s.res.askP = raVote, p
			return true
		}
	}
	return false
}

// voteStart is OpExt ExtVote: the first voter's ask, or nothing at all with
// an empty ballot (gorge asks nobody and remembers nothing).
func (s *State) voteStart(k *prep, in *kop.Instr) bool {
	if !s.voteBallot(k, in).any() {
		return true
	}
	s.res.left, s.res.chosen = 0, 0
	return !s.voteNext()
}

// voteOptions poses one voter's ballot: one AkChoose per candidate.
func (s *State) voteOptions(k *prep, d *Decision) {
	d.Sub = ChVoteCard
	for m := s.voteBallot(k, &s.prog(k)[s.res.pc]); m.any(); m = m.rest() {
		d.add(MakeAction(AkChoose, TCode(s.perms[m.first()].obj), 0, 0))
	}
}

// voteAnswer records a vote, then poses the next voter's ask or, once every
// vote is in, remembers the most-voted candidates and goes on.
func (s *State) voteAnswer(k *prep, a Action) {
	s.res.arr[s.res.left] = Obj(a.Source())
	s.res.left++
	if s.voteNext() {
		return
	}
	in := &s.prog(k)[s.res.pc]
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.voteBallot(k, in), &ord)
	var votes [MaxPerm]uint8
	best := uint8(0)
	for i := 0; i < n; i++ {
		o := s.perms[ord[i]].obj
		for _, v := range s.res.arr[:s.res.left] {
			if v == o {
				votes[i]++
			}
		}
		best = max(best, votes[i])
	}
	var won slotMask
	for i := 0; i < n && best > 0; i++ {
		if votes[i] == best {
			s.remember(s.perms[ord[i]].obj)
			won = won.with(ord[i])
		}
	}
	if in.C&1 != 0 && won.any() { // seat-ok: an instruction flag bit
		s.leaveAll(k, won, kop.ZoneExile) // one simultaneous departure (gorge's departure batch)
	}
	s.continueRes(k, 0)
}
