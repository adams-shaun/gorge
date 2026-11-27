package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Whenever you get one or more {E}" (TrigPlayerCounterAdded; CR 122.1,
// 603.2c): Mode$ CounterAddedOnce with ValidPlayer$ You, the player-counter
// reading of the batch trigger (Brotherhood Scribe, Fabrication Module,
// Territorial Gorger, Aether Revolt).
//
// gorge publishes one events.PlayerCounterChange per player and counter kind
// with the batch in Amount, and trigmatch playerCounterAddedOnceMatches
// queues one trigger for it (ValidPlayer$ the gaining player, CounterType$
// the kind, Amount > 0). The batch size rides TriggerCount$Amount (Aether
// Revolt's "that much damage"). The kernel's player counters (energy,
// experience) rise at three sites, each firing here once the counter has
// moved: gainEnergy (OpGainEnergy), doublePlayerCounters and proliferate's
// player arm. Nothing subscribes a slot: the battlefield is scanned in
// listener order, as for TrigScry; there is no prep gate (a genPrep bool would
// be a new fixed State field), the scan runs only when a player counter rises.

// firePlayerCtr fires "whenever <Whose> gets one or more counters of player
// counter kind kind" for player p, who gained n of them (n > 0).
func (s *State) firePlayerCtr(k *prep, p uint8, kind, n int) {
	if k.gen == nil || n <= 0 || DebugPlayerCtrOff {
		return
	}
	var ord [MaxPerm]Slot
	cnt := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:cnt] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool {
			return a.Trig.Event == kop.TrigPlayerCounterAdded && int(a.Trig.N) == kind
		}, func(l trigListener, a *kop.Ability) {
			np := s.npend
			s.playerTrigGen(k, kop.TrigPlayerCounterAdded, l, a, p, int16(min(n, 255)))
			if s.npend > np && DebugMetalcraftStats != nil {
				DebugMetalcraftStats.EnergyTr++
			}
		})
	}
}
