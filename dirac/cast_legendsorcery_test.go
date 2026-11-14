package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Legendary instants and sorceries (CR 205.4e): a player can't cast one
// unless they control a legendary creature or a legendary planeswalker.
// gorge withholds it in legendarySpellLocked; dirac/kgen lowers the lock as
// the spell ability's cast restriction (an IsPresent$ condition), which
// castForbidden reads on every cast route (cast_spellcondition.go). The
// fixture builds that lowered form, so the test pins the kernel half.

type legendCards struct {
	sorc, plainSorc, legInstant, legCreature, legArtifact, legPW, bears CardID
}

func legendSorceryScen(t *testing.T) (*scen, legendCards) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{}
	// the lock kgen lowers for a legendary instant or sorcery
	// (dirac/kgen lowerAbility): "Creature.Legendary+YouCtrl,
	// Planeswalker.Legendary+YouCtrl" present, as the spell ability's cast
	// restriction
	pw := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TPlaneswalker, Ctrl: kop.CtrlYou, Flags2: kop.F2Legendary})
	cr := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags2: kop.F2Legendary, Or: pw})
	if len(tbl.Conds) == 0 {
		tbl.Conds = append(tbl.Conds, kop.Cond{}) // CondID 0 is "always"
	}
	lock := kop.CondID(len(tbl.Conds))
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondPresent, F: cr, Op: kop.CmpGE, A: kop.Const(1)})
	add := func(name string, ty kop.TypeMask) CardID {
		id := CardID(len(tbl.Cards))
		cd := kop.CardDef{Name: name, Cost: kop.ManaCost{Generic: 1}, Types: ty, Power: 2, Toughness: 2}
		if ty&kop.TLegendary != 0 && ty&(kop.TInstant|kop.TSorcery) != 0 {
			cd.Abilities = []kop.Ability{{Kind: kop.AbSpell, Trig: kop.Trigger{Cond: lock}}}
		}
		tbl.Cards = append(tbl.Cards, cd)
		return id
	}
	c := legendCards{
		sorc:        add("Test Legendary Sorcery", kop.TSorcery|kop.TLegendary),
		plainSorc:   add("Test Sorcery", kop.TSorcery),
		legInstant:  add("Test Legendary Instant", kop.TInstant|kop.TLegendary),
		legCreature: add("Test Legend", kop.TCreature|kop.TLegendary),
		legArtifact: add("Test Legendary Relic", kop.TArtifact|kop.TLegendary),
		legPW:       add("Test Legendary Walker", kop.TPlaneswalker|kop.TLegendary),
		bears:       add("Test Bear", kop.TCreature),
	}
	return newScenTable(t, tbl, Config{NoElide: true}), c
}

// castOffered reports whether the priority walk offers p a cast of o.
func (sc *scen) castOffered(p uint8, o Obj) bool {
	sc.s.prio = p
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			return true
		}
	}
	return false
}

// TestLegendarySorceryNeedsLegendaryPermanent: a Legendary Sorcery (or
// instant) is not offered without a legendary creature or planeswalker the
// caster controls, and is offered with one. A nonlegendary sorcery is
// unaffected, a legendary artifact or the opponent's legend does not unlock
// it, and a legend that leaves locks it again.
func TestLegendarySorceryNeedsLegendaryPermanent(t *testing.T) {
	sc, c := legendSorceryScen(t)
	sc.lands(0, v.Plains, 4)
	sorc, plain, inst := sc.hand(c.sorc, 0), sc.hand(c.plainSorc, 0), sc.hand(c.legInstant, 0)
	if !sc.castOffered(0, plain) {
		t.Fatal("setup: a nonlegendary sorcery is not offered")
	}
	if sc.castOffered(0, sorc) || sc.castOffered(0, inst) {
		t.Fatal("a legendary sorcery/instant is offered with no legend in play")
	}
	sc.perm(c.legArtifact, 0)
	sc.perm(c.bears, 0)
	sc.perm(c.legCreature, 1)
	if sc.castOffered(0, sorc) || sc.castOffered(0, inst) {
		t.Fatal("a legendary artifact, a plain creature or the opponent's legend unlocked the cast")
	}
	leg := sc.perm(c.legCreature, 0)
	if !sc.castOffered(0, sorc) || !sc.castOffered(0, inst) || !sc.castOffered(0, plain) {
		t.Fatal("a legendary creature in play did not unlock the legendary sorcery")
	}
	sc.s.removePerm(sc.k, sc.slot(leg), kop.ZoneGraveyard, false)
	if sc.castOffered(0, sorc) {
		t.Fatal("the legend left and the legendary sorcery is still offered")
	}
	sc.perm(c.legPW, 0)
	if !sc.castOffered(0, sorc) {
		t.Fatal("a legendary planeswalker did not unlock the legendary sorcery")
	}
}
