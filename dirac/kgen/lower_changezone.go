package kgen

// changeZone lowers api:ChangeZone (changeZoneMove). ForgetChanged$ True
// rides as a move flag through many helper lowerings; the
// kernel honors it on exactly four emitted ops, so any other op that carries
// it is refused by name here, whichever helper emitted it.
func (lc *lowerCtx) changeZone(ps *params) error {
	// NonBasicSpell$ True is gorge cleaveSA's marker (engine/rules/cleave.go):
	// the SECOND SP ability of a Cleave spell (CR 702.148a, every corpus
	// carrier PrecostDesc$ Cleave), the bracket-free text a cleaved cast
	// resolves. It has no reader on the body itself -- gorge's cast-level
	// walk reads it only to find that ability and substitute it for
	// f.SpellAbility() at the offer, the target ask and the resolution -- so
	// the body lowering consumes it. The cleave cast itself is the ability's
	// Cost$ (lower.go's spell-cost check refuses a Cost$ that differs from
	// the printed mana cost, which every cleave ability states), so the
	// card stays refused there until the cost family lowers.
	ps.consume("NonBasicSpell")
	if r := lc.retCtr; r != nil && r.node == lc.curNode {
		lc.retCtr = nil
		return lc.retCtrMove(ps, r) // lower_effectreturnctr.go
	}
	if ps.peek("Chooser") == "You" && ps.peek("ValidTgts") != "" && ps.peek("Origin") == "Graveyard" && ps.peek("Hidden") == "" {
		// a targeted graveyard move has no pick to answer: gorge reads
		// Chooser$ in the hidden-origin pick, the library search
		// (searchChooser, hiddenPickChooser) and the known-origin move's
		// Optional$ confirmation (changeZoneOptionalConfirm); "You" is the
		// controller there, the default, so it is inert here (Misinformation;
		// the Optional$+Chooser$ shape still refuses in changeZoneMove).
		ps.consume("Chooser")
	}
	atEOT, hasAtEOT, err := lc.changeZoneAtEOT(ps) // lower_ateot.go
	if err != nil {
		return err
	}
	start := len(lc.ab.Prog)
	if err := lc.changeZoneMove(ps); err != nil {
		return err
	}
	lc.monarchJailMark(ps, start) // lower_monarchjail.go
	if hasAtEOT {
		if err := lc.emitChangeZoneAtEOT(start, atEOT); err != nil {
			return err
		}
	}
	for i := start; i < len(lc.ab.Prog); i++ {
		in := &lc.ab.Prog[i]
		fl := flagsOf(in.CS)
		if _, ok := fl["ForgetChanged"]; !ok {
			continue
		}
		switch in.Op {
		case "Return", "Bounce", "Exile", "Search":
		default:
			return refuse("ForgetChanged$ on a %s", in.Op)
		}
		if _, ok := fl["ForgetOther"]; ok {
			return refuse("ForgetChanged$ with ForgetOtherRemembered$")
		}
		if _, ok := fl["AlsoSelf"]; ok {
			return refuse("ForgetChanged$ with ThisDefinedAndTgts$")
		}
	}
	return nil
}
