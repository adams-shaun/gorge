package kgen

import "strings"

// randomGraveMove lowers a Hidden$ True ChangeZone Origin$ Graveyard with
// AtRandom$ True over the controller's own graveyard to an Exile (Tersa
// Lightshatter: "exile a card at random from your graveyard"), a Hand
// (Haunted Fengraf, Ghoulraiser, Charmbreaker Devils, Woodland Sleuth, Make a
// Wish) or the Battlefield (Moldgraf Monstrosity). gorge effHiddenPick takes
// the engine's seeded pick (randomObjIDs: Rand(len) per card over the
// eligible pool in graveyard order, no draw when the pool is no larger than
// the count) instead of asking, then moves the picks as an answered pick
// would. A constant ChangeNum$ and DefinedPlayer$ You, or none beside a
// YouCtrl filter (gorge's game-wide pool narrowed to the controller's own
// cards: off the battlefield a card's controller is its owner); no
// Optional$ (gorge would ask the yes/no first), no WithTotalCMC$.
// RememberChanged$ remembers the moved cards. Kernel OpChooseMove with
// SrRandom (act_randomgravepick.go).
func (lc *lowerCtx) randomGraveMove(ps *params, f *lFilter, n lValue, flags []string, counterKind string, override *lOverride, dest int) error {
	if !ps.is("Hidden", "True") {
		return refuse("random graveyard move without Hidden$")
	}
	if counterKind != "" || override != nil {
		return refuse("random graveyard move with counters/override")
	}
	if !n.isConst() || n.N < 1 || n.N > 255 {
		return refuse("random graveyard move ChangeNum$ %v", n)
	}
	for _, k := range []string{"Optional", "WithTotalCMC", "ChooseFromDefined", "ValidTgts", "ForgetOtherRemembered"} {
		if _, ok := ps.m[k]; ok {
			return refuse("random graveyard move with %s$", k)
		}
	}
	dp := ps.str("DefinedPlayer")
	if dp != "" && dp != "You" {
		return refuse("random graveyard move DefinedPlayer$ %q", dp)
	}
	if dest == zExile && (dp != "You" || n.N != 1 || !ps.is("Mandatory", "True")) {
		// Tersa Lightshatter's shape only: the exile destination keeps its
		// original gate
		return refuse("random graveyard exile outside the DefinedPlayer$ You, ChangeNum$ 1, Mandatory$ shape")
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("random graveyard move filter %s", f)
	}
	for i := range f.Clauses {
		c := &f.Clauses[i]
		if c.Owner != whoAny {
			return refuse("random graveyard move filter with an owner")
		}
		if dp == "" && c.Ctrl != whoYou {
			return refuse("random graveyard move without DefinedPlayer$ over a pool that is not YouCtrl")
		}
		if dp == "You" && c.Ctrl != whoAny {
			return refuse("random graveyard move DefinedPlayer$ You with a controller")
		}
	}
	cs := []string{"from=Graveyard", "Random"}
	for _, fl := range flags {
		switch fl {
		case "Mandatory", "Remember":
			cs = append(cs, fl)
		default:
			return refuse("random graveyard move flag %s", fl)
		}
	}
	for i := range f.Clauses {
		f.Clauses[i].Owner = whoYou // the pool is the controller's own graveyard
		f.Clauses[i].Ctrl = whoAny
	}
	lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: n, F: f, C: dest, CS: strings.Join(cs, "|")})
	return nil
}
