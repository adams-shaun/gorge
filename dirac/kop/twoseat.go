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
		return in.B.N&(TokAttacking|TokAttackAsk) == TokAttacking // attacking the controller's opponent (TokenAttacking$ True asks, at any seat count)
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

// CardPodLimited reports whether a game of three or more seats holding c stops
// at its start (the kernel's StallTwoSeat, dirac/stallclass.go prepTwoSeat):
// c plays only two seats (CardTwoSeat), or it reads state the four-seat build
// keeps for two seats only: speed (OpChangeSpeed, Start your engines), the
// skip-untap marks (OpSkipUntap), a battle's one protector (TBattle), a static
// goad's goader (StatGoad). The kernel's other two-seat-only reading, a
// counter-added-this-turn condition on the card itself (ctr_addedthisturn.go),
// needs the table's Exprs and stays a StallTwoSeat.
func CardPodLimited(c *CardDef) bool {
	if CardTwoSeat(c) || c.Engines || c.Types&TBattle != 0 {
		return true
	}
	for i := range c.Abilities {
		if a := &c.Abilities[i]; a.Kind == AbStatic && a.Static.Kind == StatGoad {
			return true
		}
		for j := range c.Abilities[i].Prog {
			if op := c.Abilities[i].Prog[j].Op; op == OpChangeSpeed || op == OpSkipUntap {
				return true
			}
		}
	}
	return false
}
