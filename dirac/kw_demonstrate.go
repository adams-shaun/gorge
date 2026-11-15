package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Demonstrate (OpDemonstrate): the Demonstrate keyword's cast trigger
// (cards/kw_demonstrate.go) resolves by asking the spell's controller
// whether to copy it, then, on yes, copying it once for that controller and
// once for each living opponent (gorge effects/demonstrate.go). Each copy's
// controller may choose new targets (CR 702.144a), inside this resolution. A
// copy is not cast, so it cannot re-fire.
//
// This kernel is two-player: the single opponent's copy follows the
// caster's with no opponent pick (gorge asks only when several opponents
// could copy, the strict-supersets convention).

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

// demonstrateCopy mints the demonstrate copies after a yes answer: the
// caster's, then each living opponent's in turn order (gorge effDemonstrate);
// left is how many are still to make (demoStart: all of them). False: a
// copy's election is posed, or the game stalled.
func (s *State) demonstrateCopy(k *prep, left uint8) bool {
	in := &s.prog(k)[s.res.pc]
	si := s.spellStackIndex(s.objRefObj(in.Obj))
	if si < 0 {
		return true
	}
	caster := s.res.e.ctrl
	var who seatSeq
	who.add(caster)
	opps := s.oppsFrom(caster)
	for i := uint8(0); i < opps.n; i++ {
		if q := opps.at(i); !s.pl[q].lost {
			who.add(q)
		}
	}
	if left == demoStart {
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
	return true
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
	n := s.g.hist[p][histCastN]
	hidden := n > 0 && Obj(s.cold.castRing[int(p)*castRingN+int(n)-1].obj-1) == e.obj
	if hidden {
		s.g.hist[p][histCastN] = n - 1
	}
	g, _ := s.stackKwN(k, kop.StackKwDemonstrate, e.card, e.obj, p, s.castFromHandBy(e, p))
	if hidden {
		s.g.hist[p][histCastN] = n
	}
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
