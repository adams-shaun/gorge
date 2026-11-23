package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStackKwCastCameFromExile pins the cast-origin read of a stack keyword
// grant. A cast trigger reads its grant after hideInflightCast has dropped the
// in-flight cast's own push from the cast ring (queueCascade, kw_cascade.go),
// so the origin must come from the spell's stack entry; the ring alone loses
// it. The proposed-cast and hand-origin arms pin the other two answers.
func TestStackKwCastCameFromExile(t *testing.T) {
	set := cardsettest.Set(t, "stackgrant4")
	deck := []cardset.Entry{
		{Name: "Wild-Magic Sorcerer", Count: 1},
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
	if z := tbl.Cards[id["Wild-Magic Sorcerer"]].Abilities[0].Static.Zone; z != kop.ZoneExile {
		t.Fatalf("Wild-Magic Sorcerer's stack grant zone = %d, want exile (%d)", z, kop.ZoneExile)
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	o := sc.hand(id["Lightning Bolt"], 0)
	if !sc.k.gen.anyCastProv || !sc.k.gen.castRingOn {
		t.Fatalf("table does not record cast provenance (anyCastProv=%v castRingOn=%v)", sc.k.gen.anyCastProv, sc.k.gen.castRingOn)
	}
	// the spell is on the stack, cast from exile by seat 0
	sc.s.objs[o].zone = kop.ZoneStack
	sc.s.stack[0] = stackEntry{kind: seSpell, obj: o, card: id["Lightning Bolt"], ctrl: 0, prov: uint8(kop.ZoneExile)}
	sc.s.sp = 1
	sc.s.noteCast(o, 0, kop.ZoneExile)
	// the cast trigger hides the in-flight push before reading its grant
	saved := sc.s.hideInflightCast(&sc.s.stack[0])
	if saved == 0 {
		t.Fatal("precondition: the in-flight push was not hidden")
	}
	if !sc.s.castCameFromExile(o, 0) {
		t.Fatal("a hidden in-flight push lost the stack entry's exile origin")
	}
	sc.s.restoreInflightCast(0, saved)

	// a hand origin answers false even though the restored ring has the exile
	// push: the stack entry is authoritative for a spell already on the stack
	sc.s.stack[0].prov = uint8(kop.ZoneHand)
	if sc.s.castCameFromExile(o, 0) {
		t.Fatal("a hand-origin stack entry answered from exile")
	}

	// a proposed cast is still in exile: the zone shortcut answers true
	sc.s.objs[o].zone = kop.ZoneExile
	if !sc.s.castCameFromExile(o, 0) {
		t.Fatal("a proposed cast in exile did not answer from exile")
	}
}
