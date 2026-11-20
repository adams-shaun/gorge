package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStackKwSelfGrantIgnoresBattlefieldCopy pins that an EffectZone$ Stack
// self grant (Bloodbraid Marauder's "this spell has cascade") functions only
// while its source is a spell on the stack. A battlefield Marauder keeps the
// ability on its card, but gorge denies a stack-zone static whose source is off
// the stack, so it grants no cascade to another spell; the spell matcher does
// not read the filter's Self flag, so stackKwN's live-static loop must skip it.
func TestStackKwSelfGrantIgnoresBattlefieldCopy(t *testing.T) {
	set := cardsettest.Set(t, "stackgrant4")
	deck := []cardset.Entry{
		{Name: "Bloodbraid Marauder", Count: 1},
		{Name: "Lightning Bolt", Count: 1},
		{Name: "Island", Count: 8},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var self *kop.Ability
	for i := range tbl.Cards[id["Bloodbraid Marauder"]].Abilities {
		a := &tbl.Cards[id["Bloodbraid Marauder"]].Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatStackKw {
			self = a
		}
	}
	if self == nil || self.Static.Zone != kop.ZoneStack || self.Static.Cond2 == 0 {
		t.Fatalf("precondition: Marauder's gated stack self grant not lowered: %+v", self)
	}
	DebugStackGrantIgnoreGate = true // every gate reads true: the Delirium gate is not under test
	defer func() { DebugStackGrantIgnoreGate = false }()

	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.perm(id["Bloodbraid Marauder"], 0)
	bolt := sc.hand(id["Lightning Bolt"], 0)
	sc.s.objs[bolt].zone = kop.ZoneStack
	sc.s.stack[0] = stackEntry{kind: seSpell, obj: bolt, card: id["Lightning Bolt"], ctrl: 0, prov: uint8(kop.ZoneHand)}
	sc.s.sp = 1
	if n, _ := sc.s.stackKwN(sc.k, kop.StackKwCascade, id["Lightning Bolt"], bolt, 0, true); n != 0 {
		t.Fatalf("a battlefield Marauder granted cascade to another spell (n=%d)", n)
	}
	if sc.s.creditOfferLive(sc.k, 0, id["Lightning Bolt"], bolt, true) {
		t.Fatal("a battlefield Marauder credited another spell's cast")
	}

	// control: the Marauder itself on the stack keeps its own grant
	sc.s.stack[0] = stackEntry{kind: seSpell, obj: bolt, card: id["Lightning Bolt"], ctrl: 0, prov: uint8(kop.ZoneHand)}
	mar := sc.hand(id["Bloodbraid Marauder"], 0)
	sc.s.objs[mar].zone = kop.ZoneStack
	sc.s.stack[1] = stackEntry{kind: seSpell, obj: mar, card: id["Bloodbraid Marauder"], ctrl: 0, prov: uint8(kop.ZoneHand)}
	sc.s.sp = 2
	if n, _ := sc.s.stackKwN(sc.k, kop.StackKwCascade, id["Bloodbraid Marauder"], mar, 0, true); n != 1 {
		t.Fatalf("the Marauder on the stack lost its own cascade (n=%d)", n)
	}
}
