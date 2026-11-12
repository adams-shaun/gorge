package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Draw up to N (gorge effDraw's Upto$ True; Arcane Denial's "its controller
// may draw up to two cards"). The drawer is asked how many cards to draw: a
// KChoose of Min 0 / Max min(N, library size) whose options are the top cards
// of their own library in order, the answer's size the count (gorge's
// "draw_upto" ask). One DkChoose ChDrawUpto pick per card, Done ends the
// count early; the picks are only counted, then drawn one by one. An empty
// library asks nobody and draws nothing (never a mandatory draw).

// raDrawUpto is the draw-up-to ask (res.askP the drawer, res.left the most
// cards, res.peek[:res.peekN] the picks so far).
const raDrawUpto = raGenManifestDread + 81

// drawUptoStart is OpExt ExtDrawUpto: the drawer's ask, or nothing at all
// when the count is 0 or the library empty.
func (s *State) drawUptoStart(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return true
	}
	m := min(s.value(k, in.B), s.pl[p].libLen(), len(s.res.peek))
	if m <= 0 {
		return true
	}
	s.res.peekN, s.res.left = 0, uint8(m)
	s.res.ask, s.res.askP = raDrawUpto, p
	return false
}

// drawUptoOptions poses the pick: each top card not yet picked, library
// order, then Done.
func (s *State) drawUptoOptions(d *Decision) {
	d.Sub = ChDrawUpto
	pl := &s.pl[s.res.askP]
	for i := 0; i < int(s.res.left); i++ {
		o := pl.lib[int(pl.libLo)+i]
		picked := false
		for j := 0; j < int(s.res.peekN); j++ {
			picked = picked || s.res.peek[j] == o
		}
		if !picked {
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// drawUptoAnswer records a pick; at Done or the last card the drawer draws
// as many cards as were picked and the program goes on.
func (s *State) drawUptoAnswer(k *prep, a Action) {
	r := &s.res
	if a.Kind() != AkDone && r.peekN < uint8(len(r.peek)) {
		r.peek[r.peekN] = Obj(a.Source())
		r.peekN++
		if r.peekN < r.left {
			return
		}
	}
	n, p := int(r.peekN), r.askP
	r.ask, r.peekN = raNone, 0
	for i := 0; i < n && !s.over; i++ {
		s.draw(p)
	}
	s.continueRes(k, 0)
}
