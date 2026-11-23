package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// grantTapSacScen: p0 controls Lotus Ring attached to a creature (tapped or
// summoning-sick as asked) and nothing else. The table's granted ability's
// cost is changed to the upstream "{T}, Sacrifice this creature" shape: the
// corpus card still carries the sacrifice-only script until the fetch lands,
// so the surgery stands in for it and the granted {T}, Sacrifice path
// (mana_grantcost.go tapSacBranch) runs on a real table.
func grantTapSacScen(t *testing.T, tapped, sick bool) (sc *scen, ring, crt Slot, crtObj Obj) {
	t.Helper()
	tbl, id := grantRingTable(t)
	cd := &tbl.Cards[id["Lotus Ring"]]
	fixed := false
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind == kop.AbMana && a.AFlags&kop.AfLatent != 0 && a.Cost.Flags == kop.CostSacSelf {
			a.Cost.Flags = kop.CostTap | kop.CostSacSelf
			fixed = true
		}
	}
	if !fixed {
		t.Fatal("the table's Lotus Ring carries no granted sacrifice-only mana ability")
	}
	sc = newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	ring = sc.slot(sc.perm(id["Lotus Ring"], 0))
	crtObj = sc.perm(id["Grand Abolisher"], 0)
	crt = sc.slot(crtObj)
	sc.s.attachTo(sc.k, ring, crt)
	if tapped {
		sc.s.perms[crt].flags |= pfTapped
	}
	if sick {
		sc.s.perms[crt].flags |= pfSick
	}
	return sc, ring, crt, crtObj
}

// TestGrantTapSacPrefix: the equipped creature's granted "{T}, Sacrifice this
// creature: Add three mana of any one color" is priced as the tap grant it
// is: a tapped or a summoning-sick bearer cannot activate it (CR 302.6), and
// an untapped one's prefix search finds one step that taps the creature,
// floats {G}{G}{G} and sacrifices it, unattaching the Ring at the state-based
// check (CR 704.5n); the replay (grantTapSacApply) does the same.
func TestGrantTapSacPrefix(t *testing.T) {
	cost := kop.ManaCost{}
	cost.Pips[kop.G] = 3
	need := grantNeedColours(cost)
	for _, v := range []struct {
		name         string
		tapped, sick bool
	}{{"untapped", false, false}, {"tapped", true, false}, {"sick", false, true}} {
		t.Run(v.name, func(t *testing.T) {
			sc, ring, crt, crtObj := grantTapSacScen(t, v.tapped, v.sick)
			s, k := sc.s, sc.k
			// preconditions: the creature holds one granted {T}, Sacrifice
			// ability, the Ring is on it, and the printed sources do not pay
			// the cost
			var gi [maxGrants]int16
			if n := s.grantIdxs(k, crt, &gi); n != 1 {
				t.Fatalf("the equipped creature: %d granted abilities, want one", n)
			}
			gv := s.grantView(k.gen, gi[0], crt)
			if !gv.manaTap || !gv.manaSac || gv.grantSac {
				t.Fatalf("the granted view is not a {T}, Sacrifice tap grant: tap %v sac %v grantSac %v", gv.manaTap, gv.manaSac, gv.grantSac)
			}
			if s.perms[ring].attached != crt || !sc.onBF(crtObj) {
				t.Fatal("Lotus Ring is not attached to the creature on the battlefield")
			}
			if s.affordsUngrantedFor(k, 0, &cost, NoSlot, payDesc{}) {
				t.Fatal("the printed sources pay {G}{G}{G}: the board does not need the grant")
			}
			canTap := !v.tapped && !v.sick
			if s.canTapForMana(k, crt) != canTap {
				t.Fatalf("canTapForMana %v, want %v: the {T} in the granted cost", s.canTapForMana(k, crt), canTap)
			}
			var g grantSrch
			if !canTap {
				if s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, need, &g) {
					t.Fatal("a tapped or summoning-sick bearer cannot activate the granted {T}, Sacrifice ability (CR 302.6)")
				}
				if z := s.objs[crtObj].zone; z != kop.ZoneBattlefield {
					t.Errorf("the creature is in zone %d after the refused search, want the battlefield", z)
				}
				return
			}
			if !s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, need, &g) || g.np != 1 || !g.path[0].sac || g.path[0].col != kop.G || g.path[0].n != 3 {
				t.Fatalf("the grant search: np %d, want one step tapping and sacrificing the creature for {G}{G}{G}", g.np)
			}
			s.grantApply(k, &g)
			if z := s.objs[crtObj].zone; z != kop.ZoneGraveyard {
				t.Errorf("the creature is in zone %d after the prefix, want the graveyard", z)
			}
			if s.perms[ring].attached != NoSlot {
				t.Errorf("Lotus Ring still attached to slot %d after the prefix", s.perms[ring].attached)
			}
			if s.pl[0].pool[kop.G] != 3 {
				t.Errorf("pool %v, want {G}{G}{G}", s.pl[0].pool)
			}
		})
	}
}

// TestGrantTapSacWindow: the manual mana window owing {R} takes the granted
// "{T}, Sacrifice this creature" ability of an untapped bearer: it taps the
// creature, floats {R} and sacrifices it, unattaching the Ring; a tapped or
// summoning-sick bearer is no source (CR 302.6) and keeps the creature.
func TestGrantTapSacWindow(t *testing.T) {
	cost := kop.ManaCost{}
	cost.Pips[kop.R] = 1
	for _, v := range []struct {
		name         string
		tapped, sick bool
		want         bool
	}{{"untapped", false, false, true}, {"tapped", true, false, false}, {"sick", false, true, false}} {
		t.Run(v.name, func(t *testing.T) {
			sc, ring, crt, crtObj := grantTapSacScen(t, v.tapped, v.sick)
			s, k := sc.s, sc.k
			// precondition: the creature has no own mana ability (the window
			// takes the grant, not an own ability)
			if k.cards[s.cardAt(crt)].manaAb >= 0 {
				t.Fatal("the creature has an own mana ability: the window need not take the grant")
			}
			ok, _ := s.windowTap(k, 0, &cost, true)
			if ok != v.want {
				t.Fatalf("window ok %v, want %v", ok, v.want)
			}
			if !v.want {
				if z := s.objs[crtObj].zone; z != kop.ZoneBattlefield {
					t.Errorf("the creature is in zone %d after the refused window, want the battlefield", z)
				}
				if s.perms[ring].attached != crt {
					t.Errorf("Lotus Ring unattached after the refused window")
				}
				if s.pl[0].pool[kop.R] != 0 {
					t.Errorf("pool %v, want no {R}", s.pl[0].pool)
				}
				return
			}
			if z := s.objs[crtObj].zone; z != kop.ZoneGraveyard {
				t.Errorf("the creature is in zone %d after the window, want the graveyard", z)
			}
			if s.perms[ring].attached != NoSlot {
				t.Errorf("Lotus Ring still attached to slot %d after the window", s.perms[ring].attached)
			}
			if s.pl[0].pool[kop.R] != 3 {
				t.Errorf("pool %v, want three {R} (the grant's \"three mana of any one color\" answers the owed {R})", s.pl[0].pool)
			}
		})
	}
}
