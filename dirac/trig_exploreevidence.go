package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "whenever a creature explores" (TrigExplores;
// CR 701.44a) and "whenever you collect evidence" (TrigCollectEvidence;
// CR 701.59a).
//
// gorge emits the events.Explore record once per explore, AFTER the outcome
// (engine/effects/explore.go): a land once it is in hand, a nonland after the
// counter and the graveyard-or-top answer (also when the card stays on top).
// An empty library, a departed explorer or a replaced explore emit nothing.
// trigmatch exploresMatches reads ValidCard$ against the explorer and
// ValidExplored$ against the revealed card wherever it now is.
//
// events.CollectEvidenceAction is emitted once per cost payment that exiled
// at least one card (cast_commit.go, cumulative.go, pay/evidence.go), after
// the exiles; the ward evidence payment (pay.PayEvidence) records none, so
// settleWardEvidence fires nothing. Neither event has a slot subscription:
// the battlefield is scanned, as for TrigScry.

// DebugExplEvAnyPlayer is a mutation hook (TestTrigexplevMutation): a
// CollectEvidence line fires for either player's evidence, and an Explores
// line ignores its Filter2 (the revealed card).
var DebugExplEvAnyPlayer bool

// fireExplores fires "whenever a creature explores" after the outcome of an
// explore by the permanent in.Obj names, controlled last by p, that revealed
// object rev (NoObj: an empty library revealed nothing).
//
//go:noinline
func (s *State) fireExplores(k *prep, in *kop.Instr, p uint8, rev Obj) {
	explorerSl := s.objRef(k, in.Obj)
	evObj, evCtrl := s.objRefObj(in.Obj), p
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigExplores }, func(l trigListener, a *kop.Ability) {
			if f2 := a.Trig.Filter2; f2 != 0 && !DebugExplEvAnyPlayer &&
				(rev == NoObj || !s.matchCardGen(k, f2, s.objs[rev].card, s.objs[rev].owner(), l.ctrl, NoObj)) {
				return
			}
			s.objTrigGen(k, kop.TrigExplores, l, a, explorerSl, evObj, evCtrl, 0)
		})
	}
}

// fireEvidence fires "whenever <Whose> collects evidence" for player p,
// after a cost payment exiled their picked cards.
//
//go:noinline
func (s *State) fireEvidence(k *prep, p uint8) {
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigCollectEvidence }, func(l trigListener, a *kop.Ability) {
			who := p
			if DebugExplEvAnyPlayer {
				who = l.ctrl
			}
			s.playerTrigGen(k, kop.TrigCollectEvidence, l, a, who, 0)
		})
	}
}
