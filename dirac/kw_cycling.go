package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// cycleDiscard is discardObj for a cycling ability's discard-this cost
// (AfCycling): gorge's cost discard is one event that both its Discarded
// and its Cycled triggers match, so the battlefield listeners are walked
// once, each ability in order, a Tf2Cycled one matched as a cycle
// (cycledTrigGen) and the others as the discard (playerTrigGen); then the
// cycled card's own triggers (fireCycled).
//
//go:noinline
func (s *State) cycleDiscard(k *prep, o Obj, x uint8) {
	from := s.detach(o)
	s.placeFrom(o, kop.ZoneGraveyard, false, from, false)
	s.noteDisc(o) // value_discardedthisturn.go
	if k.mayhem {
		s.mayhemMark(k, o) // kw_mayhem.go
	}
	s.g.disc = o
	who := s.objs[o].owner()
	if s.subs[kop.TrigDiscarded].any() {
		s.eachTrigger(k, kop.TrigDiscarded, func(l trigListener, a *kop.Ability) {
			if a.Trig.Flags2&kop.Tf2Cycled != 0 {
				s.cycledTrigGen(k, l, a, o, who)
				return
			}
			s.playerTrigGen(k, kop.TrigDiscarded, l, a, who, 1)
		})
	}
	if k.gen != nil && k.gen.anyDiscAll {
		s.discAllNote(k, o)
	}
	s.fireCycled(k, o, x)
	if k.gen != nil && k.gen.anySelfDisc {
		s.fireSelfDisc(k, o, false) // the cost discard is also a discard (trig_params.go)
	}
}

// cycledTrigGen queues a battlefield "whenever [a player] cycles [a card]"
// trigger (gorge cycledMatches: eventCardAndPlayerMatch over the cycled
// card and its owner, who cycled it).
func (s *State) cycledTrigGen(k *prep, l trigListener, a *kop.Ability, o Obj, who uint8) {
	t := &a.Trig
	if !whoseOK(t.Whose, l.ctrl, who) {
		return
	}
	if t.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl || t.Flags2&kop.Tf2OppTurn != 0 && s.active == l.ctrl {
		return
	}
	if !s.matchCardGen(k, t.Filter, s.objs[o].card, who, l.ctrl, o) || !s.trigOK(k, l, a) {
		return
	}
	s.queueTrig(k, l, a, o, who, 1)
}

// fireCycled fires the cycled card's own "when you cycle this card"
// triggers (Tf2Cycled), in ability order, from wherever the cycling
// cost's discard put it (gorge cycledMatches: the cost discard tagged with
// the cycling ability; trigger_zone_gate.go admits the card's own trigger
// in its new zone). Its owner, who cycled it, controls them.
//
//go:noinline
func (s *State) fireCycled(k *prep, o Obj, x uint8) {
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbTriggered || a.Trig.Flags2&kop.Tf2Cycled == 0 || a.Trig.Zone != kop.ZoneGraveyard {
			continue
		}
		p := s.objs[o].owner()
		l := trigListener{obj: o, sl: NoSlot, ctrl: p, card: c, ab: uint8(j)}
		if !s.trigOK(k, l, a) {
			continue
		}
		n := s.npend
		s.queueTrig(k, l, a, o, p, 0)
		if a.Trig.Flags2&kop.Tf2XBound != 0 && s.npend > n {
			// Mode$ Cycled with Count$xPaid: the X announced for the cycling
			// activation (gorge trigger_referents.go e.cast.x), not the
			// graveyard card's spell X
			s.pend[s.npend-1].x = x
		}
	}
}
