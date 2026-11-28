package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The secret ChooseNumber election (kop.ExtChooseNumberElection; gorge
// effects.effChooseNumberElection, choose.go; Expert-Level Safe): the
// resolving controller and the targeted opponent (Obj) each choose a number
// from ChooseNumberMid's bounds (chooseNumberMidBounds: C&0xff the floor, B
// the Max$ value), asked in gorge's repeatPlayers order (AliveFrom(controller):
// the controller first, then the opponent), one ChGenNumber ask each
// (raNumberElection, mode = the number).
//
// Secrecy: gorge posts no event while the picks are asked, and its second
// decision offers only the number list. The kernel keeps the first pick in
// res.left (pick + 1; 0 none yet), which no option list, view or ExChosen*
// read sees, and clears it with the verdict. On the last answer the
// controller's pick is stored as the source's chosen number (gorge's Choose
// event on the source), and the verdict is left in the result slot (g.roll,
// ExRollResult): 1 every pick matched, 2 not. kgen lowers MatchedAbility$ and
// UnmatchedAbility$ as blocks guarded by it (lower_choosenumber_election.go).
//
// A target that is no longer a living opponent leaves the controller the only
// chooser (gorge's chooser set holds the controller alone), a one-pick
// election that matches.

// electionOpp is the election's second chooser: the targeted living
// opponent, noSeat when there is none.
func (s *State) electionOpp(k *prep, in *kop.Instr) uint8 {
	p := s.playerRef(k, in.Obj)
	if p == 255 || p == s.res.e.ctrl || p >= s.nSeats() || s.out(p) {
		return noSeat
	}
	return p
}

// numberElectionExt is OpExt ExtChooseNumberElection; false: the controller's
// ask is posed.
//
//kmanifest:ext ExtChooseNumberElection
func (s *State) numberElectionExt(k *prep, in *kop.Instr) (ok, done bool) {
	if _, _, fits := s.chooseNumberMidBounds(k, in); !fits {
		s.capFull(StallChooseNumber) // a declared capacity limit, never a truncated ask
		return true, true
	}
	s.res.left = 0
	s.res.ask, s.res.askP = raNumberElection, s.res.e.ctrl
	return false, true
}

// numberElectionOptions poses the number list, the same for both choosers.
//
//kmanifest:askopts raNumberElection
func (s *State) numberElectionOptions(k *prep, d *Decision) {
	d.Sub = ChGenNumber
	lo, hi, ok := s.chooseNumberMidBounds(k, &s.prog(k)[s.res.pc])
	if !ok {
		s.capFull(StallChooseNumber)
		return
	}
	for n := lo; n <= hi; n++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(n)))
	}
}

// numberElectionAnswer holds the controller's pick and asks the opponent, or
// strikes the verdict on the last pick.
//
//kmanifest:askapply raNumberElection
func (s *State) numberElectionAnswer(k *prep, a Action) {
	r := &s.res
	in := &s.prog(k)[r.pc]
	if r.askP == r.e.ctrl {
		if opp := s.electionOpp(k, in); opp != noSeat {
			r.left, r.askP = a.Mode()+1, opp // hidden until the opponent answers
			return
		}
		r.left = a.Mode() + 1 // the controller alone: a one-pick election
	}
	first := r.left - 1
	r.left = 0
	o, sl := s.resolutionTypeSource()
	s.storeChosenNumber(o, sl, int(first))
	s.g.roll = 2
	if first == a.Mode() {
		s.g.roll = 1
	}
	s.continueRes(k, 0)
}
