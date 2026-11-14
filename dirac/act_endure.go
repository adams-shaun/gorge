package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Endure N (CR 701.63; gorge effects/endure.go effEndure) as
// OpExt ExtEndure. Cold and out of line: only an ExtEndure instruction
// reaches it. N is override C's PA, read once before the ask (gorge reads
// Num once); N <= 0 does nothing and asks nothing (CR 701.63b). The object
// in Obj still on the battlefield: its controller chooses (ChGenEndure,
// option 0 N +1/+1 counters on it, option 1 an N/N white Spirit). Gone: the
// resolving controller gets the Spirit, no ask. The Spirit is card B.N,
// set to N/N by override C for as long as it stays (gorge's permanent
// layer-7b set per mint).

// raEndure is the endure ask (res.ask).
const raEndure uint8 = 248

// ChGenEndure is the endure choice: mode 0 counters, mode 1 the Spirit,
// Source the enduring permanent's object.
const ChGenEndure ChoiceKind = 248

// endureN is the instruction's N: override C's value P/T.
func (s *State) endureN(k *prep, in *kop.Instr) int {
	return s.value(k, k.t.Overrides[in.C].PA)
}

// endureStart runs ExtEndure up to its ask; false means the ask is posed.
//
//go:noinline
func (s *State) endureStart(k *prep, in *kop.Instr) bool {
	n := s.endureN(k, in)
	if n <= 0 {
		return true // CR 701.63b
	}
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		s.endureSpirit(k, in, s.res.e.ctrl, n)
		return true
	}
	s.res.ask, s.res.askP = raEndure, s.perms[sl].ctrl
	return false
}

// askOptionsEndure builds the endure ask (the tail of askOptionsColGrant).
//
//go:noinline
func (s *State) askOptionsEndure(k *prep, d *Decision) bool {
	if s.res.ask != raEndure {
		return s.askOptionsHide(k, d) // kw_hideaway.go
	}
	o := s.objRefObj(s.prog(k)[s.res.pc].Obj)
	d.Sub = ChGenEndure
	d.add(MakeAction(AkChoose, TCode(o), 0, 0))
	d.add(MakeAction(AkChoose, TCode(o), 0, 1))
	return true
}

// applyAskEndure applies the endure answer and resumes the resolution.
//
//go:noinline
func (s *State) applyAskEndure(k *prep, a Action) bool {
	if s.res.ask != raEndure {
		return s.applyAskHide(k, a) // kw_hideaway.go
	}
	in := &s.prog(k)[s.res.pc]
	s.res.ask = raNone
	spirit := a.Mode() == 1
	n := s.endureN(k, in)
	sl := s.objRef(k, in.Obj)
	if spirit {
		s.endureSpirit(k, in, s.res.askP, n)
	} else if sl != NoSlot {
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
	s.continueRes(k, 0)
	return true
}

// endureSpirit mints the N/N Spirit for p (gorge endureCreateSpirit: one
// TokenCreate through the CreateToken replacements, the set on every mint).
func (s *State) endureSpirit(k *prep, in *kop.Instr, p uint8, n int) {
	tc, m, ok := s.replTokensCard(k, p, CardID(in.B.N), 1) // a rewriter's card (repl_tokrewrite.go)
	if !ok {
		return
	}
	if m != 1 {
		s.capFull(StallEndureN) // a token doubler: no pool card has one
		return
	}
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return
	}
	o := s.newToken(tc, p)
	sl := s.enter(k, o, p, false, kop.ZoneLibrary)
	if k.gen.anyTokCr {
		s.fireTokenCreated(k, sl)
	}
	s.res.lastTok = o
	s.applyOverridePT(k, sl, int(in.C), s.res.e.card, 0, int16(n), int16(n))
}
