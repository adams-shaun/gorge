package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// the Drawn trigger that works from the graveyard (Sneaky
// Snacker, "when you draw your third card in a turn, return this card from
// your graveyard"), and the N-th draw gate it carries (gorge trigmatch
// drawnMatches: Number$ N against drawNumberThisTurn, the player's Draw
// events since the last TurnChange; the kernel's drawnTurn).

// nthDrawOK: a TfSecondDraw trigger's ordinal (Trigger.N, 0 = the second)
// is the player's draws this turn.
func (s *State) nthDrawOK(t *kop.Trigger, who uint8) bool {
	n := int(t.N)
	if n == 0 {
		n = 2
	}
	return int(s.pl[who].drawnTurn) == n
}

// fireGraveDrawn queues the graveyard cards' Drawn triggers after the
// battlefield's, player 0's graveyard first, each in graveyard order (the
// order fireCrime and firePhaseGen walk TriggerZones$ Graveyard in).
//
//go:noinline
func (s *State) fireGraveDrawn(k *prep, who uint8, amt int16) {
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < MaxSeats; pp++ {
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigDrawn {
						continue
					}
					s.playerTrigGen(k, kop.TrigDrawn, trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}, a, who, amt)
				}
			}
		}
	}
}
