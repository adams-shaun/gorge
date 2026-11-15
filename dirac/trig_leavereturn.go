package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// opLeaveReturn is the leave-return record's act (OpDelayedTrigger C and
// delayed.act; a kernel-internal number above every kop.Op, never in an
// op dispatch).
const opLeaveReturn = 230

// dlyLeaveGrave is delayed.when for a this-turn record fired by its object
// being put into a graveyard from the battlefield.
const dlyLeaveGrave = 3

// prepP3 sets anyLeaveRet and anyOnePoison.
func prepP3(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplOnePoison {
				g.anyOnePoison = true
			}
			for _, in := range a.Prog {
				if in.Op == kop.OpDelayedTrigger && in.C == opLeaveReturn {
					g.anyLeaveRet = true
				}
			}
		}
	}
}

// leaveReturnReg registers the resolving ability's leave-return record over
// Obj (no object: nothing is remembered, so it can never fire).
//
//go:noinline
func (s *State) leaveReturnReg(e *stackEntry, in *kop.Instr) {
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return
	}
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	// ver holds the registering source (gorge's delayed trigger's host,
	// Melira, is the trigger's source); the target is matched by id
	s.dly[s.ndly] = delayed{act: opLeaveReturn, ctrl: e.ctrl, obj: o, ver: objWord(e.obj), when: dlyLeaveGrave, creator: NoObj}
	s.ndly++
}

// leaveGraveFired queues the return trigger of each of o's leave-return
// records, consuming them: o was just put into a graveyard from the
// battlefield.
//
//go:noinline
func (s *State) leaveGraveFired(o Obj) {
	w := uint8(0)
	n := s.ndly
	for i := uint8(0); i < n; i++ {
		d := s.dly[i]
		if d.when == dlyLeaveGrave && d.obj == o {
			src := Obj(d.ver)
			s.enqueue(src, s.objs[src].ver, s.objs[src].card, abDelayed, d.ctrl, o, d.ctrl, int16(d.act))
			continue
		}
		s.dly[w] = d
		w++
	}
	s.ndly = w
}

// dropTurnLeave removes the this-turn leave-return records as a turn begins.
//
//go:noinline
func (s *State) dropTurnLeave() {
	w := uint8(0)
	for i := uint8(0); i < s.ndly; i++ {
		if s.dly[i].when == dlyLeaveGrave {
			continue
		}
		s.dly[w] = s.dly[i]
		w++
	}
	s.ndly = w
}

// leaveReturn resolves the return trigger: the card, still in a graveyard,
// returns to the battlefield under its owner's control (gorge ChangeZone
// Defined$ TriggeredCard Origin$ Graveyard); a token that ceased to exist or
// a card that moved on does nothing.
//
//go:noinline
func (s *State) leaveReturn(k *prep, e *stackEntry) {
	o := e.trigObj
	if s.objs[o].zone != kop.ZoneGraveyard {
		return
	}
	ctrl := s.objs[o].owner()
	s.detach(o)
	s.enterFromGrave(k, o, ctrl, false)
}
