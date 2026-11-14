package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// newModeTable copies the vanilla table, adds a creatures-you-control
// filter and the cards next builds; the generated-table gate is armed.
func newModeTable(cards ...kop.CardDef) (*kop.Table, kop.FilterID, []CardID) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Overrides = append([]kop.Override(nil), v.Table.Overrides...)
	tbl.Conds = []kop.Cond{}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(tbl.Filters) - 1)
	ids := make([]CardID, len(cards))
	for i, c := range cards {
		for j := range c.Abilities {
			if a := &c.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Filter == 0 {
				a.Static.Filter = f
			}
		}
		ids[i] = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, c)
	}
	return &tbl, f, ids
}

// sickPerm puts c onto the battlefield under p as a creature that came
// under p's control this turn.
func (sc *scen) sickPerm(c CardID, p uint8) Obj {
	o := sc.perm(c, p)
	sc.s.perms[sc.slot(o)].flags |= pfSick
	return o
}

// activateOffered: priority offers an activation of o's ability ab.
func (sc *scen) activateOffered(o Obj, ab uint8) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) && a.Mode() == ab {
			return true
		}
	}
	return false
}

// TestAsIfHasteActivatesNotAttacks: with a StatAsIfHaste static (Shang-Chi,
// Master of Kung Fu) out, a summoning-sick creature taps for mana, pays an
// {Q} cost and is offered its {T} ability, but still can't attack (CR
// 302.6; gorge activatesAsIfHaste is no haste grant). Without the static,
// or under DebugAsIfHasteOff, none of the three is allowed.
func TestAsIfHasteActivatesNotAttacks(t *testing.T) {
	tbl, _, ids := newModeTable(
		kop.CardDef{Name: "Test Shang", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatAsIfHaste}}}},
		kop.CardDef{Name: "Test Elf", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1}}},
		kop.CardDef{Name: "Test Pinger", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostTap}, Prog: []kop.Instr{{Op: kop.OpGainLife, A: kop.Const(1)}}}}},
	)
	shang, elf, pinger := ids[0], ids[1], ids[2]
	run := func(t *testing.T, withShang, off bool) (mana, act, untap, attack bool) {
		t.Helper()
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if sc.k.gen == nil || !sc.k.gen.anyAsIfHaste {
			t.Fatal("precondition: the table does not arm the as-if-haste read (gen.anyAsIfHaste)")
		}
		if withShang {
			sc.perm(shang, 0)
		}
		e := sc.sickPerm(elf, 0)
		pg := sc.sickPerm(pinger, 0)
		el, pl := sc.slot(e), sc.slot(pg)
		if sc.s.perms[el].flags&pfSick == 0 || sc.s.kw(sc.k, el)&kop.KwHaste != 0 || !sc.s.isCreature(sc.k, el) {
			t.Fatal("precondition: the elf is not a summoning-sick creature without haste")
		}
		save := DebugAsIfHasteOff
		DebugAsIfHasteOff = off
		defer func() { DebugAsIfHasteOff = save }()
		mana = sc.s.canTapForManaPC(sc.k, el, &sc.k.cards[elf])
		act = sc.activateOffered(pg, 0)
		sc.s.perms[pl].flags |= pfTapped
		untap = sc.s.untapCostOK(sc.k, pl)
		sc.s.perms[pl].flags &^= pfTapped
		attack = sc.s.canAttack(sc.k, pl)
		return
	}
	t.Run("Shang-Chi out", func(t *testing.T) {
		mana, act, untap, attack := run(t, true, false)
		if !mana || !act || !untap {
			t.Fatalf("mana %v, {T} activation %v, {Q} %v: want all allowed", mana, act, untap)
		}
		if attack {
			t.Fatal("a summoning-sick creature could attack: the exemption is no haste grant")
		}
	})
	for _, c := range []struct {
		name            string
		withShang, mute bool
	}{{"no static", false, false}, {"DebugAsIfHasteOff", true, true}} {
		t.Run(c.name, func(t *testing.T) {
			mana, act, untap, attack := run(t, c.withShang, c.mute)
			if mana || act || untap || attack {
				t.Fatalf("mana %v, {T} activation %v, {Q} %v, attack %v: want none", mana, act, untap, attack)
			}
		})
	}
}

// TestCantBeSuspectedSkipsTheSuspect: a creature a StatCantBeSuspected
// static selects (Airtight Alibi's enchanted creature) does not become
// suspected, while an Optional$ suspect over it is still posed (gorge's
// alterAttributeWouldChange does not read the static; suppressSuspectedEvent
// swallows the event after the ask). One already suspected
// stays suspected, and an un-suspect still clears it (CR 701.60; gorge
// gates the Suspected event only).
func TestCantBeSuspectedSkipsTheSuspect(t *testing.T) {
	tbl, f, ids := newModeTable(
		kop.CardDef{Name: "Test Alibi", Types: kop.TCreature, Power: 0, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantBeSuspected}}}},
		kop.CardDef{Name: "Test Suspect", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{kop.ExtInstr(kop.ExtSuspect)}}}},
	)
	alibi, bear := ids[0], ids[1]
	sweep := kop.ExtInstr(kop.ExtSuspect)
	sweep.Obj, sweep.F = kop.RefNone, f
	setup := func(t *testing.T) (*scen, Slot) {
		t.Helper()
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if sc.k.gen == nil || !sc.k.gen.anyCantSuspect || !sc.k.gen.anySuspect {
			t.Fatal("precondition: the table does not arm the suspect mask and the can't-be-suspected read")
		}
		return sc, sc.slot(sc.perm(bear, 0))
	}
	t.Run("without the static", func(t *testing.T) {
		sc, sl := setup(t)
		if !sc.s.suspectWouldChange(sc.k, &sweep) {
			t.Fatal("an Optional$ suspect over an unsuspected creature was not posed")
		}
		sc.s.suspectOne(sc.k, sl, true)
		if !sc.s.suspected(sc.k, sl) {
			t.Fatal("the creature did not become suspected")
		}
	})
	t.Run("with the static", func(t *testing.T) {
		sc, sl := setup(t)
		sc.perm(alibi, 0)
		if !sc.s.cantSuspect(sc.k, sl) {
			t.Fatal("precondition: the static does not select the creature")
		}
		if !sc.s.suspectWouldChange(sc.k, &sweep) {
			t.Fatal("an Optional$ suspect over a creature that can't become suspected was not posed (gorge asks, then swallows the event)")
		}
		sc.s.suspectOne(sc.k, sl, true)
		if sc.s.suspected(sc.k, sl) {
			t.Fatal("a creature that can't become suspected became suspected")
		}
	})
	t.Run("DebugCantSuspectOff", func(t *testing.T) {
		sc, sl := setup(t)
		sc.perm(alibi, 0)
		save := DebugCantSuspectOff
		DebugCantSuspectOff = true
		defer func() { DebugCantSuspectOff = save }()
		sc.s.suspectOne(sc.k, sl, true)
		if !sc.s.suspected(sc.k, sl) {
			t.Fatal("the mutation did not lift the prohibition")
		}
	})
	t.Run("already suspected", func(t *testing.T) {
		sc, sl := setup(t)
		sc.s.suspectOne(sc.k, sl, true)
		sc.perm(alibi, 0)
		if !sc.s.suspected(sc.k, sl) {
			t.Fatal("the static removed a designation the creature already had")
		}
		sc.s.suspectOne(sc.k, sl, false)
		if sc.s.suspected(sc.k, sl) {
			t.Fatal("an un-suspect did not clear the designation under the static")
		}
	})
}
