package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// startingLife is each player's starting life total: 40 in a Commander game
// (CR 903.7), 20 otherwise (CR 119.1).
func (s *State) startingLife() int {
	if s.cfg&cfgCommander != 0 {
		return 40
	}
	return 20
}

// noCommanders is a Config.Commanders that seats no commander anywhere.
func noCommanders() [MaxSeats][2]int16 {
	var c [MaxSeats][2]int16
	for i := range c {
		c[i] = [2]int16{-1, -1}
	}
	return c
}

// cmdGenesis puts seat p's commanders, cards idx of its deck (-1 none; two
// for a partner pair, CR 702.124), into the command zone before the seat's
// shuffle and opening hand (CR 903.6), as gorge's genesis does: in Config
// order, each leaving the ordered library, the rest keeping their relative
// order. Both indices name the deck as dealt, so both cards are found before
// either moves. An index outside the deck seats nothing (gorge skips it the
// same way); the seated commanders fill slot 0 first, as gorge's Commanders
// list holds only the valid ones.
func (s *State) cmdGenesis(p uint8, idx [2]int16) {
	pl := &s.pl[p]
	var os [2]Obj
	n := 0
	for _, ix := range idx {
		if ix < 0 || int(ix) >= pl.libLen() {
			continue
		}
		o := pl.lib[int(pl.libLo)+int(ix)]
		if n == 1 && os[0] == o {
			continue // one card named twice is one commander
		}
		os[n] = o
		n++
	}
	for j := 0; j < n; j++ {
		o := os[j]
		pl.libRemove(o)
		s.touch(o)
		s.objs[o].zone = kop.ZoneCommand
		if j == 0 {
			pl.cmd = o
		} else {
			s.partnerCold().cmd[p] = o
		}
		s.noteCommandZone(o)
	}
}

// satAdd8 is a + b saturating at 255.
func satAdd8(a, b uint8) uint8 {
	if t := int(a) + int(b); t < 255 {
		return uint8(t)
	}
	return 255
}
