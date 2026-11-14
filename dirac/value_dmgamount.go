package dirac

// The positive damage a reader card's object received this turn, as an amount
// (gorge Object.DamageReceivedThisTurn; Count$TotalDamageReceivedThisTurn).
// genState.dmgAmt holds four records, o = Obj+1 (0 empty) and n the amount,
// saturating at 255. Only a card whose CardDef.DmgAmt is set has its damage
// recorded, so a pool with no reader never opens the sites; the record follows
// the object across zone changes, as gorge's does (only WasDealtDamageThisTurn
// clears on a battlefield entry), and resets at the turn change.

// StallDmgAmt is a fifth reader-card object damaged in one turn (genState.dmgAmt).
const StallDmgAmt = 162

func init() { stallNames[StallDmgAmt] = "dmgamt" }

// recordDmgAmt adds n positive damage to the record of the reader card at sl.
// Sites call it beside dmgTurn.set; n is capped by the record's byte width.
func (s *State) recordDmgAmt(k *prep, sl Slot, n int) {
	if n <= 0 || k.gen == nil || !k.gen.anyDmgAmt || !k.t.Cards[s.cardAt(sl)].DmgAmt {
		return
	}
	o := s.perms[sl].obj + 1
	for i := range s.g.dmgAmt {
		if s.g.dmgAmt[i].o == o {
			s.g.dmgAmt[i].n = uint8(min(int(s.g.dmgAmt[i].n)+n, 255))
			return
		}
	}
	for i := range s.g.dmgAmt {
		if s.g.dmgAmt[i].o == 0 {
			s.g.dmgAmt[i].o, s.g.dmgAmt[i].n = o, uint8(min(n, 255))
			return
		}
	}
	s.capFull(StallDmgAmt)
}

// dmgAmountReceived is the positive damage object o received this turn, 0 when
// it holds no record (never a reader card, or a new turn).
func (s *State) dmgAmountReceived(o Obj) int {
	o1 := o + 1
	for i := range s.g.dmgAmt {
		if s.g.dmgAmt[i].o == o1 {
			return int(s.g.dmgAmt[i].n)
		}
	}
	return 0
}
