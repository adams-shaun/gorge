package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// an Effect MayPlay with Duration$ AsLongAsControl
// ("you may play that card for as long as you control <this>": Lightning,
// Security Sergeant). gorge registers the grant as a continuous effect that
// continuousLive (rules/layers_effect.go) keeps only while the effect's
// source is on the battlefield under the effect's controller (since;
// cast_aslongascontrol.go drops the record for good on a control change). The
// source incarnation is not recorded for a Card.IsRemembered grant, and no
// sweep drops the registration when the source leaves (EndOfTurnCleanup keeps
// it), so a source that returns under the same controller, on this turn or a
// later one, makes the grant live again. The record (until = kop.DurWhileSrcMay)
// keeps the source as turn = source Obj | source ver<<8 and is live while that
// object is on the battlefield. The card leaving exile ends its own
// permission through the record's ver. 0 B: the source rides turn.

// DebugMayWhileSrcForever makes the record ignore its source (mutation
// testing only: the lockstep must catch it).
var DebugMayWhileSrcForever bool

// prepMayWhileSrc sets anyMayWhileSrc: some OpMayPlay lasts DurWhileSrcMay.
func prepMayWhileSrc(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				g.anyMayWhileSrc = g.anyMayWhileSrc || in.Op == kop.OpMayPlay && (in.A.N == kop.DurWhileSrcMay || in.A.N == kop.DurWhileValid)
			}
		}
	}
}

// mayWhileSrcStamp names src as the source of every DurWhileSrcMay record
// from index from on (the records one OpMayPlay resolution just appended).
// Callers gate on genPrep.anyMayWhileSrc.
//
//go:noinline
func (s *State) mayWhileSrcStamp(from uint8, src Obj) {
	for i := int(from); i < int(s.g.nmay); i++ {
		if r := s.mayAt(i); r.dur() == kop.DurWhileSrcMay {
			r.turn = uint16(src) | uint16(s.objs[src].ver)<<8
		}
	}
}

// maySrcLive reports whether DurWhileSrcMay record r still has its source:
// on the battlefield (a token must also still be the recorded object) and
// under the record's player (CR 611.2b).
//
//go:noinline
func (s *State) maySrcLive(r mayPlay) bool {
	if DebugMayWhileSrcForever {
		return true
	}
	src := &s.objs[uint8(r.turn)]
	return src.zone == kop.ZoneBattlefield && (src.of&ofToken == 0 || src.ver == uint8(r.turn>>8)) &&
		s.maySrcCtrlOK(r, src) // cast_aslongascontrol.go
}

// maySrcDead reports whether r is a DurWhileSrcMay record whose source is
// gone: a play surface skips it.
//
//go:noinline
func (s *State) maySrcDead(r mayPlay) bool {
	return r.dur() == kop.DurWhileSrcMay && !s.maySrcLive(r) || r.dur() == kop.DurWhileValid && !s.mayValidHolds(r)
}

// mayValidHolds reports whether a DurWhileValid record's controller controls a
// permanent matching its filter.
//
//go:noinline
func (s *State) mayValidHolds(r mayPlay) bool {
	k := s.k()
	for m := s.ctrlM[r.ctrl].and(s.occ); m.any(); m = m.rest() {
		if s.matchPermGen(k, kop.FilterID(r.turn), m.first(), r.ctrl, NoSlot, layerFull) {
			return true
		}
	}
	return false
}

// mayWhileSrcKeep reports whether cleanup keeps DurWhileSrcMay record r.
// gorge's EndOfTurnCleanup keeps the grant (neither UntilEOT, UntilTurn nor
// Permanent) and continuousLive re-reads its source on every look, so a
// source that left and returns on a later turn revives it: the record stays
// while its card is still the exiled object (ForgetOnMoved$ Exile). A token
// source that is gone can never return, so its record is dropped.
//
//go:noinline
func (s *State) mayWhileSrcKeep(r mayPlay) bool {
	src := &s.objs[uint8(r.turn)]
	return s.plotLive(r) && (src.of&ofToken == 0 || src.ver == uint8(r.turn>>8))
}
