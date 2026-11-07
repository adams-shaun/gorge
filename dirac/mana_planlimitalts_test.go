package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPlanLimitAltsBound is unitAltsChoice on a source with a five-colour
// choice ability under Leyline of the Guildpact's five granted basic land
// types (mana_planlimitorder.go): gorge's census (pay.appendUnitAlternatives
// over paymentPlanManaUnitsOnlyCompute) lists each granted type's intrinsic
// ability and then the any-colour ability's five colours, ten alternatives
// that fill planLimitAlts exactly. A {1},{T} filter's any-colour ability is
// not tap-only, so gorge lists none of its colours, and a RestrictValid$ one
// counts only when the payment admits it (White Lotus Hideout's shape). A
// second admitted any-colour ability takes the true count to fifteen: the
// list stops at the array and the game stalls with StallPlanAlts, a counted
// capacity stall, never a write past the array.
func TestPlanLimitAltsBound(t *testing.T) {
	set := cardsettest.Set(t, "hybridn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	lib := CardID(0xffff)
	leyline := CardID(0xffff)
	for c := range tbl.Cards {
		switch tbl.Cards[c].Name {
		case "Plains":
			lib = CardID(c)
		case "Leyline of the Guildpact":
			leyline = CardID(c)
		}
	}
	if lib == 0xffff || leyline == 0xffff {
		t.Fatal("the hybridn table lacks Plains or Leyline of the Guildpact")
	}
	tap := kop.Cost{Flags: kop.CostTap}
	filter := kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}}
	anyAb := kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColAny, ManaN: 1}
	filterAb := kop.Ability{Kind: kop.AbMana, Cost: filter, ManaColors: kop.ColAny, ManaN: 1}
	restrAb := kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColAny, ManaN: 1, ManaRestrict: 1}
	cAb := kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: 1 << kop.C, ManaN: 1}
	if !restrSrcAb(&restrAb) || restrSrcAb(&anyAb) {
		t.Fatal("restrSrcAb does not read the test's restricted ability as the planner's restricted source")
	}
	if filterAb.Cost.Mana.MV() != 1 {
		t.Fatalf("the filter's fee has mana value %d; want 1", filterAb.Cost.Mana.MV())
	}
	base := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Any Land", Types: kop.TLand, Abilities: []kop.Ability{anyAb}},
		kop.CardDef{Name: "Test Lotus Hideout", Types: kop.TLand, Abilities: []kop.Ability{cAb, restrAb, filterAb}},
		kop.CardDef{Name: "Test Twin Any Land", Types: kop.TLand, Abilities: []kop.Ability{anyAb, anyAb}},
	)
	sc := newScenTableLib(t, tbl, lib, Config{})
	if !sc.k.gen.blgOn {
		t.Fatal("blgOn unset: Leyline's AddSub carries no basic land type")
	}
	sc.perm(leyline, 0)
	anyO, lotusO, twinO := sc.perm(base, 0), sc.perm(base+1, 0), sc.perm(base+2, 0)
	if bm := sc.s.blgMask(sc.k, sc.slot(anyO)); bm != kop.ColAny {
		t.Fatalf("the land's granted mask %#x; want all five basic land types' %#x", bm, kop.ColAny)
	}
	intrinsic := []lsAlt{{uint8(kop.W), -1}, {uint8(kop.U), -1}, {uint8(kop.B), -1}, {uint8(kop.R), -1}, {uint8(kop.G), -1}}
	of := func(ab int8) []lsAlt {
		return []lsAlt{{uint8(kop.W), ab}, {uint8(kop.U), ab}, {uint8(kop.B), ab}, {uint8(kop.R), ab}, {uint8(kop.G), ab}}
	}
	cat := func(xs ...[]lsAlt) (out []lsAlt) {
		for _, x := range xs {
			out = append(out, x...)
		}
		return out
	}
	for _, tt := range []struct {
		name  string
		o     Obj
		rk    uint8
		want  []lsAlt
		stall uint8
	}{
		{"any-colour under five grants", anyO, 0, cat(intrinsic, of(0)), StallNone},
		{"restricted refused, filter unlisted", lotusO, 0, cat([]lsAlt{{uint8(kop.C), 0}}, intrinsic), StallNone},
		{"restricted admitted past the array", lotusO, rkWiden, cat([]lsAlt{{uint8(kop.C), 0}}, intrinsic, of(1)[:4]), StallPlanAlts},
		{"two any-colour abilities past the array", twinO, 0, cat(intrinsic, of(0)), StallPlanAlts},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sc.s.g.stalled = StallNone
			var pk planner
			pk.types[0].mask = uint8(kop.ColAny) // the blgView union: the printed colours and the granted ones
			if tt.o == lotusO {
				pk.types[0].mask = planLimitAny // and the Hideout's {C}
			}
			pk.units[0], pk.utyp[0], pk.nu, pk.rkind[0] = sc.slot(tt.o), 0, 1, tt.rk
			var alts [planLimitAlts]lsAlt
			n, ok := sc.s.unitAltsChoice(sc.k, &pk, 0, &alts, true)
			if !ok {
				t.Fatal("unitAltsChoice declined: no choice-shaped ability, or no granted type for it to follow")
			}
			if n != len(tt.want) {
				t.Fatalf("%d alternatives %v; want %d %v", n, alts[:n], len(tt.want), tt.want)
			}
			for i, w := range tt.want {
				if alts[i] != w {
					t.Fatalf("alternative %d is %+v; want %+v (all %v)", i, alts[i], w, alts[:n])
				}
			}
			if sc.s.g.stalled != tt.stall {
				t.Fatalf("stalled %d (%s); want %d (%s)", sc.s.g.stalled, StallName(sc.s.g.stalled), tt.stall, StallName(tt.stall))
			}
			if tt.stall != StallNone && StallClassOf(tt.stall) != StallCapacity {
				t.Fatalf("%s is not a counted capacity stall", StallName(tt.stall))
			}
		})
	}
}
