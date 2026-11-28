package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext EnterCtrPick (kop.ExtEnterCtrPick): the as-enters choice of two
// different counter kinds from three (Grimdancer; gorge effPutCounter's
// ChooseDifferent$ arm, a Min=Max=2 "counter_kinds" KChoose). The enters
// program queues the first ask (akCtrPick, runReplProgAs); its answer is kept
// in the permanent's chosen byte and the second ask queued (akCtrPick2, the
// two kinds left); the second answer puts one counter of each picked kind
// and the rest of the program runs. Both asks are a ChRemCtrKind decision
// (mode = the counter kind), canonical as gorge's "counter" options.
const (
	akCtrPick  uint8 = 31 // the first of the two kinds
	akCtrPick2 uint8 = 32 // the second, the first pick kept in the permanent's chosen byte
)

// ctrPickKinds is the three kinds of the instruction, in printed order.
func ctrPickKinds(in *kop.Instr) [3]uint8 {
	return [3]uint8{uint8(in.B.N), uint8(in.C & 0xff), uint8(in.C >> 8)}
}

// ctrPickOptions fills the queued ask: every kind for the first pick, the two
// not yet picked for the second.
func (s *State) ctrPickOptions(k *prep, d *Decision, kind uint8) {
	d.Sub = ChRemCtrKind
	if s.g.nask == 0 {
		return
	}
	a := &s.g.ask[0]
	in := &k.t.Cards[a.card].Abilities[uint8(a.ab)].Prog[a.pc-1]
	if in.Ext() == kop.ExtPutCounterChoice {
		pccOptions(d, in) // ctr_choice.go
		return
	}
	for _, c := range ctrPickKinds(in) {
		if kind == akCtrPick2 && c == uint8(s.perms[a.sl].chosen) {
			continue
		}
		d.add(MakeAction(AkChoose, 0, 0, c))
	}
}

// ctrPickAnswer applies an answer of the ask ask: the first pick is kept and
// the second asked; the second puts the counters. true: the program goes on.
func (s *State) ctrPickAnswer(k *prep, ask askRec, a Action) bool {
	p := &s.perms[ask.sl]
	if ask.kind == akCtrPick && k.t.Cards[ask.card].Abilities[uint8(ask.ab)].Prog[ask.pc-1].Ext() == kop.ExtPutCounterChoice {
		s.pccEnterAnswer(k, ask, a) // ctr_choice.go
		return true
	}
	if ask.kind == akCtrPick {
		p.chosen = objWord(a.Mode())
		// The second pick goes back to the queue's head: gorge settles this
		// entry's two picks before the next entry's queued ask, as the Devour
		// and reveal picks do.
		ask.kind = akCtrPick2
		s.requeueAskHead(ask)
		return false
	}
	first, second := uint8(p.chosen), a.Mode()
	p.chosen = objWord(255)
	if s.objs[p.obj].zone == kop.ZoneBattlefield {
		in := &k.t.Cards[ask.card].Abilities[uint8(ask.ab)].Prog[ask.pc-1]
		for _, c := range ctrPickKinds(in) {
			if c == first || c == second {
				s.placeCounters(k, ask.sl, int(c), 1, p.ctrl, ctrPutEffect)
			}
		}
	}
	return true
}
