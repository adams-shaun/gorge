package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// RepeatOptionalForEachPlayer$ True (gorge effRepeatEach, the Tempt cycle and
// Tempting Contract): each player the loop runs over is offered its own yes /
// no before its body runs (repeatEachElectionDecision: a KChoose of "yes" /
// "no", ResumeKind repeat_each_optional, Player the subject). A yes runs the
// body for that player; a no skips it and the loop goes on to the next.
//
// The lowering (kgen lower_repeatplayers.go) puts ExtOptInSeat first in the
// OpRepeatEachPlayer body. It asks the repeated player (s.g.eachPl: the
// loop's seat, never the resolving controller, whom ChGenConfirm asks) with
// ChGenOptInSeat; a no skips the body's other instructions (B.N of them), so
// the loop's own jump back (loopBackGen) moves on to the next player. The
// answers are canonical yes_seat / no_seat (canon.ChooseKinds): the same
// answer the generic yes / no kinds would name, kept apart so a named-seat
// election is never read as a controller's.

// DebugOptInNoAccum is the mutation switch: a StoreSVar write made inside an
// opt-in loop's body is dropped, so the accumulator the offer feeds (Tempt
// with Bunnies' X) stays 0 and the controller takes nothing for the
// opponents who accepted. Tests only.
var DebugOptInNoAccum bool

// optInSeatExt is OpExt ExtOptInSeat; false: the election is posed.
//
//kmanifest:ext ExtOptInSeat
func (s *State) optInSeatExt(k *prep, in *kop.Instr) (ok, done bool) {
	if s.g.repEnd == 0 {
		return true, true // outside a loop nothing is offered
	}
	s.res.ask, s.res.askP = raGenOptInSeat, s.g.eachPl
	return false, true
}

// optInSeatOptions poses gorge's options: yes, then no.
//
//kmanifest:askopts raGenOptInSeat
func (s *State) optInSeatOptions(k *prep, d *Decision) {
	d.Sub = ChGenOptInSeat
	d.add(MakeAction(AkChoose, 0, 0, 1))
	d.add(MakeAction(AkChoose, 0, 0, 0))
}

// optInSeatAnswer applies the election: a no skips the player's body.
//
//kmanifest:askapply raGenOptInSeat
func (s *State) optInSeatAnswer(k *prep, a Action) {
	skip := uint8(0)
	if a.Mode() == 0 {
		skip = uint8(s.value(k, s.prog(k)[s.res.pc].B))
	}
	s.continueRes(k, skip)
}
