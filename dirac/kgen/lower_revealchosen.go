package kgen

// RevealChosen<Type/creature type> in an activated ability's own cost (A
// Killer Among Us: "Sacrifice A Killer Among Us, Reveal the chosen creature
// type: ..."). gorge pays it with nothing and asks nothing: the payment
// prints the public reveal Note (rules/pay EmitChoiceCosts, RevealChosenText)
// and the ONLY gate is that the source still carries the secretly chosen
// type (pay castparts.go: HasRevealChosenDesignation, Object.ChosenType
// non-empty), else the ability is not offered. The kernel keeps a
// permanent's chosen creature type in its chosen byte, 255 for none (the
// byte ExChosenMode reads off the evaluating source's slot, 255 also off the
// battlefield), so the part lowers as the activation condition "the
// source's chosen byte is set" and no cost bit.
// RevealChosen<Player> (Emissary of Grudges, Guardian Archon: a secret
// ChoosePlayer) stays refused.

// revealChosenCost reads one RevealChosen<Type/...> part onto c.
func (lc *lowerCtx) revealChosenCost(c *lCost, parts []string, t string) error {
	if !lc.actCost || lc.escCost || c.RevealChosenT {
		return refuse("reveal-chosen cost %q outside an activated ability's own cost", t)
	}
	if len(parts) < 1 || parts[0] != "Type" {
		return refuse("reveal-chosen cost %q: not a chosen creature type", t)
	}
	c.RevealChosenT = true
	return nil
}

// revealChosenGate is the activation condition a RevealChosen<Type/...>
// part adds: the source permanent's chosen byte holds a type.
func revealChosenGate(c lCost) lCond {
	if !c.RevealChosenT {
		return lCond{}
	}
	return lCond{Kind: cValue, A: lValue{Kind: vChosenMode}, Op: opNE, B: cst(255)}
}
