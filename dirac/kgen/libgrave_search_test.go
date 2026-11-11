package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLibGraveSearch: Journey for the Elixir's two "search your library and
// graveyard" links (Origin$ Library,Graveyard, no OriginAlternative$) lower to
// two OpSearchEx over the library then the graveyard with the zone election
// settled (SrAltOrigin | SrAltAll, alternative zone Graveyard): the first
// keeps its Shuffle$ False, the second shuffles. Other shapes refuse by name.
func TestLibGraveSearch(t *testing.T) {
	all, bs, _, err := buildPool(lowerNamedPool(t, []string{"Journey for the Elixir"}))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for i, c := range all {
		if c.Name != "Journey for the Elixir" {
			continue
		}
		seen++
		if rs := lowerRefusals(c); len(rs) != 0 {
			t.Fatalf("Journey for the Elixir refused: %v", rs)
		}
		var noShuf []bool
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op != kop.OpSearchEx {
					continue
				}
				if in.B.N&(kop.SrAltOrigin|kop.SrAltAll) != kop.SrAltOrigin|kop.SrAltAll {
					t.Errorf("search B %#x lacks SrAltOrigin|SrAltAll", in.B.N)
				}
				if z := kop.Zone(in.B.N>>kop.SrAltZoneShift) & 7; z != kop.ZoneGraveyard {
					t.Errorf("alt zone %d, want Graveyard", z)
				}
				if kop.Zone(in.C&7) != kop.ZoneHand {
					t.Errorf("destination %d, want Hand", in.C&7)
				}
				noShuf = append(noShuf, in.B.N&kop.SrNoShuffle != 0)
			}
		}
		if len(noShuf) != 2 || !noShuf[0] || noShuf[1] {
			t.Fatalf("searches' NoShuffle %v, want [true false]", noShuf)
		}
	}
	if seen != 1 {
		t.Fatalf("built %d Journey for the Elixir cards, want 1", seen)
	}

	for _, c := range []struct{ name, root, want string }{
		{"battlefield two cards", "SP$ ChangeZone | Origin$ Library,Graveyard | Destination$ Battlefield | ChangeType$ Land.Basic | ChangeNum$ 2", "ChangeNum$"},
		{"graveyard destination", "SP$ ChangeZone | Origin$ Library,Graveyard | Destination$ Graveyard | ChangeType$ Land.Basic", "search to Graveyard"},
		{"with an alternative", "SP$ ChangeZone | Origin$ Library,Graveyard | Destination$ Hand | ChangeType$ Land.Basic | OriginAlternative$ Hand", `Origin "Library,Graveyard"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := czLowered(t, c.root)
			kdefWant(t, got, c.want)
		})
	}
}
