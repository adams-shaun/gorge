package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// grantGapScen is the grantedrestrict pool with p0's board: the granting
// permanent (grantor) and an artifact creature that holds the grant. It
// returns the scenario, the holder's slot and the granted ability's index.
func grantGapScen(t *testing.T, grantor string) (sc *scen, sl Slot, gi int16) {
	t.Helper()
	set := cardsettest.Set(t, "grantedrestrict")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id[grantor], 0)
	sl = sc.slot(sc.perm(id["Ornithopter"], 0))
	g := sc.s.grantIdx(sc.k, sl)
	if g < 0 || sc.k.gen.grantRestr[g] == 0 {
		t.Fatalf("%s: Ornithopter holds no restricted granted mana ability (grant %d)", grantor, g)
	}
	return sc, sl, int16(g)
}

// TestGrantedRestrictGapFloats: a restricted grant tapped as a gap of the
// prefix script (tapGrantedGap, and a mixed gap's otherGrant step) floats
// its mana as the grant's own ability does (floatRestrAb): the batch carries
// the activated half of the restriction (Ambulatory Reactor's abilities of
// artifact sources) and the non-spell flag (Battery Bearer's "can't be
// spent to cast a nonartifact spell"), so the mana pays an Equip as gorge's
// does (CR 106.6, 602.2).
func TestGrantedRestrictGapFloats(t *testing.T) {
	for _, grantor := range []string{"Battery Bearer", "Ambulatory Reactor"} {
		t.Run(grantor, func(t *testing.T) {
			ref, rsl, rgi := grantGapScen(t, grantor)
			ab := restrAbOfGrant(ref.k, rgi)
			if ab == nil {
				t.Fatalf("grant %d has no restricted ability", rgi)
			}
			if ref.k.gen.restrVarOf(ab) == 0 {
				t.Fatalf("%s: the grant's restriction has no activated half or flag; the test cannot tell a variant-0 float", grantor)
			}
			v := ref.k.gen.restrVarOf(ab)
			k0, ok0 := ref.k.gen.restrKindV(ab.ManaRestrict, 0)
			kv, okv := ref.k.gen.restrKindV(ab.ManaRestrict, v)
			if !ok0 || !okv || k0 == kv {
				t.Fatalf("%s: the spell-only batch kind %d (%v) is the grant's kind %d (%v); a float without the activated half would go unseen", grantor, k0, ok0, kv, okv)
			}
			if !ref.s.floatRestrAb(ref.k, 0, rsl, ab, int(kop.C), 1) {
				t.Fatal("the reference float failed")
			}
			want := ref.s.rb[0]

			// the tap-another prefix script's granted gap
			sc, sl, _ := grantGapScen(t, grantor)
			sc.s.tapGrantedGap(sc.k, 0, sl)
			if sc.s.perms[sl].flags&pfTapped == 0 || sc.s.pl[0].pool[kop.C] != 0 {
				t.Fatalf("tapGrantedGap: tapped %v, plain pool C %d; want a tapped source and the mana restricted",
					sc.s.perms[sl].flags&pfTapped != 0, sc.s.pl[0].pool[kop.C])
			}
			if sc.s.rb[0] != want {
				t.Errorf("tapGrantedGap floated %v; the grant's own float is %v", sc.s.rb[0], want)
			}

			// a mixed gap's granted step
			sc2, sl2, gi2 := grantGapScen(t, grantor)
			sc2.s.otherDo(sc2.k, 0, filtStep{sl: sl2, other: otherStep{kind: otherGrant, ans: uint8(kop.C), n: 1, gi: gi2}})
			if sc2.s.perms[sl2].flags&pfTapped == 0 || sc2.s.pl[0].pool[kop.C] != 0 {
				t.Fatalf("otherGrant: tapped %v, plain pool C %d; want a tapped source and the mana restricted",
					sc2.s.perms[sl2].flags&pfTapped != 0, sc2.s.pl[0].pool[kop.C])
			}
			if sc2.s.rb[0] != want {
				t.Errorf("otherGrant floated %v; the grant's own float is %v", sc2.s.rb[0], want)
			}
		})
	}
}

// TestRestrictedGrantRefusedByHandCastCensus: a hand cast's affordability
// census leaves out a restricted granted mana ability the cast's payment
// cannot spend, and with it the tap's extra on that source (CR 106.6, 605.3a;
// gorge potentialRestrictionAdmits). Ambulatory Reactor's {C} spends only on
// an artifact spell or an artifact source's ability, so with Forsaken
// Monument's extra it must not make a non-artifact {3}{U} spell payable from
// two Islands.
func TestRestrictedGrantRefusedByHandCastCensus(t *testing.T) {
	cast := func(t *testing.T, islands int, monument bool, hand string) bool {
		t.Helper()
		sc, id := synthScen(t, "std-trc", "Ambulatory Reactor", "Forsaken Monument", "Pelia, Immortal Innovator", "Island")
		for i := 0; i < islands; i++ {
			sc.perm(id["Island"], 0)
		}
		rs := sc.slot(sc.perm(id["Ambulatory Reactor"], 0))
		if g := sc.s.grantIdx(sc.k, rs); g < 0 || sc.k.gen.grantRestr[g] == 0 {
			t.Fatalf("the Reactor holds no restricted granted mana ability (grant %d)", g)
		}
		if monument {
			sc.perm(id["Forsaken Monument"], 0)
		}
		sc.perm(id["Ambulatory Reactor"], 1) // Pelia's ETB target
		o := sc.hand(id[hand], 0)
		d := sc.s.Step()
		return hasOption(&d, MakeAction(AkCast, uint8(o), 0, 0))
	}
	if cast(t, 2, true, "Pelia, Immortal Innovator") {
		t.Fatal("Pelia offered on 2 Islands + Reactor + Monument: the Reactor's restricted {C} and the Monument's extra were counted for a nonartifact spell")
	}
	if !cast(t, 4, false, "Pelia, Immortal Innovator") {
		t.Fatal("Pelia not offered on 4 Islands + Reactor: the filter dropped an unrestricted payment")
	}
	if !cast(t, 2, true, "Ambulatory Reactor") {
		t.Fatal("a second Reactor (an artifact spell) not offered on 2 Islands + Reactor + Monument: the filter is not keyed on the cast")
	}
}
