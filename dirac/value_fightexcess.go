package dirac

// Fight ExcessSVar$ and the Effect UnspentMana Red. Cold:
// the hot OpFight / OpAddMana arms each carry one guard and call here.

// DebugFightExcessZero is a mutation switch: a tracked Fight records 0 excess.
var DebugFightExcessZero bool

// DebugUnspentRedOff is a mutation switch: unspentRedOn marks nothing.
var DebugUnspentRedOff bool

// pmUnspentR << p in State.pmOn: player p keeps unspent red until the turn
// change (the Effect UnspentMana Red of The Last Agni Kai). pmOn uses only
// bits 0 to MaxSeats-1 for the persistent-mana flags, so this is 0 B.
const pmUnspentR = 1 << MaxSeats

// unspentRedOn marks p's red as kept through every step boundary this turn
// (persistKeep keeps the whole red slot; persistTurn clears pmOn).
func (s *State) unspentRedOn(p uint8) {
	if DebugUnspentRedOff {
		return
	}
	s.pmOn |= pmUnspentR << SeatIx(p)
}

// fightExcessRec records the excess of the hit that dealt pa (the damage
// dealt, after any replacement or prevention) to the creature
// whose lethal (toughness less marked damage, clamped at 0) the caller read
// before any hit landed (gorge effFight excessLethal): pa - lethal, clamped
// at 0.
//
//go:noinline
func (s *State) fightExcessRec(lethal, pa int) {
	if DebugFightExcessZero {
		s.g.excess = 0
		return
	}
	s.g.excess = int16(max(pa-max(lethal, 0), 0))
}
