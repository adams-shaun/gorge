package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// grantPlainPrefixScen builds seat 0's board from the mbc pool: every name
// once (untapped, not sick; a name with a "!" prefix enters tapped) and
// Peema Outrider ({2}{G}{G}) in hand. Nirkana Cutthroat (no mana ability,
// untapped) is always on the battlefield as Jaspera's tap cost.
func grantPlainPrefixScen(t *testing.T, board []string) (*scen, map[string]CardID, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Swamp", Count: 40}}
	names := append([]string{"Chromatic Lantern", "Jaspera Sentinel", "Nirkana Cutthroat", "Peema Outrider"}, board...)
	for _, n := range names {
		if n[0] == '!' {
			n = n[1:]
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
		deck[0].Count--
	}
	tbl, ids, _, err := set.Matchup(deck, []cardset.Entry{{Name: "Plains", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range ids {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Peema Outrider"], Config{NoElide: true})
	for _, n := range board {
		tapped := false
		if n[0] == '!' {
			n, tapped = n[1:], true
		}
		o := sc.perm(id[n], 0)
		if tapped {
			sc.s.perms[sc.slot(o)].flags |= pfTapped
		}
	}
	sc.perm(id["Nirkana Cutthroat"], 0)
	return sc, id, sc.hand(id["Peema Outrider"], 0)
}

// TestGrantCapPlainPrefixOffered: a play only the census's separate gap
// units pay — Peema Outrider's {2}{G}{G} funded by three untapped Swamps'
// granted "{T}: Add one mana of any color" (a tapped Chromatic Lantern's
// static, CR 611.3c) beside Jaspera Sentinel's tap-another ability — is
// found by grantCap's grant search and offered on the full surface, where
// the offer builder's join-unit census (availGrant) prices it only when the
// granting source's own ability still counts (the Lantern untapped). Beside
// no Lantern the printed and granted sources make three mana, and the cast
// is not offered.
func TestGrantCapPlainPrefixOffered(t *testing.T) {
	for _, c := range []struct {
		board []string
		want  bool
	}{
		// the pair-133 shape: three untapped Swamps' grants beside a tapped
		// Lantern and a fourth, tapped land
		{[]string{"Swamp", "Swamp", "Swamp", "!Chromatic Lantern", "!Swamp", "Jaspera Sentinel"}, true},
		// the Lantern untapped: the builder's own census prices the cast
		{[]string{"Swamp", "Swamp", "Swamp", "Chromatic Lantern", "!Swamp", "Jaspera Sentinel"}, true},
		// no Lantern: the Swamps print {B} only, and Jaspera is one green unit
		{[]string{"Swamp", "Swamp", "Swamp", "Jaspera Sentinel"}, false},
		// three grants beside a Mountain (no Jaspera) also pay it
		{[]string{"Swamp", "Swamp", "Swamp", "!Chromatic Lantern", "Mountain"}, true},
	} {
		sc, _, peema := grantPlainPrefixScen(t, c.board)
		if got := offersCast(sc.s, peema); got != c.want {
			t.Errorf("board %v: Peema Outrider offered %v, want %v", c.board, got, c.want)
		}
	}
}
