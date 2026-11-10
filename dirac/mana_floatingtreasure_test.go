package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSmaugFloatingTreasureManaTriggers: Smaug, Wicked Worm's "mana from a
// Treasure was spent to cast it" (Tf3ManaFromTreasure, the cast ring's
// CastSpecTreasure bit) reads the mana the cast spent (CR 106.4, 601.2h), not
// the Treasures tapped before it. Sources tapped at priority float; gorge
// pays a colour's plain units before a Treasure's (pay.takeUnit), so the
// Treasure's mana is spent only once the slot's plain units are gone.
func TestSmaugFloatingTreasureManaTriggers(t *testing.T) {
	cases := []struct {
		name      string
		plains    int
		treasures int
		want      bool
	}{
		// the pool W alone is the Treasure's: it pays {W}
		{"treasure_mana_is_the_only_unit", 0, 1, true},
		// W W, one a Treasure's: the plain unit pays and the Treasure's {W}
		// stays floating (pairs 716 and 1049: floating Treasure mana beside
		// plain mana of the colour spent)
		{"floating_treasure_mana_unspent", 1, 1, false},
		{"floating_treasure_pair_unspent", 1, 2, false},
		// W W, both Treasures': one of two units goes, so the slot is not
		// emptied yet a Treasure's mana was spent
		{"one_of_two_floating_treasures_spent", 0, 2, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc, id := ktreScen(t)
			lions, plains := id("Savannah Lions"), id("Plains")
			set := cardsettest.Set(t, "sacmana")
			treasure := CardID(set.PoolID(set.NumCards()))
			var srcs []Obj
			for i := 0; i < c.plains; i++ {
				srcs = append(srcs, sc.perm(plains, 0))
			}
			for i := 0; i < c.treasures; i++ {
				srcs = append(srcs, sc.perm(treasure, 0))
			}
			// the plain mana is tapped first, the Treasures sacrificed after it
			for _, o := range srcs {
				if !sc.s.tapOneForMana(sc.k, 0, sc.slot(o)) {
					t.Fatalf("source %d does not tap for mana at priority", o)
				}
			}
			tre := 0
			for _, n := range sc.s.g.poolTre[0] {
				tre += int(n)
			}
			if pool := sc.s.pl[0].pool[kop.W]; int(pool) != c.plains+c.treasures || tre != c.treasures {
				t.Fatalf("floating before the cast: %d W of which %d Treasure, want %d of %d", pool, tre, c.plains+c.treasures, c.treasures)
			}
			b := sc.hand(lions, 0)
			sc.kcastBuild(b, lions, 0)
			sc.s.manaStep(sc.k)
			sc.s.finishBuild(sc.k)
			if got := sc.s.castPushes(0)[0].prov&castTreasurePush != 0; got != c.want {
				t.Fatalf("cast ring's Treasure bit (the build's bflTre, which castTreasureOK reads): got %v, want %v", got, c.want)
			}
		})
	}
}
