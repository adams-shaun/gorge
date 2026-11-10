package kop

// TwoSeatInstr reports whether in carries an operand that names the
// controller's one opponent, a reading the kernel has not generalised past
// two seats. ChooseOpp is not one: the kernel's chooser is the source's
// controller at any seat count.
func TwoSeatInstr(in *Instr) bool {
	switch in.Op {
	case OpReturnEx:
		return in.B.N&RetUnderOpponent != 0 // under the chosen opponent's control
	case OpCopyAsTokenEx:
		return uint16(in.B.N)&CopyNonRemCtrl != 0 // for the remembered controller's opponent
	case OpCreateTokenEx:
		return in.B.N&TokAttacking != 0 // attacking the controller's opponent
	case OpMayPlay:
		return in.B.N&MayPlayOpponent != 0 // the exiler: the controller's opponent
	}
	return in.Ext() == ExtExileLandChoose // the one other player's lands
}

// CardTwoSeat reports whether c plays only in a two-seat game: kgen marked
// its lowering (TwoSeat), or a program carries a TwoSeatInstr.
func CardTwoSeat(c *CardDef) bool {
	if c.TwoSeat {
		return true
	}
	for i := range c.Abilities {
		for j := range c.Abilities[i].Prog {
			if TwoSeatInstr(&c.Abilities[i].Prog[j]) {
				return true
			}
		}
	}
	return false
}
