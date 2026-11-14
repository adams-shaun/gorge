package kgen

import "strings"

// protection lowers Protection Gains$ Choice, Choices$ AnyColor (Mother of
// Runes, Gods Willing, Aven Liberator, Benevolent Bodyguard, Emerge
// Unscathed, Shelter, Alseid of Life's Bounty): the target permanent gains
// protection from the colour its controller chooses as this resolves, until
// end of turn (gorge effProtection: protectionQualities asks the five
// colours, grantProtectionUntilEOT grants "Protection from <colour>"). The
// kernel already prices both halves for the chosen-colour sweeps (Akroma's
// Blessing): ChooseColorRes asks the colour, then ColorGrant C bit 2 marks
// the one target.
//
// Any other Gains$ (a named colour or card type: Tower of the Magistrate's
// artifacts), a Choices$ list beyond AnyColor (Giver of Runes' colorless:
// the kernel's colour ask has no sixth answer and no protection from
// colourless), a Defined$ target or a Duration$ refuses.
func (lc *lowerCtx) protection(ps *params) error {
	if g := ps.str("Gains"); !strings.EqualFold(g, "Choice") {
		return refuse("Protection Gains$ %q (only a chosen colour lowers)", g)
	}
	if c := strings.TrimSpace(ps.str("Choices")); !strings.EqualFold(c, "AnyColor") {
		return refuse("Protection Choices$ %q (only AnyColor lowers)", c)
	}
	for _, k := range [...]string{"Defined", "Duration", "Optional"} {
		if _, ok := ps.get(k); ok {
			return refuse("Protection %s$", k)
		}
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("Protection without its own target")
	}
	ps.consume("AILogic")
	lc.emit(lInstr{Op: "ChooseColorRes", Note: "the controller chooses the colour to gain protection from"})
	lc.emit(lInstr{Op: "ColorGrant", Obj: obj, A: cst(0), C: 4, Note: "until end of turn: the target has protection from the chosen colour"})
	return nil
}
