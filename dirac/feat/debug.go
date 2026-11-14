package feat

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
)

// DebugView renders the encoder's last view for a mismatch report.
func (e *Encoder) DebugView() string {
	v := &e.v
	s := fmt.Sprintf("turn %d step %d active %d prio %d\n", v.turn, v.step, v.active, v.priority)
	card := func(ci int32) string {
		c := &v.cards[ci]
		kws := ""
		e.eachKw(c, func(k *kwInfo) { kws += string(k.raw) + "," })
		return fmt.Sprintf("%s#%d[z%d c%d t%v s%v a%v blk%v %d/%d d%d kw=%s att=%d mana=%v]", c.c.name, c.obj, c.zone, c.ctrl, c.tapped, c.sick, c.attacking, c.blocked, c.power, c.tough, c.damage, kws, c.attached, c.manaSrc)
	}
	for p := 0; p < int(v.n); p++ {
		pl := &v.players[p]
		s += fmt.Sprintf(" p%d life %d hand %d lib %d grave %d avail %d\n  bf:", p, pl.life, pl.handN, pl.libN, pl.graveN, pl.avail)
		for _, ci := range pl.bf {
			s += " " + card(ci)
		}
		s += "\n  hand:"
		for _, ci := range pl.hand {
			s += " " + card(ci)
		}
		s += "\n  grave:"
		for _, ci := range pl.grave {
			s += " " + card(ci)
		}
		s += "\n  exile:"
		for _, ci := range pl.exile {
			s += " " + card(ci)
		}
		s += "\n"
	}
	s += " stack:"
	for _, st := range v.stack {
		s += " " + string(st.name.name)
		if st.cv >= 0 {
			s += "=" + card(st.cv)
		}
	}
	return s
}

// DebugObj reports the encoder's projection of target code t's object
// (zone, NObj).
func (e *Encoder) DebugObj(t dirac.TCode) string {
	if int(t) >= len(e.kpg) {
		return "zone=- (no object)"
	}
	return fmt.Sprintf("zone=%d nobj=%d", e.po(dirac.Obj(t)).Zone, e.kp.NObj)
}
