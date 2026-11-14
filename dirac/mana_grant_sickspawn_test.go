package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sickSpawnScen builds p0's board from an mbc matchup table: Path of
// Annihilation ("Eldrazi you control have {T}: Add one mana of any color")
// and one Eldrazi Spawn token, summoning-sick when sick, no other mana
// source; the named card is in p0's hand.
func sickSpawnScen(t *testing.T, sick bool, play string) (sc *scen, spawn Slot, card Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Path of Annihilation", Count: 1}, {Name: play, Count: 1}, {Name: "Swamp", Count: 30}}
	tbl, _, _, err := cardsettest.Set(t, "mbc").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, n := range []string{"Path of Annihilation", "c_0_1_eldrazi_spawn_sac", "Swamp", play} {
		if _, ok := id[n]; !ok {
			t.Fatalf("%s not in the mbc matchup table", n)
		}
	}
	sc = newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	sc.perm(id["Path of Annihilation"], 0)
	o := sc.perm(id["c_0_1_eldrazi_spawn_sac"], 0)
	sc.s.objs[o].of |= ofToken
	spawn = sc.slot(o)
	if sick {
		sc.s.perms[spawn].flags |= pfSick
	}
	return sc, spawn, sc.hand(id[play], 0)
}

// TestSickSpawnGrantedTapMana: under Path of Annihilation an Eldrazi Spawn
// has "{T}: Add one mana of any color" beside its printed "Sacrifice this
// creature: Add {C}". A summoning-sick Spawn cannot activate the {T} ability
// (CR 302.6) but can the sacrifice one (CR 602.2b), so the offer's census
// credits it {C} only: Young Wolf ({G}) is not offered while the Spawn is
// sick, is offered once it is not, and Sensei's Divining Top ({1}) is offered
// either way.
func TestSickSpawnGrantedTapMana(t *testing.T) {
	for _, v := range []struct {
		name, play string
		sick, want bool
	}{
		{"sick-green", "Young Wolf", true, false},
		{"untapped-green", "Young Wolf", false, true},
		{"sick-colourless", "Sensei's Divining Top", true, true},
	} {
		t.Run(v.name, func(t *testing.T) {
			sc, spawn, card := sickSpawnScen(t, v.sick, v.play)
			s, k := sc.s, sc.k
			// preconditions: the Spawn holds the Path's granted {T} ability
			// (not a sacrifice-only grant), its own view is the printed
			// sacrifice-only one, the granted {T} is usable exactly when not
			// sick, the card is in p0's hand, and the printed sources alone do
			// not pay a coloured pip
			var gi [maxGrants]int16
			if n := s.grantIdxs(k, spawn, &gi); n != 1 || s.grantView(k.gen, gi[0], spawn).grantSac {
				t.Fatalf("the Spawn: %d granted abilities, want the Path's one {T} ability", n)
			}
			if s.canTapForMana(k, spawn) {
				t.Fatal("canTapForMana: the Spawn's own view is not the printed sacrifice-only one")
			}
			if got := s.canTapForManaPC(k, spawn, s.grantView(k.gen, gi[0], spawn)); got == v.sick {
				t.Fatalf("the granted {T} ability usable = %v with sick = %v", got, v.sick)
			}
			if s.objs[card].zone != kop.ZoneHand {
				t.Fatalf("%s in zone %d, want the hand", v.play, s.objs[card].zone)
			}
			green := kop.ManaCost{}
			green.Pips[kop.G] = 1
			if s.affordsUngrantedFor(k, 0, &green, NoSlot, payDesc{}) {
				t.Fatal("the printed sources pay {G}: the board does not need the grant")
			}
			d := s.Step()
			if d.Kind != DkPriority || d.Player != 0 {
				t.Fatalf("decision kind %d player %d, want p0's priority", d.Kind, d.Player)
			}
			if got := castOffered(&d, card); got != v.want {
				t.Errorf("cast %s offered = %v, want %v; options %v", v.play, got, v.want, d.Options[:d.N])
			}
		})
	}
}
