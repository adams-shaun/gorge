package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Riot card returning "until this leaves" behind a returning Aura's
// queued host ask (queueAskRet: Abdel Adrian, Gorion's Ward dying to a
// state-based action returns a Spirit Mantle and a Clamor Shaman). gorge
// asks the Aura's host in the middle of the batch of returns and parks
// every later entry whose CR 702.136a election is due behind it, with the
// card still in exile (replacement_etb.go queuesBehindAsk): the Aura is
// asked and enters, then the Riot card's election is asked and the card
// enters with it. The kernel queues the election the same way: an ask with
// no permanent slot (the Aura's shape), the card's object in ab.

// queueRiotRet queues the Riot election of returning card ret behind a
// queued Aura host ask. It reports whether the card waits in exile.
func (s *State) queueRiotRet(k *prep, srcCtrl uint8, o, ret Obj) bool {
	x := &s.objs[ret]
	if s.res.active || !s.retAuraAskQueued() || !k.riot || !k.t.Cards[x.card].Riot || s.g.pre.state != 0 {
		return false
	}
	if int(s.g.nask) >= len(s.g.ask) {
		s.capFull(StallAsk)
		return false
	}
	a := &s.g.ask[s.g.nask]
	a.sl, a.ver, a.kind, a.card, a.ab, a.pc, a.ts = NoSlot, x.ver, akRiot, x.card, objWord(ret), 0, s.ts
	s.g.nask++
	if k.gen.anyLinkCtr {
		s.linkRetStall(k, srcCtrl, o, ret)
	}
	return true
}

// retRiotAsk poses a queued returning card's Riot election (queueRiotRet),
// or drops it when the card left exile meanwhile.
func (s *State) retRiotAsk(d *Decision, a *askRec) bool {
	o := Obj(a.ab)
	if x := &s.objs[o]; x.zone == kop.ZoneExile && x.ver == a.ver {
		d.Player, d.Kind = x.owner(), DkChoose
		s.riotOptions(d, o)
		return true
	}
	s.popAsk()
	return false
}

// enterRetRiot enters returning card o from exile with the answered
// election: the entry's riotEntryAsk reads it from s.g.pre, as a cast
// creature's pre-entry answer.
func (s *State) enterRetRiot(k *prep, o Obj, mode uint8) {
	if s.objs[o].zone != kop.ZoneExile {
		return
	}
	save := s.g.pre
	s.g.pre = preAsk{obj: o, kind: akRiot, state: 2, val: objWord(mode)}
	from := s.detach(o)
	s.enterFromGrantedZone(k, o, s.objs[o].owner(), false, from)
	s.g.pre = save
}
