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
// askETBDeferred (askRec.ts) marks an as-enters counter pick or Devour ask
// whose entrant's enters triggers wait for the answer. gorge asks it with the
// permanent on the battlefield and puts the counters before its enters
// triggers are checked, so a condition that reads them sees them: Denry Klin,
// Editor in Chief's "if Denry has counters on it" triggers on Denry's own
// entry, and Evolve compares a devourer with its counters (CR 614.1c, 603.4,
// 702.100a).
const askETBDeferred uint16 = 1 << 15

// deferETBToEntryCounters marks the as-enters ExtPutCounterChoice or Devour
// ask the entry at sl queued (asks n0 onwards) and reports whether its enters
// triggers wait for the answer (deferredETBFire).
func (s *State) deferETBToEntryCounters(k *prep, sl Slot, n0 uint8) bool {
	for i := n0; i < s.g.nask; i++ {
		a := &s.g.ask[i]
		if a.sl != sl || a.kind != akCtrPick && a.kind != akDevour || a.card == NoCard || a.pc == 0 {
			continue
		}
		if a.kind == akDevour || k.t.Cards[a.card].Abilities[uint8(a.ab)].Prog[a.pc-1].Ext() == kop.ExtPutCounterChoice {
			a.ts |= askETBDeferred
			return true
		}
	}
	return false
}

// deferredETBFire fires the enters triggers an answered as-enters counter
// pick or Devour held back, once the rest of the entering program has run; a later ask
// of the same program (asks n0 onwards) carries them on instead.
func (s *State) deferredETBFire(k *prep, ask askRec, n0 uint8) {
	for i := n0; i < s.g.nask; i++ {
		if s.g.ask[i].sl == ask.sl {
			s.g.ask[i].ts |= askETBDeferred
			return
		}
	}
	o := s.perms[ask.sl].obj
	if s.objs[o].zone == kop.ZoneBattlefield && (s.anyETB(k) || k.anyOffGrant) {
		s.fireObj(k, kop.TrigETB, ask.sl, o, s.perms[ask.sl].ctrl, 0)
	}
}

func (s *State) pccEnterAnswer(k *prep, ask askRec, a Action) {
	p := &s.perms[ask.sl]
	if s.objs[p.obj].zone == kop.ZoneBattlefield {
		s.placeCounters(k, ask.sl, int(a.Mode()), 1, p.ctrl, ctrPutEffect)
	}
}

// FeatCtrChoiceRank is the printed position of counter kind in the pending
// ExtPutCounterChoice ask, or -1 when the pending decision is no such ask (or
// kind is not offered). The decision's options are sorted canonically (kernel
// kind order), but gorge's bot answers its first offer in the printed list
// order, so a kernel bot that matches it reads the order here.
func (s *State) FeatCtrChoiceRank(kind uint8) int {
	var in *kop.Instr
	switch {
	case s.ctl == ctlResolve && s.res.ask == raRemCtrAny:
		in = &s.prog(s.k())[s.res.pc]
	case s.ctl == ctlGenAsk && s.g.nask > 0 && s.g.ask[0].kind == akCtrPick:
		a := &s.g.ask[0]
		in = &s.k().t.Cards[a.card].Abilities[uint8(a.ab)].Prog[a.pc-1]
	default:
		return -1
	}
	if in.Op != kop.OpExt || in.Ext() != kop.ExtPutCounterChoice {
		return -1
	}
	ks, n := in.PutCtrChoiceKinds()
	for i, k := range ks[:n] {
		if k == kind {
			return i
		}
	}
	return -1
}
