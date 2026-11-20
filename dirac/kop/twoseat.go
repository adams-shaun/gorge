package kop

// TwoSeatInstr reports whether in carries an operand that names the
// controller's one opponent, a reading the kernel has not generalised past
// two seats. ChooseOpp is not one: the kernel's chooser is the source's
// controller at any seat count. Nor is RetUnderOpponent: the return enters
// under the resolution's chosen opponent (dirac retUnderOppSeat), and kgen
// marks the one-opponent ChoosePlayer that lowers it otherwise two-seat.
// ExtExileLandChoose is seat-exact: it runs inside an OpRepeatEachPlayer over
// the living opponents and reads the loop's own player (act_exilelandchoose).
func TwoSeatInstr(in *Instr) bool {
	switch in.Op {
	case OpMayPlay:
		return in.B.N&MayPlayOpponent != 0 // the exiler: the controller's opponent
	}
	return false
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
// skip-untap marks (OpSkipUntap), a battle's one protector (TBattle), a
// granted static goad's goader (StatGoadGranted). The kernel's other two-seat-only reading, a
// counter-added-this-turn condition on the card itself (ctr_addedthisturn.go),
// needs the table's Exprs and stays a StallTwoSeat.
func CardPodLimited(c *CardDef) bool {
	if CardTwoSeat(c) || c.Engines || c.Types&TBattle != 0 || StatGoadGranted(c) {
		return true
	}
	for i := range c.Abilities {
		for j := range c.Abilities[i].Prog {
			if op := c.Abilities[i].Prog[j].Op; op == OpChangeSpeed || op == OpSkipUntap {
				return true
			}
		}
	}
	return false
}

// StatGoadGranted reports whether c carries a Goad$ True static that a grant
// switches on (Static.OvGate): the kernel reads no goader seat for it, so a
// pod holding c stops at its start. A printed static goad's goader is its
// controller and plays at any seat count.
func StatGoadGranted(c *CardDef) bool {
	for i := range c.Abilities {
		if a := &c.Abilities[i]; a.Kind == AbStatic && a.Static.Kind == StatGoad && a.Static.OvGate != 0 {
			return true
		}
	}
	return false
}
