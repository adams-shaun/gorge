package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext MayPlayGrave (Case of the Uneaten Feast): "creature cards
// in your graveyard gain 'You may cast this card from your graveyard'
// until end of turn". Gorge's Animate reads Defined$ ValidGraveyard once,
// at resolution, so a card put there later gains nothing and a card that
// leaves and returns is a new object (the record's ver); each card's
// granted static is a self may-play grant, which the kernel's permission
// records (genState.may, DurEOT, dropped at cleanup) already model. 0 B.

// mayPlayGrave records a permission for each graveyard card matching F.
func (s *State) mayPlayGrave(k *prep, in *kop.Instr) {
	ctrl := s.res.e.ctrl
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		pl := &s.pl[p]
		for i := 0; i < int(pl.graveN); i++ {
			o := pl.grave[i]
			c := s.objs[o].card
			if !s.matchCardGen(k, in.F, c, p, ctrl, o) {
				continue
			}
			if in.B.N&kop.MayPlayNonLand != 0 && k.t.Cards[c].Types&kop.TLand != 0 {
				continue
			}
			if s.mayHas(o, ctrl) {
				continue // a second Case: one more grant of the same permission, offered once
			}
			if s.mayFull() {
				s.capFull(StallMayPlay)
				return
			}
			*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: ctrl, until: kop.DurEOT, turn: s.turn}
			s.g.nmay++
		}
	}
}

// mayHas reports whether an end-of-turn permission for the current version
// of o already stands for ctrl.
func (s *State) mayHas(o Obj, ctrl uint8) bool {
	for i := 0; i < int(s.g.nmay); i++ {
		m := s.mayAt(i)
		if m.obj == o && m.ver == s.objs[o].ver && m.ctrl == ctrl && m.dur() == kop.DurEOT && m.turn == s.turn {
			return true
		}
	}
	return false
}
