package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Whenever you win / lose a coin flip" (TrigFlippedCoin; CR 705.2, 603.2).
//
// gorge (engine/effects/flipcoin.go effFlipCoin) emits one result Note per
// coin as it lands, before that coin's outcome branch runs; trigmatch
// flippedCoinMatches reads it: ValidPlayer$ against the flipper (Whose) and
// ValidResult$ Win / Lose against the side (Trigger.N: kop.TrigNFlipWin,
// kop.TrigNFlipLose, 0 either). An uncalled NoCall$ flip that no FlipCoinMod
// static fixed fires nothing (kop.RollDieCoinNoCall). The kernel fires from
// OpRollDie's coin flip at the same point. No slot subscription: the
// battlefield is scanned behind genPrep.anyFlipTrig.

// DebugFlippedCoinAnySide is the lockstep mutation switch (test only): a
// FlippedCoin line fires on either side of the flip.
var DebugFlippedCoinAnySide bool

// fireFlippedCoin fires the TrigFlippedCoin lines for player p's flip, won
// (heads) or lost.
//
//go:noinline
func (s *State) fireFlippedCoin(k *prep, p uint8, won bool) {
	if k.gen == nil || !k.gen.anyFlipTrig {
		return
	}
	side := kop.TrigNFlipLose
	if won {
		side = kop.TrigNFlipWin
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool {
			return a.Trig.Event == kop.TrigFlippedCoin && (a.Trig.N == 0 || a.Trig.N == side || DebugFlippedCoinAnySide)
		}, func(l trigListener, a *kop.Ability) {
			s.playerTrigGen(k, kop.TrigFlippedCoin, l, a, p, 0)
		})
	}
}
