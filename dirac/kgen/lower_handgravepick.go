package kgen

import "strings"

// handGravePick lowers the hidden pick over Origin$ Hand,Graveyard
// (handgrave). gorge effChangeZone sends a mixed origin that includes Hand,
// with no concrete object selector, to effSearchLibrary (engine/effects/
// zone_change.go:456-473): for the fetch player (the controller) the pool is
// each named zone of THAT player in Origin$ order (zone_search.go:136-149),
// one decision, Min 0 (the ChangeType$ states a quality), Max = min(ChangeNum$,
// pool); Optional$ You/True asks the fetch player yes/no first, even on an
// empty pool (:86-110). WithCountersType$ folds into the entry (:1331-1335),
// Tapped$ is a tap after the move (:1359-1361), nothing is shuffled (:1186).
//
// Accepted: Destination$ Battlefield, a non-empty card filter (no player
// part; Owner/Ctrl may be unstated or "you", gorge walks only the fetcher's
// zones), ChangeNum$ absent or anything amount reads, Hidden$ True,
// Tapped$ True, Optional$ True/You, WithCountersType$ at amount 1. Every other
// parameter refuses.
func (lc *lowerCtx) handGravePick(ps *params, dest int, flags []string, ctr string, override *lOverride) error {
	if dest != zBattlefield {
		return refuse("Origin$ Hand,Graveyard to %s", zoneLabel[dest])
	}
	for _, k := range []string{"StaticEffect", "ValidTgts", "Defined", "DefinedPlayer", "Chooser", "AnyNumber", "RememberChanged", "ForgetChanged", "GainControl", "Attacking", "LibraryPosition", "Mandatory", "WithCountersAmount"} {
		if _, ok := ps.get(k); ok {
			return refuse("Hand,Graveyard pick with %s$", k)
		}
	}
	if override != nil {
		return refuse("Hand,Graveyard pick with StaticEffect$")
	}
	ct, ok := ps.get("ChangeType")
	if !ok || strings.TrimSpace(ct) == "" {
		return refuse("Hand,Graveyard pick without ChangeType$")
	}
	if v, ok := ps.get("Hidden"); ok && strings.TrimSpace(v) != "True" {
		return refuse("Hand,Graveyard pick Hidden$ %q", v)
	}
	f, err := lc.filter(ct)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("Hand,Graveyard pick filter %s", f)
	}
	for _, c := range f.Clauses {
		if c.Owner > whoYou || c.Ctrl > whoYou {
			return refuse("Hand,Graveyard pick filter %s", f)
		}
	}
	n, err := lc.amount(ps, "ChangeNum", 1)
	if err != nil {
		return err
	}
	cs := []string{"from=Hand,Graveyard"}
	for _, fl := range flags {
		if fl != "Tapped" {
			return refuse("Hand,Graveyard pick flag %s", fl)
		}
		cs = append(cs, fl)
	}
	if ctr != "" {
		if _, err := counterKind(ctr); err != nil {
			return err
		}
		cs = append(cs, "counters="+ctr)
	}
	if o, ok := ps.get("Optional"); ok {
		switch strings.TrimSpace(o) {
		case "True", "You":
			cs = append(cs, "Confirm")
		default:
			return refuse("Hand,Graveyard pick Optional$ %q", o)
		}
	}
	ps.consume("SelectPrompt", "ChangeTypeDesc", "ChangeNumDesc")
	lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: n, F: f, C: dest, CS: strings.Join(cs, "|")})
	return nil
}
