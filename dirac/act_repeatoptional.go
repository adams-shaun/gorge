package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// RepeatOptional$ (gorge effects/resolve.go effRepeat): the body runs, and
// after every pass that changed something the resolving controller is asked
// "Repeat this process?" (repeat_optional, a KChoose of yes / no); a yes runs
// the body again, a no ends the loop. A pass that posed no decision and
// changed nothing ends the loop unasked (gorge's CR 732.2a shortcut: its
// repeat could only reproduce the same no-op).
//
// The lowering (kgen lower_repeatoptional.go) is an OpRepeatWhile that only
// its cap ends, its body bracketed by two ExtRepeatOptional ops. The head op
// (B.N 0) judges the pass as it starts. A pass whose progress condition C
// holds (Ad Nauseam: your library holds a card) moves a card, a change. One
// that starts without it moves nothing, and gorge's change test
// (rules.Engine.StateChangedSince) then sees only the body's LoseLife: a 0
// life loss is no LifeChange, but the AFLifeLost StoreSVar it always writes
// is a change unless it repeats the value the previous write to the source
// held. So the empty pass changed something when it is the first pass (no
// earlier write since the spell was cast) or when the previous pass lost life
// (the kernel's last life loss, ExLastLifeLost, is that pass's); otherwise it
// is made the loop's last (loopN 1). The tail op (B.N 1) poses the election
// unless the pass is the last.

// DebugRepeatOptionalOnce is the mutation switch: the election is never
// posed, so the body runs once. Tests only.
var DebugRepeatOptionalOnce bool

// repeatOptionalExt is OpExt ExtRepeatOptional; false: the election is posed.
//
//kmanifest:ext ExtRepeatOptional
func (s *State) repeatOptionalExt(k *prep, in *kop.Instr) (ok, done bool) {
	r := &s.res
	if r.loopD == 0 {
		return true, true
	}
	d := r.loopD - 1
	if in.B.N == 2 {
		// the pure head (Dance with Calamity's Dig-to-exile body): no write
		// but the moved card, so a pass that starts without the progress
		// condition changes nothing and is the last
		if !s.condEval(k, kop.CondID(in.C), s.ctxEntry(&r.e)) {
			r.loopN[d] = 1
		}
		return true, true
	}
	if in.B.N == 0 {
		first := r.loopN[d] == uint8(min(max(s.value(k, s.prog(k)[r.loopS[d]-1].A), 0), 255))
		if !s.condEval(k, kop.CondID(in.C), s.ctxEntry(&r.e)) && !first && s.g.lostAmt <= 0 {
			r.loopN[d] = 1
		}
		return true, true
	}
	if r.loopN[d] <= 1 || DebugRepeatOptionalOnce {
		r.loopN[d] = 1
		return true, true // the last pass (no change, or the cap): no election
	}
	r.ask, r.askP = raGenRepeatOptional, r.e.ctrl
	return false, true
}

// repeatOptionalOptions poses gorge's options: yes (repeat), then no (stop).
//
//kmanifest:askopts raGenRepeatOptional
func (s *State) repeatOptionalOptions(k *prep, d *Decision) {
	d.Sub = ChGenConfirm
	d.add(MakeAction(AkChoose, 0, 0, 1))
	d.add(MakeAction(AkChoose, 0, 0, 0))
}

// repeatOptionalAnswer applies the election: a no makes this pass the loop's
// last, and the program goes on past the tail op either way.
//
//kmanifest:askapply raGenRepeatOptional
func (s *State) repeatOptionalAnswer(k *prep, a Action) {
	r := &s.res
	if a.Mode() == 0 && r.loopD > 0 {
		r.loopN[r.loopD-1] = 1
	}
	s.continueRes(k, 0)
}
