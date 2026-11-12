package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRaiseCostStackSelf: a RaiseCost with EffectZone$ Stack
// (kop.ZoneStack on an IncreaseCost) is live while its source spell sits on
// the stack and charges a cast only when the cast's chosen target is that
// source (ValidTarget$ Spell.Self; gorge costTargetsMatch against the Spell
// on the stack). Kaervek's Torch's shape.
func TestRaiseCostStackSelf(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf, Zone: kop.ZoneStack})
	self := kop.FilterID(len(tbl.Filters) - 1)
	torch, counter := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	var plains CardID
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	if tbl.Cards[plains].Name != "Plains" {
		t.Fatal("precondition: the pool holds Plains")
	}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Torch", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatIncreaseCost,
				Filter2: self, A: 2, Zone: kop.ZoneStack, StFlags: kop.StTargetsFilter2}}}},
		kop.CardDef{Name: "Test Counter", Types: kop.TInstant,
			Cost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 2}}})
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
	if !sc.k.gen.anyRaise {
		t.Fatal("precondition: anyRaise false, want true")
	}
	src := sc.obj(torch, 1)
	sc.s.objs[src].zone = kop.ZoneStack
	sc.s.objs[src].slot = NoSlot
	sc.s.push(stackEntry{kind: seSpell, obj: src, card: torch, ctrl: 1, srcVer: sc.s.objs[src].ver})
	if !sc.s.stackRaiseOnStack(sc.k) {
		t.Fatal("precondition: the stack Torch's raise is not live")
	}

	spell := sc.obj(counter, 0)
	sc.s.ctl = ctlBuild
	defer func() { *sc.s.bld() = build{} }()
	base := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 2}}
	price := func(targets ...Obj) kop.ManaCost {
		t.Helper()
		*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: spell, card: counter, ctrl: 0, srcVer: sc.s.objs[spell].ver}}
		sc.s.bld().e.tgt = noTargets
		if len(targets) > 0 {
			if len(targets) > kop.MaxTargets {
				t.Fatalf("%d targets", len(targets))
			}
			sc.s.bld().e.nt = uint8(len(targets))
			for i, o := range targets {
				sc.s.bld().e.tgt[i] = uint8(o)
			}
		}
		return sc.s.adjustCostGen(sc.k, 0, counter, spell, base)
	}
	if got := price(src); got.Generic != 2 {
		t.Fatalf("the counter targeting the Torch costs %d generic, want 2", got.Generic)
	}
	other := sc.obj(counter, 1)
	if got := price(other); got.Generic != 0 {
		t.Fatalf("the counter targeting another object costs %d generic, want 0", got.Generic)
	}
	if got := price(); got.Generic != 0 {
		t.Fatalf("the counter before its target is chosen costs %d generic, want 0", got.Generic)
	}
	// The source leaving the stack ends the static (gorge's EffectZone$ gate).
	sc.s.sp = 0
	if sc.s.stackRaiseOnStack(sc.k) {
		t.Fatal("the raise is still live after the source left the stack")
	}
	if got := price(src); got.Generic != 0 {
		t.Fatalf("the counter targeting the departed Torch costs %d generic, want 0", got.Generic)
	}
}
