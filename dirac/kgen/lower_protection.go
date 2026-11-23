package kgen

import (
	"strings"
)

// protection lowers the Protection effect (gorge effProtection: the target or
// Defined$ permanent gains "protection from <quality>" until end of turn,
// grantProtectionUntilEOT).
//
// Gains$ Choice, Choices$ AnyColor (Mother of Runes, Gods Willing, Aven
// Liberator, Benevolent Bodyguard, Emerge Unscathed, Shelter, Alseid of
// Life's Bounty): the controller chooses the colour as this resolves. The
// kernel already prices both halves for the chosen-colour sweeps (Akroma's
// Blessing): ChooseColorRes asks the colour, then ColorGrant C bit 2 marks
// the one target.
//
// A fixed Gains$ (Crimson Acolyte's red, Tower of the Magistrate's Artifact,
// Grotesque Hybrid's green,white) asks nothing: it is the same until-end-of-turn
// override record a Pump's KW$ Protection from <quality> lowers to
// (splitProtFromKW, kop.Override.ProtFrom), one clause per comma-separated
// quality (CR 702.16g; gorge splits the list the same way). The target is
// either the ValidTgts$ target or a Defined$ permanent (Keeper of Kookus,
// Self).
//
// A Choices$ list beyond AnyColor (Giver of Runes' colorless), Gains$
// ChosenColor (Floating Shield: the sacrificed source's colour), a Gains$
// Defined$ reference (Opal Titan's Duration$ Permanent grant) and a Duration$
// refuse by name.
func (lc *lowerCtx) protection(ps *params) error {
	for _, k := range [...]string{"Duration", "Optional"} {
		if _, ok := ps.get(k); ok {
			return refuse("Protection %s$", k)
		}
	}
	gains := strings.TrimSpace(ps.str("Gains"))
	if !strings.EqualFold(gains, "Choice") {
		return lc.protectionFixed(ps, gains)
	}
	if c := strings.TrimSpace(ps.str("Choices")); !strings.EqualFold(c, "AnyColor") {
		return refuse("Protection Choices$ %q (only AnyColor lowers)", c)
	}
	if _, ok := ps.get("Defined"); ok {
		return refuse("Protection Gains$ Choice with Defined$")
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

// protectionFixed lowers a Protection effect whose Gains$ names its qualities
// (red; Artifact; green,white) to one end-of-turn override record carrying the
// quality filter.
func (lc *lowerCtx) protectionFixed(ps *params, gains string) error {
	var q *lFilter
	switch {
	case strings.EqualFold(gains, "ChosenColor"), strings.EqualFold(gains, "TargetedCardColor"), strings.HasPrefix(gains, "Defined "):
		return refuse("Protection Gains$ %q reads the source or a triggering card at resolution (not lowered)", gains)
	}
	for _, w := range strings.Split(gains, ",") {
		w = strings.TrimSpace(w)
		if w == "" || strings.ContainsAny(w, " .$") {
			return refuse("Protection Gains$ %q (a fixed colour or card type lowers)", gains)
		}
		if c := protColourLetters[strings.ToLower(w)]; c != "" {
			q = appendProtClause(q, &lFilter{Clauses: []lClause{{Colors: c}}})
			continue
		}
		f, err := lc.qualityFilter(w)
		if err != nil {
			return refuse("Protection Gains$ %q: %v", gains, err)
		}
		q = appendProtClause(q, f)
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	ps.consume("AILogic")
	lc.emitOverride(obj, &lOverride{ProtFrom: q}, "EOT")
	return nil
}
