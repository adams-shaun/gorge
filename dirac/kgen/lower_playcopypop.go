package kgen

import "strings"

// playCopyPopOK reads the population of a CopyCard$ True Play (CR 707.12: the
// cast is of a minted copy; gorge effects/play.go, beginPlay). Two shapes are
// proven in lockstep:
//
//   - Valid$ Card.IsRemembered (or the exile-granted Card.Self), ValidZone$
//     Exile, ValidSA$ Spell: the remembered cards in exile;
//   - Defined$ Remembered with the Play chained straight behind the Exile that
//     remembered its card (Narset, Enlightened Exile; Roving Actuator): the
//     population is the one remembered card, still in exile, the same cards
//     the first shape reads, and the cast is free.
//
// Every other population names what it needs in the refusal: a Defined$
// ExiledWith or ExiledWithSource population, an imprinted card, a chosen or
// delay-trigger-remembered card, a paid copy, a targeted card.
func (lc *lowerCtx) playCopyPopOK(ps *params) error {
	if _, ok := ps.m["ValidSA"]; !ok && ps.m["Defined"] == "Remembered" && lc.playRememberedNonland() {
		// an absent ValidSA$ over a population no land can match reads as
		// Spell (playDefaultValidSA's rule; Narset's noncreature, nonland
		// target)
		ps.m["ValidSA"] = "Spell"
	}
	if ps.m["ValidSA"] != "Spell" || ps.m["ValidTgts"] != "" || ps.m["ForgetPlayed"] != "" || ps.m["ReplaceGraveyard"] != "" {
		return refuse("Play CopyCard outside Valid$ IsRemembered, ValidZone$ Exile, ValidSA$ Spell")
	}
	if def := ps.m["Defined"]; def != "" {
		if def != "Remembered" || ps.m["Valid"] != "" || ps.m["ValidZone"] != "" {
			return refuse("Play CopyCard Defined$ %q population (the kernel reads remembered cards in exile only)", def)
		}
		if !ps.is("WithoutManaCost", "True") {
			return refuse("Play CopyCard Defined$ Remembered paid copy")
		}
		if !lc.playRememberedExiled() {
			return refuse("Play CopyCard Defined$ Remembered not behind the Exile that remembered it")
		}
		return nil
	}
	if ps.m["ValidZone"] != "Exile" {
		return refuse("Play CopyCard outside Valid$ IsRemembered, ValidZone$ Exile, ValidSA$ Spell")
	}
	valid := ps.m["Valid"]
	if strings.Contains(valid, "ExiledWithSource") && !strings.Contains(valid, "IsRemembered") {
		return refuse("Play CopyCard Valid$ ExiledWithSource population (the copy drain reads remembered cards)")
	}
	if !(strings.Contains(valid, "IsRemembered") || lc.grantExile && valid == "Card.Self") {
		return refuse("Play CopyCard outside Valid$ IsRemembered, ValidZone$ Exile, ValidSA$ Spell")
	}
	return nil
}

// playRememberedExiled reports that the chain's last lowered op is the Exile
// that remembered its card, so Defined$ Remembered names a card in exile.
func (lc *lowerCtx) playRememberedExiled() bool {
	n := len(lc.ab.Prog)
	if n == 0 {
		return false
	}
	last := &lc.ab.Prog[n-1]
	return last.Op == "Exile" && strings.Contains(last.CS, "Remember")
}

// playRememberedNonland reports that the Exile ending the chain took a card of
// a target slot whose every filter clause excludes lands or names a nonland
// card type, so the remembered card is never a land.
func (lc *lowerCtx) playRememberedNonland() bool {
	if !lc.playRememberedExiled() {
		return false
	}
	ref := lc.ab.Prog[len(lc.ab.Prog)-1].Obj
	if ref.Kind != rTarget || ref.N >= len(lc.ab.Targets) || lc.ab.Targets[ref.N].F == nil {
		return false
	}
	f := lc.ab.Targets[ref.N].F
	if len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.NotTypes&tLand == 0 && (c.Types == 0 || c.Types&tLand != 0) {
			return false
		}
	}
	return true
}
