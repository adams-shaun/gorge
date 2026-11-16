package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext PutCounterChoice (kop.ExtPutCounterChoice): PutCounter with a
// comma-listed CounterType$, "put your choice of a menace, trample, reach, or
// haste counter on target Dinosaur" (Owen Grady) and "enters with your choice
// of a deathtouch counter or a lifelink counter" (Boot Nipper). gorge
// (effects counters_put.go, the counterKinds > 1 arm) asks the resolving
// controller one Min=Max=1 "counter_kind" KChoose over the listed kinds in
// list order, always (a list of two or more kinds is never answered unasked),
// before it looks at the recipients; one counter of the answer is then put.
// The kernel poses it as a ChRemCtrKind decision (mode = the counter kind);
// the decision's options sit in canonical order, kernel kind order, and the
// oracle compares gorge's list-order options by kind (canon counterKindAction).
//
// Two forms. The resolving form (Obj the recipient) asks mid-resolution
// (raRemCtrAny, as ExtPutCounterExisting does) and puts an ordinary
// OpPutCounter of the picked kind: replacements, the CR 616.1 order ask, and
// the counter-added triggers all apply. The as-enters form (Obj RefReplaced,
// the ETB$ True body in an enters program) asks once the permanent is on the
// battlefield, through ExtEnterCtrPick's ask (akCtrPick, repl_enterctrpick.go),
// and puts the counter on the entered permanent. The kinds ride B and C
// (kop.PutCtrChoiceOperands).

// putCounterChoice is ExtPutCounterChoice. The as-enters form is a no-op here:
// runReplProgAs queues its ask. The resolving form asks, or, with the CR 616.1
// order answered, puts the kind picked before it. false in ok: the ask or the
// order ask is posed.
//
//kmanifest:ext ExtPutCounterChoice
func (s *State) putCounterChoice(k *prep, in *kop.Instr) (ok, done bool) {
	if in.Obj == kop.RefReplaced {
		return true, true
	}
	r := &s.res
	if s.g.ro.fin && s.g.ro.ac&ctrOrdTag == ctrOrdTag {
		return s.pccPut(k, in, r.peek[0]), true // the order answered: the same pick again
	}
	r.ask, r.askP = raRemCtrAny, r.e.ctrl
	return false, true
}

// pccOptions fills the pick: every offered kind, once each.
func pccOptions(d *Decision, in *kop.Instr) {
	d.Sub = ChRemCtrKind
	ks, n := in.PutCtrChoiceKinds()
	for _, c := range ks[:n] {
		d.add(MakeAction(AkChoose, 0, 0, c))
	}
}

// pccAnswer takes the resolving form's pick and puts the counter.
func (s *State) pccAnswer(k *prep, in *kop.Instr, a Action) {
	s.res.ask = raNone
	if s.pccPut(k, in, Obj(a.Mode())) { // code-ok: a counter kind
		s.continueRes(k, 0)
	} else {
		s.ctl = ctlResolve // the CR 616.1 order ask
	}
}

// pccPut puts one counter of the picked kind; false when the put parked on
// the order ask.
func (s *State) pccPut(k *prep, in *kop.Instr, kind Obj) bool {
	s.res.peek[0] = kind
	s.g.lastCK = kop.CounterKind(kind) // code-ok: a counter kind (CounterAddedOnce reads it)
	pi := kop.Instr{Op: kop.OpPutCounter, Obj: in.Obj, A: kop.Const(1), C: int16(kind)}
	return s.execPutCounter(k, &pi)
}

// pccEnterAnswer applies the as-enters pick: one counter of the picked kind on
// the entered permanent (placed as an ordinary put, like ctrPickAnswer's).
func (s *State) pccEnterAnswer(k *prep, ask askRec, a Action) {
	p := &s.perms[ask.sl]
	if s.objs[p.obj].zone == kop.ZoneBattlefield {
		s.placeCounters(k, ask.sl, int(a.Mode()), 1, p.ctrl, ctrPutEffect)
	}
}
