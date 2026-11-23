package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Demonstrate (OpDemonstrate): the Demonstrate keyword's cast trigger
// (cards/kw_demonstrate.go) resolves by asking the spell's controller
// whether to copy it, then, on yes, copying it once for that controller and
// once for each living opponent (gorge effects/demonstrate.go). Each copy's
// controller may choose new targets (CR 702.144a), inside this resolution. A
// copy is not cast, so it cannot re-fire.
//
// "Choose an opponent to also copy it" (CR 702.152a): with two or more living
// opponents the caster picks one (raTrigDemoOpp, ChGenPlayer, mode = the seat,
// in AliveFrom order; gorge effDemonstrate stage 1) and only that opponent
// copies, after the caster. With one (or none) there is no pick (the
// strict-supersets convention) and the lone opponent's copy follows the
// caster's. gorge emits both copies only once the pick is settled, so the
// kernel mints nothing before it either. The pick is held in res.chPl (seat
// + 1) until the last copy is made.

// demonstrate runs OpDemonstrate; false: the election is posed.
func (s *State) demonstrate(k *prep, in *kop.Instr) bool {
	if s.spellStackIndex(s.objRefObj(in.Obj)) < 0 {
		return true
	}
	s.res.ask, s.res.askP = raGenDemonstrate, s.res.e.ctrl
	return false
}

// spellStackIndex is the stack index of spell src below the resolving
// entry, or -1 when it is no longer on the stack.
func (s *State) spellStackIndex(src Obj) int {
	if src == NoObj || s.objs[src].zone != kop.ZoneStack {
		return -1
	}
	for i := int(s.sp) - 2; i >= 0; i-- {
		if s.stack[i].kind == seSpell && s.stack[i].obj == src {
			return i
		}
	}
	return -1
}

// demoStart is demonstrateCopy's left after the yes answer: every copy is
// still to make.
const demoStart = 255

// demoPicked is demonstrateCopy's left once the opponent is picked: the same
// copies to make as demoStart, without posing the pick again.
const demoPicked = 254

// demonstrateCopy mints the demonstrate copies after a yes answer: the
// caster's, then each living opponent's in turn order (gorge effDemonstrate);
// left is how many are still to make (demoStart: all of them). False: a
// copy's election or the opponent pick is posed, or the game stalled.
func (s *State) demonstrateCopy(k *prep, left uint8) bool {
	in := &s.prog(k)[s.res.pc]
	si := s.spellStackIndex(s.objRefObj(in.Obj))
	if si < 0 {
		s.res.chPl = 0
		return true
	}
	caster := s.res.e.ctrl
	var who seatSeq
	who.add(caster)
	opps := s.oppsFrom(caster)
	if left == demoStart {
		s.res.chPl = 0
		if opps.n > 1 && !DebugDemoAllOpps {
			s.res.ask, s.res.askP = raTrigDemoOpp, caster
			return false
		}
	}
	if s.res.chPl != 0 {
		who.add(s.res.chPl - 1)
	} else {
		for i := uint8(0); i < opps.n; i++ {
			if q := opps.at(i); !s.pl[q].lost {
				who.add(q)
			}
		}
	}
	if left == demoStart || left == demoPicked {
		left = who.n
	}
	for left > 0 && left <= who.n {
		if !s.mintSpellCopy(k, si, who.at(who.n-left)) {
			return false
		}
		left--
		if s.copyElect(k, left) {
			return false
		}
	}
	s.res.chPl = 0
	return true
}

// DebugDemoAllOpps is the lockstep mutation of the demonstrate opponent pick:
// every living opponent copies, none asked, the two-seat reading
// (TestFourSeatDemonstrateLockstep must then diverge). Never set in production.
var DebugDemoAllOpps bool

// demoOppOptions fills raTrigDemoOpp: the caster's living opponents in
// AliveFrom order.
func (s *State) demoOppOptions(d *Decision) {
	d.Sub = ChGenPlayer
	q := s.oppsFrom(s.res.askP)
	for i := uint8(0); i < q.n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, q.s[i]))
	}
}

// demoOppAnswer keeps the picked opponent and makes the copies.
func (s *State) demoOppAnswer(k *prep, a Action) {
	s.res.ask = raNone
	s.res.chPl = a.Mode() + 1
	if s.demonstrateCopy(k, demoPicked) {
		s.continueRes(k, 0)
	}
}

// stormDemonstrate is the trigAmt of an abStorm entry that is a granted
// demonstrate's copy trigger (a storm count is never negative): resolving
// it runs demonstrateProg, the OpDemonstrate a printed Demonstrate card's
// self-cast trigger carries.
const stormDemonstrate int16 = -1

// demonstrateProg is the resolution of a granted demonstrate trigger (CR
// 702.144a, gorge effects/demonstrate.go): the election, then the copies.
var demonstrateProg = [1]kop.Instr{
	{Op: kop.OpDemonstrate, Obj: kop.RefTriggerObj},
}

// queueDemonstrate queues the demonstrate trigger of the spell e just cast
// when a live StatStackKw Demonstrate grant of its caster reaches it (gorge
// trigger_granted.go checkGrantedDemonstrateTriggers: a spell has the
// keyword or not, so two granting statics give one trigger, and a face that
// prints Demonstrate is skipped, its printed trigger owning the line).
//
// gorge reads the grant's CheckSVar$ gate with the in-flight cast's own push
// left out of the cast ledger (hasKeywordPriorCasts), so "the first
// nonlegendary creature spell you cast each turn" (Elda, Conjurer of
// Spectacle) holds for the very cast the grant is for. The cast ring already
// holds that push, the caster's latest, so the gate is read with it hidden.
func (s *State) queueDemonstrate(k *prep, e *stackEntry) {
	if s.printsDemonstrate(k, e.card) {
		return
	}
	p := e.ctrl
	saved := s.hideInflightCast(e)
	g, _ := s.stackKwN(k, kop.StackKwDemonstrate, e.card, e.obj, p, s.castFromHandBy(e, p))
	s.restoreInflightCast(p, saved)
	if g == 0 {
		return
	}
	s.enqueue(e.obj, s.objs[e.obj].ver, e.card, abStorm, p, e.obj, p, stormDemonstrate)
}

// printsDemonstrate: card c prints Demonstrate, a self-cast trigger whose
// program holds OpDemonstrate.
func (s *State) printsDemonstrate(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigSpellCast || a.Trig.Flags&kop.TfSelf == 0 {
			continue
		}
		for j := range a.Prog {
			if a.Prog[j].Op == kop.OpDemonstrate {
				return true
			}
		}
	}
	return false
}
