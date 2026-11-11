package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the hidden pick over Origin$ Hand,Graveyard. gorge
// effChangeZone sends a mixed origin that includes Hand, with no concrete
// object selector, to effSearchLibrary (engine/effects/zone_change.go:
// 456-473); its pool is, for the fetch player (the controller), each named
// zone of THAT player in Origin$ order, matched against ChangeType$
// (zone_search.go:136-149). One decision; Min is 0 (the ChangeType$ states
// a quality, zone_search.go:232-236); an empty pool asks nothing (:239-245).
// A picked card moves through settleChangeZoneMoveAs with WithCountersType$
// folded into the entry (:1331-1335) and Tapped$ after the move (:1359-1361).
// No library was searched, so nothing is shuffled (:1186).

// DebugHandGraveNoHand is the mutation switch:
// handGraveCands drops the hand half of the pool. TestHandgraveMutation
// requires the lockstep to diverge.
var DebugHandGraveNoHand bool

// handGraveOp reports whether the search op in progress is a Hand,Graveyard
// pick (the flag rides the instruction's B, never pickF).
func (s *State) handGraveOp(k *prep) bool {
	in := &s.prog(k)[s.res.pc]
	return in.Op == kop.OpChooseMove && in.B.N&kop.SrHandGrave != 0
}

// handGraveCands adds the fetching player's hand (hand order), then their
// graveyard (graveyard order), each card matched against the filter.
//
//go:noinline
func (s *State) handGraveCands(k *prep, in *kop.Instr, d *Decision) {
	p := s.g.pickP
	pl := &s.pl[p]
	ctrl := s.res.e.ctrl
	for zi := 0; zi < 2; zi++ {
		var list []Obj
		if zi == 0 {
			if DebugHandGraveNoHand {
				continue
			}
			list = pl.hand[:pl.handN]
		} else {
			list = pl.grave[:pl.graveN]
		}
		for _, o := range list {
			if s.picksHas(o) {
				continue
			}
			if !s.tokenLeftBattlefield(o) && s.matchCardGen(k, in.F, s.objs[o].card, p, ctrl, o) { // a token off the battlefield is no card (CR 111.8)
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	}
}

// handGraveArm arms the entry counter (WithCountersType$, amount 1; B bits
// 4..11 hold the kind+1) a Hand,Graveyard pick's entrant consumes
// and a plain Graveyard pick's
// (Scavenger's Talent: the kind rides the same bits without SrHandGrave; the
// pick's filter zone names the graveyard).
//
//go:noinline
func (s *State) handGraveArm(k *prep) {
	pr := s.prog(k)
	if !s.res.active || int(s.res.pc) >= len(pr) {
		return // moveFound driven outside a program (kernel tests)
	}
	in := &pr[s.res.pc]
	if in.Op != kop.OpChooseMove {
		return
	}
	if in.B.N&kop.SrHandGrave == 0 && (DebugGyBfNoCounter || s.searchZone(k) != kop.ZoneGraveyard) {
		return // not a Hand,Graveyard pick nor a plain graveyard pick
	}
	if kd := int(in.B.N>>4) & 0xFF; kd != 0 {
		s.g.enterCtr.n, s.g.enterCtr.kind = 1, kop.CounterKind(kd-1)
	}
}

// handGraveCounter reports a graveyard pick whose entrants get a counter of
// kind ck (B bits 4..11 hold kind+1): a Hand,Graveyard pick (SrHandGrave) or
// a plain Graveyard pick (fz is the filter's zone); the prep scans read it.
func handGraveCounter(in *kop.Instr, fz kop.Zone, ck kop.CounterKind) bool {
	if in.Op != kop.OpChooseMove || in.B.K != kop.VConst || int(in.B.N>>4)&0xFF != int(ck)+1 {
		return false
	}
	return in.B.N&kop.SrHandGrave != 0 || fz == kop.ZoneGraveyard
}
