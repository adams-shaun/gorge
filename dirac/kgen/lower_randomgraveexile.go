package kgen

import "strings"

// randomGraveExile lowers a Hidden$ True ChangeZone Origin$ Graveyard
// Destination$ Exile with AtRandom$ True over the controller's own graveyard
// (Tersa Lightshatter: "exile a card at random from your graveyard"). gorge
// effHiddenPick takes the engine's seeded pick (randomObjIDs: Rand(len) per
// card over the eligible pool in graveyard order, no draw when the pool is
// no larger than the count) instead of asking. A constant ChangeNum$ 1,
// DefinedPlayer$ You and Mandatory$ True only; RememberChanged$ remembers
// the moved card. Kernel OpChooseMove with SrRandom (act_randomgravepick.go).
func (lc *lowerCtx) randomGraveExile(ps *params, f *lFilter, n lValue, flags []string, counterKind string, override *lOverride) error {
	if !ps.is("Hidden", "True") {
		return refuse("random graveyard exile without Hidden$")
	}
	if counterKind != "" || override != nil {
		return refuse("random graveyard exile with counters/override")
	}
	if !n.isConst() || n.N != 1 {
		return refuse("random graveyard exile ChangeNum$ %v", n)
	}
	if dp := ps.str("DefinedPlayer"); dp != "You" {
		return refuse("random graveyard exile DefinedPlayer$ %q", dp)
	}
	if !ps.is("Mandatory", "True") {
		return refuse("random graveyard exile without Mandatory$ True")
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("random graveyard exile filter %s", f)
	}
	for i := range f.Clauses {
		if f.Clauses[i].Owner != whoAny {
			return refuse("random graveyard exile filter with an owner")
		}
	}
	cs := []string{"from=Graveyard", "Random"}
	for _, fl := range flags {
		switch fl {
		case "Mandatory", "Remember":
			cs = append(cs, fl)
		default:
			return refuse("random graveyard exile flag %s", fl)
		}
	}
	for i := range f.Clauses {
		f.Clauses[i].Owner = whoYou // the pool is the controller's own graveyard
	}
	lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: n, F: f, C: zExile, CS: strings.Join(cs, "|")})
	return nil
}
