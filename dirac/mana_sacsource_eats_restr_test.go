package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXWitnessEatsCreditsRestrictedLeftover: the eats probe prices the plan
// the non-eats taps use. A lowered {X}{U}{U} sorcery targeting the only
// artifact, with restricted {U}{R}{R} floating and two Islands, announces a
// witness X the restricted leftover L funds ({X} folded at x, L generic off,
// CR 106.6). The probe's fork must price that same credited plan: pricing the
// uncredited cost leaves it L short, finds no plan, and the non-eats block
// taps for the cheapest plan, sacrificing the spell's only target (the
// Treasure). With L = 0 the behaviour is unchanged (the control subtest).
func TestXWitnessEatsCreditsRestrictedLeftover(t *testing.T) {
	run := func(t *testing.T, float bool) {
		set := cardsettest.Set(t, "town")
		tbl0, err := set.Pool()
		if err != nil {
			t.Fatal(err)
		}
		tbl := *tbl0
		tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
		tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant | kop.TSorcery, Ctrl: kop.CtrlYou})
		castOrAct := kop.FilterID(len(tbl.Filters) - 1)
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact})
		artF := kop.FilterID(len(tbl.Filters) - 1)
		mkLand := func(name string, col kop.ColorMask, restricted bool) kop.CardDef {
			a := kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: col, ManaN: 1}
			if restricted {
				a.ManaRestrict, a.ManaRestrictAb = castOrAct, castOrAct
			}
			return kop.CardDef{Name: name, Types: kop.TLand, Abilities: []kop.Ability{a}}
		}
		isle := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, mkLand("Test Isle", kop.ColU, false))
		usp := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, mkLand("Test Blue Spring", kop.ColU, true))
		rsp := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, mkLand("Test Red Spring", kop.ColR, true))
		treasure := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Treasure", Types: kop.TArtifact,
			Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostSacSelf, N: 1},
				ManaColors: kop.ColAny, ManaN: 1}}})
		melt := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Melt", Types: kop.TSorcery,
			Cost: kop.ManaCost{X: 1, Pips: [kop.NumManaKinds]uint8{kop.U: 2}},
			Abilities: []kop.Ability{{Kind: kop.AbSpell, NTargets: 1,
				Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: artF, Min: 1, Max: 1}}}}})

		sc := newScenTableLib(t, &tbl, isle, Config{NoElide: true})
		s := sc.s
		// the full surface lowers the hand cast gorge's cast planner never prices
		s.cfg = s.cfg&^cfgGorgeAutopay | cfgGorgeLowering

		// The springs' mana is already floating restricted, as earlier taps
		// left it.
		springs := []struct {
			o   Obj
			col int
		}{{sc.perm(usp, 0), int(kop.U)}, {sc.perm(rsp, 0), int(kop.R)}, {sc.perm(rsp, 0), int(kop.R)}}
		if float {
			for _, e := range springs {
				sl := sc.slot(e.o)
				c := s.cardAt(sl)
				rab := &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)]
				if !s.floatRestrAb(sc.k, 0, sl, rab, e.col, 1) {
					t.Fatalf("precondition: the %s's mana did not float restricted", sc.k.t.Cards[c].Name)
				}
				s.perms[sl].flags |= pfTapped // the tap that produced it
			}
		}

		isles := []Obj{sc.perm(isle, 0), sc.perm(isle, 0)}
		tre := sc.perm(treasure, 0)
		if !sc.onBF(tre) {
			t.Fatal("precondition: the Treasure is not on the battlefield")
		}
		if sc.tapped(isles[0]) || sc.tapped(isles[1]) {
			t.Fatal("precondition: an Island entered tapped")
		}
		if float {
			if s.restrN(0) != 3 || s.restrOf(0, int(kop.U)) != 1 || s.restrOf(0, int(kop.R)) != 2 {
				t.Fatalf("precondition: restricted %v, want one {U} and two {R}", s.rb[0])
			}
		} else if s.restrN(0) != 0 {
			t.Fatalf("precondition: %d restricted mana floating, want none", s.restrN(0))
		}
		if spec := tspec(sc.k, melt, spellAb(sc.k, melt), 0); spec == nil || spec.Filter != artF {
			t.Fatalf("precondition: the spell does not target the artifact filter")
		}

		spell := sc.hand(melt, 0)
		sc.do(t, AkCast, uint8(spell), 0xff)
		if d := s.Step(); d.Kind != DkChoose || d.Sub != ChX {
			t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
		}
		if z := s.objs[tre].zone; z != kop.ZoneBattlefield {
			t.Fatalf("the X witness's payment sacrificed the spell's only target: Treasure zone %d", z)
		}
		if float && (!sc.tapped(isles[0]) || !sc.tapped(isles[1])) {
			t.Fatalf("Islands tapped %v/%v: the keeping plan taps both", sc.tapped(isles[0]), sc.tapped(isles[1]))
		}
	}
	t.Run("restricted leftover keeps the only target", func(t *testing.T) { run(t, true) })
	t.Run("no float keeps it at land", func(t *testing.T) { run(t, false) })
}
