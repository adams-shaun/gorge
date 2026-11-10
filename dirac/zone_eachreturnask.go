package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Return to the battlefield over several targets ("return up to two target
// land cards from your graveyard to the battlefield tapped") moves them one
// target at a time in gorge (effects/zone_change.go), and an entering
// permanent's as-enters election (a shock land's pay-life choice, CR 614.12,
// 614.1c) is asked inside its own move: the targets after it are still in
// their zone while it is answered, the spell still resolving (CR 608.2c).
// The kernel queued every target's election and asked them once the whole
// loop had run. execEachTarget now parks the loop in genState.each at the
// first queued as-enters ask (the raGenEnterAsk midResAsk suspend), and the
// raGenEnterAsk answer resumes it (eachReturnResume).

// returnToBattlefield reports whether in is a Return to the battlefield.
func returnToBattlefield(in *kop.Instr) bool {
	return in.Op == kop.OpReturn && kop.Zone(in.C) == kop.ZoneBattlefield ||
		in.Op == kop.OpReturnEx && kop.Zone(in.C&7) == kop.ZoneBattlefield
}

// eachReturnParks reports whether the Return just run for one target queued an
// as-enters ask (beyond the n0 queued before it) that the resolution now poses
// (midResAsk sets raGenEnterAsk); the loop must wait for its answer.
func (s *State) eachReturnParks(k *prep, in *kop.Instr, n0 uint8) bool {
	return returnToBattlefield(in) && s.g.nask > n0 && s.midResAsk(k)
}

// eachReturnParked reports whether the Return just run for one target parked
// its as-enters choice with the card still in its zone (cloneReturnPark's
// preReturn ask, raGenRetChoice); the loop waits for that answer before it
// moves the targets after it, so none of them is on the battlefield while
// gorge asks (CR 614.12, 707.2).
func (s *State) eachReturnParked(in *kop.Instr) bool {
	return returnToBattlefield(in) && s.res.ask == raGenRetChoice
}

// eachReturnResume continues a Return loop parked on an as-enters ask once
// the queue is answered; true when a later target parked it again.
func (s *State) eachReturnResume(k *prep) bool {
	if s.g.each.j == 0 {
		return false
	}
	in := &s.prog(k)[s.res.pc]
	if !returnToBattlefield(in) {
		return false
	}
	return s.eachResume(k, in)
}
