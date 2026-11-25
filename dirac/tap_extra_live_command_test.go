package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapExtraLiveSeesCommandZoneVanguard is the sentinel for the grant-play
// pass's tapExtraLive: with Sisay seated in the command zone and no
// battlefield StatTapExtra anywhere, the board still has a tap extra in force.
// A command-zone Vanguard static has no perm slot, so plain eachStatic does
// not visit it; tapExtraLive must walk eachTapExtra.
func TestTapExtraLiveSeesCommandZoneVanguard(t *testing.T) {
	t.Parallel()
	set := cardsettest.Set(t, "tapsformana")
	decks := [2][]cardset.Entry{
		{{Name: "Sisay", Count: 1}, {Name: "Forest", Count: 39}},
		{{Name: "Forest", Count: 40}},
	}
	tbl, da, db, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: the table's Sisay entry is the lowered Vanguard shape
	// with the command-zone reflected tap extra.
	var def *kop.CardDef
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Sisay" {
			def = &tbl.Cards[i]
			break
		}
	}
	if def == nil {
		t.Fatal("Sisay is not in the matchup's table")
	}
	if def.Types&kop.TVanguard == 0 {
		t.Fatalf("Sisay's types %#x are not Vanguard", def.Types)
	}
	extra := false
	for _, a := range def.Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra &&
			a.Static.Zone == kop.ZoneCommand && a.Static.StFlags&kop.StTapReflect != 0 {
			extra = true
		}
	}
	if !extra {
		t.Fatalf("Sisay carries no command-zone reflected StatTapExtra: %+v", def.Abilities)
	}

	cfg := Config{SeatCommanders: true}
	for p := range cfg.Commanders {
		cfg.Commanders[p] = [2]int16{-1, -1}
	}
	cfg.Commanders[0][0] = 0 // deck index 0 is Sisay
	var s State
	s.Init(tbl, da, db, 1, cfg)
	k := s.k()
	if k.gen == nil || !k.gen.anyTapExtra || !k.gen.anyCmdTapExtra {
		t.Fatalf("the table's prep does not carry the command-zone tap extra: %+v", k.gen)
	}
	if k.manaEach {
		t.Fatal("precondition: the all-at-once (manaEach) leg would decide the test, not the command-zone static")
	}
	// Precondition: the Vanguard is seated in the command zone and plain
	// eachStatic sees no StatTapExtra at all.
	o := s.pl[0].cmd
	if o == NoObj || s.objs[o].zone != kop.ZoneCommand || k.t.Cards[s.objs[o].card].Name != "Sisay" {
		t.Fatalf("Sisay is not seated in seat 0's command zone: obj %d", o)
	}
	plain := 0
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatTapExtra {
			plain++
		}
		return true
	})
	if plain != 0 {
		t.Fatalf("precondition: %d battlefield/emblem StatTapExtra statics, want none", plain)
	}
	if !s.tapExtraLive(k) {
		t.Fatal("tapExtraLive missed the command-zone Vanguard tap extra; the grant-play pass would drop plays it funds")
	}
}
