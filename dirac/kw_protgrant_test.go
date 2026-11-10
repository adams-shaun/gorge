package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// protGrantCards are protGrantScen's cards.
type protGrantCards struct {
	sword, blade, bear, instant, sorcery, creature CardID
}

// protGrantScen is a table with two Equipment whose Continuous static grants
// the equipped creature protection from a quality (kop.Static.ProtFrom): a
// Sword (instants and sorceries) and a Blade (creatures), beside a bear, an
// instant, a sorcery and a creature spell.
func protGrantScen(t *testing.T) (*scen, protGrantCards) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Overrides = append([]kop.Override(nil), v.Table.Overrides...)
	tbl.Conds = []kop.Cond{} // the generated-table gate (prepareGen)
	flt := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	equipped := flt(kop.Filter{Types: kop.TCreature, Flags: kop.FAttachedBySrc})
	sorc := flt(kop.Filter{Types: kop.TSorcery})
	inst := flt(kop.Filter{Types: kop.TInstant, Or: sorc})
	crea := flt(kop.Filter{Types: kop.TCreature})
	var c protGrantCards
	next := func(d kop.CardDef) CardID {
		tbl.Cards = append(tbl.Cards, d)
		return CardID(len(tbl.Cards) - 1)
	}
	equip := func(name string, q kop.FilterID) CardID {
		return next(kop.CardDef{Name: name, Types: kop.TArtifact | kop.TEquipment,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: equipped, A: 2, B: 2, ProtFrom: q}}}})
	}
	c.sword = equip("Test Sword", inst)
	c.blade = equip("Test Blade", crea)
	c.bear = next(kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	c.instant = next(kop.CardDef{Name: "Test Instant", Types: kop.TInstant})
	c.sorcery = next(kop.CardDef{Name: "Test Sorcery", Types: kop.TSorcery})
	c.creature = next(kop.CardDef{Name: "Test Creature Spell", Types: kop.TCreature, Power: 1, Toughness: 1})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyProtGrant || !sc.k.gen.anyProtF {
		t.Fatal("precondition: the table does not arm the granted-protection reads (gen.anyProtGrant)")
	}
	return sc, c
}

// protGrantTargetable asks whether a spell (the probe) controlled by p may
// target the battlefield permanent o.
func protGrantTargetable(sc *scen, card CardID, p uint8, o Obj) bool {
	sc.s.g.probe = probe{kind: probeSpell, card: card, ctrl: p}
	defer func() { sc.s.g.probe = probe{} }()
	return sc.s.objTargetable(sc.k, sc.slot(o), p)
}

// TestProtGrantInstantSorcery: an Equipment's static gives the equipped
// creature protection from instants and sorceries: neither can target it
// (the caster's own included), damage from one is prevented, and the
// creature spell is not affected; unequipping ends the grant.
func TestProtGrantInstantSorcery(t *testing.T) {
	sc, c := protGrantScen(t)
	bear := sc.perm(c.bear, 0)
	sword := sc.perm(c.sword, 0)
	bsl, ssl := sc.slot(bear), sc.slot(sword)
	// the precondition every refusal below depends on: nothing protects the bear yet
	for _, spell := range []CardID{c.instant, c.sorcery, c.creature} {
		if !protGrantTargetable(sc, spell, 1, bear) || sc.s.protFrom(sc.k, bsl, NoSlot, spell, 0, false) {
			t.Fatalf("precondition: the bare bear is protected from %s", sc.k.t.Cards[spell].Name)
		}
	}
	sc.s.perms[ssl].attached = bsl
	if sc.s.kw(sc.k, bsl)&(kop.KwProtBlack|kop.KwProtRed|kop.KwProtectionAll) != 0 {
		t.Fatal("the grant is a quality, not a keyword bit")
	}
	for _, spell := range []CardID{c.instant, c.sorcery} {
		name := sc.k.t.Cards[spell].Name
		for _, p := range []uint8{0, 1} {
			if protGrantTargetable(sc, spell, p, bear) {
				t.Errorf("seat %d's %s targeted a creature with protection from instants and sorceries", p, name)
			}
		}
		if !sc.s.protFrom(sc.k, bsl, NoSlot, spell, 0, false) {
			t.Errorf("damage from %s was not prevented", name)
		}
	}
	if !protGrantTargetable(sc, c.creature, 1, bear) || sc.s.protFrom(sc.k, bsl, NoSlot, c.creature, 0, false) {
		t.Error("a creature spell is protected against")
	}
	if !protGrantTargetable(sc, c.instant, 1, sword) {
		t.Error("the Sword itself, not the equipped creature, is protected")
	}
	sc.s.perms[ssl].attached = NoSlot
	for _, spell := range []CardID{c.instant, c.sorcery} {
		if !protGrantTargetable(sc, spell, 1, bear) || sc.s.protFrom(sc.k, bsl, NoSlot, spell, 0, false) {
			t.Errorf("the unequipped bear is still protected from %s", sc.k.t.Cards[spell].Name)
		}
	}
}

// TestProtGrantBlock: a granted protection from creatures stops the
// equipped attacker being blocked by a creature (CR 702.16f) and prevents a
// permanent source's damage to it; the protected creature may still block.
func TestProtGrantBlock(t *testing.T) {
	sc, c := protGrantScen(t)
	att := sc.perm(c.bear, 0)
	blk := sc.perm(c.bear, 1)
	blade := sc.perm(c.blade, 0)
	asl, bsl := sc.slot(att), sc.slot(blk)
	if !sc.s.canBlock(sc.k, bsl, asl) {
		t.Fatal("precondition: the bare attacker can't be blocked")
	}
	sc.s.perms[sc.slot(blade)].attached = asl
	if sc.s.canBlock(sc.k, bsl, asl) {
		t.Error("a creature blocked a creature with protection from creatures")
	}
	if !sc.s.protFrom(sc.k, asl, bsl, c.bear, 0, true) {
		t.Error("combat damage from a creature was not prevented")
	}
	if !sc.s.canBlock(sc.k, asl, bsl) {
		t.Error("the protected creature itself is blocked from blocking")
	}
}
