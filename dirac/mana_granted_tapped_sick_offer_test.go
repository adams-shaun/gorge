package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// grantedTappedSickScen is the grantedrestrict pool's grant-only shape:
// Battery Bearer (a creature with no printed mana ability) holds its own
// restricted "{T}: Add {C}" and p0 holds Skullclamp ({1}, an artifact spell
// the restriction admits) to cast. It returns the scenario, the hand cast
// and the grant-only source's slot.
func grantedTappedSickScen(t *testing.T) (*scen, Obj, Slot) {
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
	sc := newScenTableLib(t, tbl, id["Plains"], Config{GorgeLowering: true, NoElide: true})
	sl := sc.slot(sc.perm(id["Battery Bearer"], 0))
	return sc, sc.hand(id["Skullclamp"], 0), sl
}

// grantedOnlyScen is the same shape on a hand-written table: a non-creature
// grantor gives a creature with no mana ability "{T}: Add {R}", so the source
// is grant-only and Shock's {R} is the play the grant funds.
func grantedOnlyScen(t *testing.T) (*scen, Obj, Slot) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}}
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	creatures := kop.FilterID(len(tbl.Filters) - 1)
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Grant Only", Types: kop.TCreature, Power: 1, Toughness: 1})
	grant := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Grantor", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: creatures, Grant: 2}},
			{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColR, ManaN: 1},
		},
	})
	sc := newScenTable(t, &tbl, Config{GorgeLowering: true, NoElide: true})
	sc.perm(grant, 0)
	sl := sc.slot(sc.perm(src, 0))
	return sc, sc.hand(v.Shock, 0), sl
}

// grantAvail is availGrant's own census of sl, the shared unit the offer gate
// reads: the play the grant funds is priced only when the grant-only source
// can tap for it.
func grantAvail(s *State, k *prep, sl Slot) avail {
	var a avail
	s.availGrant(k, sl, s.manaPCGen(k, sl), &a)
	return a
}

// TestGrantOnlyTappedSickSourceNotOffered: a source whose only mana ability is
// one a permanent grants it prices nothing while it cannot tap (CR 302.6,
// 605.1a). The untapped source pays the play; tapped or summoning sick it is
// no unit of the offer gate's availability, as gorge's census reads it.
func TestGrantOnlyTappedSickSourceNotOffered(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*testing.T) (*scen, Obj, Slot)
	}{
		{"grantedrestrict", grantedTappedSickScen},
		{"oneGrant", grantedOnlyScen},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// control: the untapped grant-only source pays the play and its own
			// census is one unit, or the two cases below assert nothing.
			ctl, cast, sl := tc.build(t)
			if !ctl.s.manaGranted(ctl.k, sl) {
				t.Fatal("precondition: the source's mana is not a granted ability")
			}
			if ctl.s.ownManaUsable(ctl.k, sl) {
				t.Fatal("precondition: the source has a usable own mana ability")
			}
			gv := ctl.s.grantView(ctl.k.gen, int16(ctl.s.grantIdx(ctl.k, sl)), sl)
			if !gv.manaTap {
				t.Fatal("precondition: the granted ability does not cost a {T}")
			}
			if !offersCast(ctl.s, cast) {
				t.Fatal("precondition: the play is not offered with the untapped grant-only source")
			}
			if got := grantAvail(ctl.s, ctl.k, sl); got.total != 1 {
				t.Fatalf("precondition: the untapped source prices %d units, want 1", got.total)
			}

			for _, mode := range []string{"tapped", "sick"} {
				t.Run(mode, func(t *testing.T) {
					sc, cast, sl := tc.build(t)
					switch mode {
					case "tapped":
						sc.s.perms[sl].flags |= pfTapped
					case "sick":
						sc.s.perms[sl].flags |= pfSick
					}
					if sc.s.canTapForMana(sc.k, sl) {
						t.Fatalf("%s: the grant-only source still reads as able to tap", mode)
					}
					if offersCast(sc.s, cast) {
						t.Fatalf("%s: the play is offered though the grant-only source cannot tap for it", mode)
					}
					if got := grantAvail(sc.s, sc.k, sl); got.total != 0 {
						t.Fatalf("%s: the grant-only source prices %d units though it cannot tap, want 0", mode, got.total)
					}
				})
			}
		})
	}
}
