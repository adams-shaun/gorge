package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// spawnFiveScen seats seat 0's board of lockstep pair 27: nPath Paths of
// Annihilation, two untapped Eldrazi Spawn tokens and two summoning-sick
// ones, no other mana source, Imoti, Celebrant of Bounty ({3}{G}{U}) in hand.
func spawnFiveScen(t *testing.T, nPath int) (*scen, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Path of Annihilation", Count: 4}, {Name: "Imoti, Celebrant of Bounty", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 10}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeLowering: true})
	for i := 0; i < nPath; i++ {
		sc.perm(tableID(t, tbl, "Path of Annihilation"), 0)
	}
	tok := tableID(t, tbl, "c_0_1_eldrazi_spawn_sac")
	for i := 0; i < 4; i++ {
		o := sc.obj(tok, 0)
		sc.s.objs[o].of |= ofToken
		sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
		if i < 2 {
			sc.s.perms[sl].flags &^= pfSick
		}
	}
	sc.s.npend = 0
	return sc, sc.hand(tableID(t, tbl, "Imoti, Celebrant of Bounty"), 0)
}

// TestSpawnTapsAndSacsFundFiveMana: two untapped Spawns under Path of
// Annihilation each tap for any colour and are then sacrificed for {C}, and
// two sick ones are sacrificed too: six units pay {3}{G}{U}, so gorge offers
// the cast and the kernel must (CR 602.2b, 605.1a). Under one Path or two.
func TestSpawnTapsAndSacsFundFiveMana(t *testing.T) {
	for _, nPath := range []int{1, 2} {
		sc, imoti := spawnFiveScen(t, nPath)
		d := sc.s.Step()
		found := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(imoti)) {
				found = true
			}
		}
		if !found {
			t.Errorf("%d Path(s): Imoti's {3}{G}{U} not offered beside six Spawn units: %v", nPath, d.Options[:d.N])
		}
	}
}
