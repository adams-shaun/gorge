package kgen

import (
	"sort"
	"strings"
)

// api_venture.go lowers api Venture (CR 701.49; gorge engine/effects/venture.go
// effVenture): the resolving controller ventures into the dungeon. The
// kernel's half is dirac/act_venture.go (the dungeon choice, the room choice)
// over act_initiative.go's marker. A plain venture offers every dungeon script
// but Undercity (CR 701.49a, the card's own "you can't enter this dungeon
// unless you venture into Undercity"), so the carrier names them all, each
// with its rooms' tokens, in key order -- the order the kernel's dungeon ask
// offers them (the pool sorts tokens by key). Undercity comes only with the
// initiative (lower_initiative.go).
func init() {
	apiPS("Venture", (*lowerCtx).venture)
}

func (lc *lowerCtx) venture(ps *params) error {
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" {
		return refuse("Venture Defined$ %q", d)
	}
	if q, ok := ps.get("Dungeon"); ok {
		return refuse("Venture Dungeon$ %q", q)
	}
	var keys []string
	for k := range lc.p.reg.Tokens {
		if isDungeonKey(lc.p, k) && k != undercityKey {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return refuse("no dungeon script in the corpus")
	}
	for _, k := range keys {
		if err := lc.nameDungeon(k); err != nil {
			return err
		}
	}
	lc.emit(lInstr{Op: "Venture", Obj: lRef{Kind: rYou}, Note: "the controller ventures into the dungeon (CR 701.49; dirac/act_venture.go)"})
	return nil
}
