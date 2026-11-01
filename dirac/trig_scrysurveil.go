package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "whenever you scry" / "whenever you surveil"
// (TrigScry, TrigSurveil; CR 701.22, 701.25, 603.2).
//
// gorge (engine/effects/cardflow.go effLookAndArrange) emits the
// events.Surveil marker per acting player BEFORE the look, so a surveil of
// an empty library still fires; the events.Scry record is emitted once the
// arrangement is answered (engine/rules/arrange.go handleArrange), or by
// the stand-in when no ask is posed (an empty library). A scry or surveil
// of 0 is none (CR 701.22b, 701.25c; effScry/effSurveil): no ask,
// no marker, no record, so nothing fires. trigmatch cards.go
// surveilMatches / scryMatches read ValidPlayer$ against that player.
// Neither event has a slot subscription: the battlefield is scanned, as
// for TrigProliferate.

// DebugScrySurveilAnyPlayer is a mutation hook (TestScrysurveilMutation):
// a scry/surveil line fires for either player's scry or surveil.
var DebugScrySurveilAnyPlayer bool

// DebugScryZeroFires is a mutation hook (TestScryZeroNoTrigger): a scry
// or surveil of 0 fires its trigger again.
var DebugScryZeroFires bool

// scrySurveilStart runs as OpScry / OpSurveil starts for player p over n
// cards (n already clamped to the library; amt is the instruction's own
// count): a count of 0 fires nothing (DebugScryZeroFires restores the old
// fire, the TestScryZeroNoTrigger mutation); a surveil fires its
// trigger now, before the look; a scry that poses no ask (n <= 0)
// completes here.
func (s *State) scrySurveilStart(k *prep, op kop.Op, p uint8, n, amt int) {
	if amt <= 0 && !DebugScryZeroFires {
		return
	}
	if op == kop.OpSurveil {
		s.noteScry(k, op, p) // value_scrytally.go
		s.fireScrySurveil(k, kop.TrigSurveil, p)
	} else if n <= 0 {
		s.noteScry(k, op, p)
		s.fireScrySurveil(k, kop.TrigScry, p)
	}
}

// fireScrySurveil fires "whenever <Whose> scries / surveils" (ev is
// TrigScry or TrigSurveil) for player p.
func (s *State) fireScrySurveil(k *prep, ev kop.TrigEvent, p uint8) {
	if !k.gen.anyScryTrig {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == ev }, func(l trigListener, a *kop.Ability) {
			who := p
			if DebugScrySurveilAnyPlayer {
				who = l.ctrl
			}
			s.playerTrigGen(k, ev, l, a, who, 0)
		})
	}
}
