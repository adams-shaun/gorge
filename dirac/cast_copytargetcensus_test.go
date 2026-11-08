package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// copytargetCensusScen boards an untapped Plains, a self-sacrifice any-colour
// Treasure token ("Test Treasure") and, when asked, an artifact-hosing
// Can'tSacrifice static ("Test Quark", Quark, Humble Proprietor's "You can't
// sacrifice Treasure tokens").
func copytargetCensusScen(t *testing.T, withQuark bool) *scen {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	pool, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *pool
	plains := CardID(0xffff)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
			break
		}
	}
	if plains == 0xffff {
		t.Skip("the fdn table has no Plains")
	}
	tbl.Cards = append(append([]kop.CardDef(nil), pool.Cards...),
		kop.CardDef{Name: "Test Treasure", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			ManaColors: kop.ColAny,
			ManaN:      1,
			Cost:       kop.Cost{Flags: kop.CostTap | kop.CostSacSelf, N: 1},
		}}})
	tbl.Filters = append(append([]kop.Filter(nil), pool.Filters...), kop.Filter{Types: kop.TArtifact, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	treasure, quark := CardID(len(tbl.Cards)-1), CardID(len(tbl.Cards)-1)
	if withQuark {
		quark = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Quark", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic,
				Static: kop.Static{Kind: kop.StatCantSacrifice, Filter: kop.FilterID(len(tbl.Filters) - 1)}}}})
	}
	sc := newScenTableLib(t, &tbl, plains, Config{NoElide: true})
	if sc.k.gen == nil {
		t.Fatal("the fdn table has no generated extension")
	}
	sc.perm(plains, 0) // an untapped source is left to tap (gorge's hasUntappedManaSource)
	if withQuark {
		sc.perm(quark, 0)
	}
	o := sc.perm(treasure, 0)
	sc.s.objs[o].of |= ofToken
	return sc
}

// TestWindowCensusSacAnySourceLeavesTheMenuOpen: a Treasure token's
// {T}, Sacrifice any-colour mana ability is a source gorge's CR 601.2g window
// census cannot price while a Can'tSacrifice static blocks it, so a
// pool-short copy cast's target menu keeps its candidates (castWindowCensusComplete's
// incomplete arm) instead of aborting at the target ask (affordableTargetCandidates).
func TestWindowCensusSacAnySourceLeavesTheMenuOpen(t *testing.T) {
	sc := copytargetCensusScen(t, true)
	if !sc.k.sacMana {
		t.Fatal("the table does not arm the sacrifice census guard (sacMana)")
	}
	// precondition: the board really holds a self-sacrifice any-colour source,
	// and the static is what makes the shared census skip it.
	found := false
	for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
		sl := m.first()
		cd := &sc.k.t.Cards[sc.s.cardAt(sl)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbMana || a.Cost.Flags&kop.CostSacSelf == 0 || a.ManaColors != kop.ColAny {
				continue
			}
			found = true
			if !sc.s.sacBlocked(sc.k, sl) {
				t.Fatalf("the Treasure at slot %d is not blocked from being sacrificed", sl)
			}
		}
	}
	if !found {
		t.Fatal("no self-sacrifice any-colour mana source on the board")
	}
	if !sc.s.sacAnyManaGap(sc.k, 0) {
		t.Error("the blocked self-sacrifice any-colour source is not a census gap")
	}
	e := stackEntry{kind: seSpell, ctrl: 0}
	if !sc.s.copyCensusOpen(sc.k, &e) {
		t.Error("the census gap did not keep the pool-short copy target menu open")
	}
	// negative control: without the static the Treasure is usable and priced
	// by the shared census, so the copy menu is not held open.
	plain := copytargetCensusScen(t, false)
	if plain.s.sacAnyManaGap(plain.k, 0) {
		t.Error("a usable Treasure is a census gap")
	}
	if plain.s.copyCensusOpen(plain.k, &e) {
		t.Error("the copy menu is held open beside a usable Treasure")
	}
}
