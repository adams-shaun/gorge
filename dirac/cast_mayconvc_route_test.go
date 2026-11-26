package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The two pool-route conversions must not fold into one another. A table
// holding Abstruse Appropriation (an OpMayPlay with the colourless-mana
// conversion, cast_mayconvc.go) beside a StatAnyColorSpells card arms both
// useMayConvC and useAnyColorSpell, and the convC cast's payment must not ride
// bflAnyColor: that flag is the any-colour conversion's route (convRouteFlag,
// mana_convertcreature.go), which would let the convC cast's coloured mana
// cover another colour's pip — exactly what gorge's C->AnyColor arm refuses
// (pay.ApplyConversionTo is wild for {C} alone). The offer gates read both
// conversions (convAnyPoolPays); the route readers read the any-colour arm
// alone.

// convCRouteScen builds a table with a StatAnyColorSpells card (oath, an
// enchantment whose filter matches every card) and a card whose program
// carries an OpMayPlay with the colourless-mana conversion (the instr arms
// useMayConvC; the test writes the record itself), and puts a Divination in
// exile under a live convC may-play record with a floating {C}{C}{C}{C} pool.
// oathOnBF puts the any-colour static on p0's battlefield (convAnyColor's
// read); left off, only its presence in the table (useAnyColorSpell) differs.
func convCRouteScen(t *testing.T, oathOnBF bool) (*scen, Obj, kop.ManaCost) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	oath := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test AnyColour", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatAnyColorSpells}}}})
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test ConvC Grant", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, // never triggered: the test never steps the scenario
			Trig: kop.Trigger{Event: kop.TrigPhase, Step: kop.StepEndCombat, Whose: kop.WhoseYou},
			Prog: []kop.Instr{{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurPermanent),
				C: int16(kop.ZoneExile) | kop.MayPlayConvC}}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if oathOnBF {
		sc.perm(oath, 0)
	}
	o := sc.obj(v.Divination, 0)
	sc.s.place(o, kop.ZoneExile, false)
	// the record the Abstruse grant would have written (mayPlayRemembered with
	// convCMark): a while-exiled permission on the Divination carrying
	// mayTurnConvC, live on this turn
	*sc.s.mayAt(int(sc.s.g.nmay)) = mayPlay{obj: o, ver: mayVerAt(sc.s, o, kop.ZoneExile, false), ctrl: 0,
		turn: mayWord(sc.s.turn) | mayTurnConvC}
	sc.s.g.nmay++
	sc.s.pl[0].pool[kop.C] = 4
	return sc, o, kop.ManaCost{Generic: 1, Pips: [kop.NumManaKinds]uint8{kop.G: 1}}
}

// TestConvRouteFlagConvCNotAnyColor: a convC cast in a table that also holds a
// StatAnyColorSpells card (useAnyColorSpell is table-level; the static is not
// even on the battlefield) leaves bflAnyColor clear, while a cast the
// any-colour conversion itself covers still rides it.
func TestConvRouteFlagConvCNotAnyColor(t *testing.T) {
	t.Run("convC leaves bflAnyColor clear", func(t *testing.T) {
		sc, o, cost := convCRouteScen(t, false)
		k := sc.k
		// the offer gate reads both conversions: the floating {C}{C}{C}{C} pays
		// {1}{G} only through the record's conversion ({G} takes a {C})
		if !sc.s.convAnyPoolPays(k, 0, o, &cost) {
			t.Fatal("the offer gate did not widen the convC cast through the pool")
		}
		// the cast being built under the record: the payment's route read must
		// not fold the convC pool route into the any-colour one
		*sc.s.bld() = build{e: stackEntry{kind: seSpell, card: v.Divination, obj: o, ctrl: 0}, convC: true}
		sc.s.ctl = ctlBuild
		sc.s.convRouteFlag(k, &sc.s.bld().e, cost)
		if sc.s.bld().bfl&bflAnyColor != 0 {
			t.Fatal("convRouteFlag rode bflAnyColor on a convC cast: the C->AnyColor arm folded into the any-colour route")
		}
		// the pool still pays under the conversion: the offer gate's disjunction
		// is what kept it (the any-colour arm alone reads false here)
		if sc.s.convPoolPays(k, 0, o, &cost) {
			t.Fatal("convPoolPays read the any-colour conversion on a cast it does not cover")
		}
	})
	t.Run("the any-colour route still rides", func(t *testing.T) {
		sc, o, cost := convCRouteScen(t, true)
		k := sc.k
		// with the static on the battlefield the cast is under the any-colour
		// conversion (its filter matches every card): both arms read true, and
		// the pool covers the printed pips only as any colour
		if !sc.s.convPoolPays(k, 0, o, &cost) {
			t.Fatal("the any-colour conversion did not widen the cast through the pool")
		}
		*sc.s.bld() = build{e: stackEntry{kind: seSpell, card: v.Divination, obj: o, ctrl: 0}, convC: true}
		sc.s.ctl = ctlBuild
		sc.s.convRouteFlag(k, &sc.s.bld().e, cost)
		if sc.s.bld().bfl&bflAnyColor == 0 {
			t.Fatal("the any-colour conversion's pool route did not ride bflAnyColor")
		}
	})
}
