package kgen

import "strings"

// validSAHead lowers the SpellCast ValidSA$ heads gorge's ValidSAMatches
// (trigmatch/cast.go:731) answers through the cast spell itself
// Spell.ManaFromTreasure (the cast spent Treasure mana)
// and the comma list "Spell.X,...,Spell.IsTargeting Valid Y" (the spell
// matches X, or some target of it matches Y). It reports whether it took
// the head; (false, nil) leaves v to the older arms and their refusal.
func (lc *lowerCtx) validSAHead(v, mode string, tr *lTrigger) (bool, error) {
	if mode != "SpellCast" {
		return false, nil
	}
	if v == "Spell.ManaFromTreasure" {
		tr.ManaFromTreasure = true
		return true, nil
	}
	if !strings.Contains(v, ",") || !strings.Contains(v, ".IsTargeting ") {
		return false, nil
	}
	const tgtPrefix = "Spell.IsTargeting Valid "
	var rest []string
	inner, nTgt := "", 0
	for _, alt := range strings.Split(v, ",") {
		switch {
		case strings.HasPrefix(alt, tgtPrefix):
			nTgt++
			inner = strings.TrimPrefix(alt, tgtPrefix)
		case strings.HasPrefix(alt, "Spell.") && !strings.Contains(alt, " "):
			rest = append(rest, "Card."+strings.TrimPrefix(alt, "Spell."))
		default:
			return true, refuse("ValidSA %q", v)
		}
	}
	if nTgt != 1 || len(rest) == 0 || tr.F != nil || inner == "" || strings.ContainsAny(inner, "~<>,") {
		return true, refuse("ValidSA %q", v)
	}
	var err error
	if tr.F, err = lc.filter(strings.Join(rest, ",")); err != nil {
		return true, err
	}
	if tr.F2, err = lc.filter(inner); err != nil {
		return true, err
	}
	if tr.F2.Players != 0 || len(tr.F2.Clauses) == 0 {
		return true, refuse("ValidSA %q", v)
	}
	tr.OrTargets = true
	return true, nil
}
