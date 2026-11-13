package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Earthbend N (CR 701.66; gorge effEarthbend,
// registerEarthbendReturn). Cold and out of line: a table with no
// OpEarthbend never takes these paths (genPrep.anyEarthbend).
//
// The land becomes a 0/0 Creature with haste for as long as it stays on the
// battlefield, gets N +1/+1 counters, and gets ONE leave-keyed delayed record
// (delayed.when == dlyLeave). When the land is put into a graveyard or
// exile, the record fires an OpEarthbendReturn trigger that returns the card
// from either zone, tapped, under its OWNER's control (gorge deviates from
// 701.66a the same way). gorge tracks no incarnation: the record is keyed by
// object id, never expires, and a bounced then replayed land that dies still
// returns.

// dlyLeave is delayed.when for a record fired by the object leaving the
// battlefield for a graveyard or exile, not by an end step.
const dlyLeave = 1

// DebugEarthbendNoReturn skips the return registration. The lockstep
// mutation test flips it and requires a divergence.
var DebugEarthbendNoReturn bool

// DebugEarthbendResolved, when set, observes one earthbend resolution: card is
// the resolving card and live is the number of earthbend return records
// registered after it. The lockstep uses it to assert every carrier resolves
// (not merely that it is cast) and to measure the peak simultaneous records.
// A nil hook -- every production caller -- changes nothing.
var DebugEarthbendResolved func(card CardID, live int)

// DebugEarthbendFull, when set, observes a full delayed table at an earthbend
// registration: card is the resolving card, earthbend is how many of the
// MaxDelayed records are leave-keyed earthbend returns, other the rest.
// Measurement only; a nil hook changes nothing.
var DebugEarthbendFull func(card CardID, earthbend, other int)

// prepEarthbend sets anyEarthbend: some program earthbends.
func prepEarthbend(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				g.anyEarthbend = g.anyEarthbend || in.Op == kop.OpEarthbend
			}
		}
	}
}

// execEarthbend runs OpEarthbend (execOblige's default branch); done=false
// means the op is not this one.
//
//go:noinline
func (s *State) execEarthbend(k *prep, in *kop.Instr) (ok, done bool) {
	if in.Op != kop.OpEarthbend {
		return false, false
	}
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return true, true // gorge skips a target no longer on the battlefield
	}
	sl := s.objs[o].slot
	// gorge evaluates Num once before the target loop and before the land
	// becomes a creature; read it here too (a Count$Valid Creature carrier
	// would otherwise see the animated land).
	n := s.value(k, in.A)
	s.applyOverrideFl(k, sl, int(in.C), s.res.e.card, 0) // DurPermanent: ends when it leaves
	if n > 0 {
		s.g.lastCK = kop.CtrP1P1
		n = s.placeCounters(k, sl, int(kop.CtrP1P1), n, s.perms[sl].ctrl, ctrPutEffect)
		p := &s.perms[sl]
		if n > 0 && k.anyCtrAdded {
			s.ctrNote(k, sl, int(kop.CtrP1P1))
		}
		if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
			s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(n))
		}
	}
	if DebugEarthbendNoReturn {
		if DebugEarthbendResolved != nil {
			DebugEarthbendResolved(s.res.e.card, int(s.ndly))
		}
		return true, true
	}
	// A token land is registered like any other: gorge's registration is
	// keyed by object id with no token test, so the return trigger still
	// queues when the token dies (CR 603.7); it resolves to nothing, the
	// token having ceased to exist (CR 111.8; earthbendReturn).
	if s.ndly >= MaxDelayed {
		if DebugEarthbendFull != nil {
			eb, other := 0, 0
			for i := uint8(0); i < s.ndly; i++ {
				if s.dly[i].when == dlyLeave {
					eb++
				} else {
					other++
				}
			}
			DebugEarthbendFull(s.res.e.card, eb, other)
		}
		s.capFull(StallDelayed)
		return true, true
	}
	s.dly[s.ndly] = delayed{act: uint8(kop.OpEarthbendReturn), ctrl: s.res.e.ctrl, obj: o, ver: s.objs[o].ver, when: dlyLeave, creator: s.res.e.obj}
	s.ndly++
	if DebugEarthbendResolved != nil {
		live := 0
		for i := uint8(0); i < s.ndly; i++ {
			if s.dly[i].when == dlyLeave {
				live++
			}
		}
		DebugEarthbendResolved(s.res.e.card, live)
	}
	return true, true
}

// splitLeaveDelayed partitions the n registered delayed records for the end
// step: the end-step records stay at dly[:m] (original order) and are
// returned as m; the leave-keyed ones go to dly[m:m+keep] for the caller to
// restore.
//
//go:noinline
func (s *State) splitLeaveDelayed(n uint8) (m, keep uint8) {
	var timed, leave [MaxDelayed]delayed
	for i := uint8(0); i < n; i++ {
		if d := s.dly[i]; d.when == dlyLeave || d.when == dlyLeaveGrave {
			leave[keep] = d
			keep++
		} else {
			timed[m] = d
			m++
		}
	}
	copy(s.dly[:m], timed[:m])
	copy(s.dly[m:m+keep], leave[:keep])
	return m, keep
}

// earthbendLeft queues the return trigger of every leave-keyed record of
// object o (matching by object id only, as gorge), in registration order,
// consuming the records. The land has just been put into a graveyard or
// exile.
//
//go:noinline
func (s *State) earthbendLeft(k *prep, o Obj) {
	w := uint8(0)
	n := s.ndly
	for i := uint8(0); i < n; i++ {
		d := s.dly[i]
		if d.when == dlyLeave && d.obj == o {
			s.enqueue(dlySource(&d), s.objs[o].ver, s.objs[o].card, abDelayed, d.ctrl, o, d.ctrl, int16(d.act))
			continue
		}
		s.dly[w] = d
		w++
	}
	s.ndly = w
}

// earthbendReturn resolves the return trigger: the card in a graveyard or
// exile (whatever its incarnation) returns to the battlefield tapped under
// its owner's control; a card elsewhere does nothing.
//
//go:noinline
func (s *State) earthbendReturn(k *prep, e *stackEntry) {
	o := e.trigObj // the land; e.obj is the source, what created the promise (CR 603.7d)
	ob := &s.objs[o]
	if ob.zone != kop.ZoneGraveyard && ob.zone != kop.ZoneExile {
		return
	}
	if s.earthbendReturnPark(k, o) {
		return
	}
	s.earthbendEnter(k, o)
}

// preEarthbendRet is preAsk.state for an earthbent card parked at its
// as-enters choice: asked with the card still in its zone, the entry
// follows the answer (earthbendEnter).
const preEarthbendRet uint8 = 14

// earthbendEnter puts the card onto the battlefield tapped under its owner.
func (s *State) earthbendEnter(k *prep, o Obj) {
	ctrl := s.objs[o].owner()
	from := s.detach(o)
	if from == kop.ZoneGraveyard {
		s.enterFromGrave(k, o, ctrl, true)
	} else {
		s.enter(k, o, ctrl, true, from)
	}
}

// earthbendReturnPark asks the returning card's "As it enters, choose ..."
// with the card still in its graveyard or exile (as returnLinked does;
// CR 614.12): the owner answers, then earthbendReturnAnswer enters it.
func (s *State) earthbendReturnPark(k *prep, o Obj) bool {
	if s.g.pre.state != 0 {
		return false
	}
	switch kind := s.enterChoiceKind(k, s.objs[o].card); kind {
	case akColor, akType, akNumber, akName, akNameAny:
		s.g.pre = preAsk{obj: o, kind: kind, state: preEarthbendRet}
		s.res.ask, s.res.askP = raGenRetChoice, s.objs[o].owner()
		s.ctl = ctlResolve
		return true
	}
	return false
}

// earthbendReturnAnswer enters the parked card with its answered choice
// (the enters program reads pre.val), then the trigger goes on.
//
//go:noinline
func (s *State) earthbendReturnAnswer(k *prep) {
	o := s.g.pre.obj
	s.g.pre.state = 2
	if z := s.objs[o].zone; z == kop.ZoneGraveyard || z == kop.ZoneExile {
		s.earthbendEnter(k, o)
	}
	s.g.pre = preAsk{}
	s.continueRes(k, 0)
}
