package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// TrigLandPlayed, TrigSearchedLibrary, TrigForage and
// TrigDiscover. None has a slot subscription; every fire is behind a prep
// bool (genPrep.anyLandPlayed / anyKwAct), so a table without them runs
// the old paths.
//
// gorge: landPlayedMatches (trigmatch/actions.go) is the land's move to the
// battlefield while it is a land play (a Play effect's too, so hideaway's
// land counts); it is the same event as the land's ETB, so gorge's face
// walk queues ETB and LandPlayed lines interleaved, per object in walk
// order and per line in ability order (enterLandPlayed). The three player
// events fire once each: a hidden library search (also when nothing is
// found; trigmatch/cards.go), a paid forage after its moves, a discover at
// its completion with the discover value as the amount.

// DebugTrigModes2Off is the mutation hook (TestLandSearchForageDiscoverTriggersMutation): no
// LandPlayed / SearchedLibrary / Forage / Discover line fires.
var DebugTrigModes2Off bool

// fireKwAct fires "whenever <Whose> searches their library / forages /
// discovers" (ev is TrigSearchedLibrary, TrigForage or TrigDiscover) for
// player who; amt is the trigger amount (the discover value).
//
//go:noinline
func (s *State) fireKwAct(k *prep, ev kop.TrigEvent, who uint8, amt int16) {
	if k.gen == nil || !k.gen.anyKwAct || DebugTrigModes2Off {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == ev }, func(l trigListener, a *kop.Ability) {
			s.playerTrigGen(k, ev, l, a, who, amt)
		})
	}
}

// enterLandPlayed is a land play's entry: enter's ETB fire and the
// LandPlayed lines in one battlefield walk, gorge's interleaving. A
// LandPlayed line matches on the origin mask (Trigger.N, 0 = any) and its
// ValidCard$ filter against the land on the battlefield.
//
//go:noinline
func (s *State) enterLandPlayed(k *prep, o Obj, p uint8, from kop.Zone) {
	sl := s.enterNF(k, o, p, false, from, 0)
	etb := s.anyETB(k) && !(k.gen.anyDisETB && s.etbDisabled(k, sl))
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, ls := range ord[:n] {
		s.slotTriggers(k, ls, func(a *kop.Ability) bool {
			return etb && a.Trig.Event == kop.TrigETB || a.Trig.Event == kop.TrigLandPlayed && !DebugTrigModes2Off
		}, func(l trigListener, a *kop.Ability) {
			if a.Trig.Event == kop.TrigETB {
				s.objTrigGen(k, kop.TrigETB, l, a, sl, o, p, 0)
				return
			}
			if a.Trig.N != 0 && a.Trig.N&(1<<from) == 0 {
				return
			}
			if !s.objEventMatch(k, l, a, sl) {
				return
			}
			if s.trigOK(k, l, a) {
				s.queueTrig(k, l, a, o, p, 0)
			}
		})
	}
	if etb && k.gen.graveETB {
		s.fireGraveETB(k, sl, o, p)
	}
	if etb && k.gen.embObjEv&(1<<kop.TrigETB) != 0 && sl != NoSlot {
		s.fireEmblemsObj(k, kop.TrigETB, sl, o)
	}
}

// forageCastDone fires TrigForage once a cast or activation cost that
// includes a forage is paid, after every sacrifice (gorge cast_commit.go).
//
//go:noinline
func (s *State) forageCastDone(k *prep) {
	if k.costObj && k.gen.anyKwAct {
		if pc := s.buildPickCost(k); pc != nil && pc.Flags&kop.CostForage != 0 {
			s.fireKwAct(k, kop.TrigForage, s.bld().e.ctrl, 0)
		}
	}
}
