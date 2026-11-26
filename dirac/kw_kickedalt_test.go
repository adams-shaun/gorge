package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// kickedAltTable is the vanilla table plus a creature with two self-ETB
// triggers, one filtered on PredKickedAlt PredN 1 (ability 0) and one on PredN
// 2 (ability 1), and a plain creature with no such filter.
func kickedAltTable() (t *kop.Table, dual CardID) {
	t = v.NewTable()
	var fids [2]kop.FilterID
	for i := range fids {
		t.Filters = append(t.Filters, kop.Filter{Pred: kop.PredKickedAlt, PredN: int16(i + 1)})
		fids[i] = kop.FilterID(len(t.Filters) - 1)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	ab := func(f kop.FilterID) kop.Ability {
		return kop.Ability{Kind: kop.AbTriggered, Prog: gain,
			Trig: kop.Trigger{Event: kop.TrigETB, Flags: kop.TfSelf, Filter: f}}
	}
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Dual Kicker", Types: kop.TCreature, Power: 1, Toughness: 1,
		Cost: kop.ManaCost{Generic: 1}, Abilities: []kop.Ability{ab(fids[0]), ab(fids[1])}})
	return t, CardID(len(t.Cards) - 1)
}

// kickedAltEnter enters card c's object as the resolving spell (kicked as
// asked) and returns the abilities queued by its entering and the object.
func (sc *scen) kickedAltEnter(c CardID, kicked bool, bits uint8, pre func(Obj)) (queued []uint8, o Obj) {
	s := sc.s
	o = sc.obj(c, 0)
	if pre != nil {
		pre(o)
	}
	s.setKickAlt(o, bits)
	e := stackEntry{kind: seSpell, obj: o, card: c, ctrl: 0}
	if kicked {
		e.flags |= sfKicked
	}
	s.objs[o].zone = kop.ZoneStack
	s.res.active, s.res.e = true, e
	s.npend = 0
	s.enter(sc.k, o, 0, false, kop.ZoneStack)
	s.res.active = false
	for i := 0; i < int(s.npend); i++ {
		queued = append(queued, s.pend[i].ab)
	}
	return queued, o
}

func sameAbs(got []uint8, want ...uint8) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestKickedAltByte: the per-object kicker-alternative byte (coldKickAlt) is
// what a printed `Card.Self+kicked N` self-ETB filter reads. Bit N-1 set
// fires only trigger N; an unkicked cast and a single-cost kicked cast (no
// bit) fire neither (gorge's index forms ignore the bare FlagKicked); a card
// cast again and a reused object id read 0; a table with no PredKickedAlt
// filter allocates no region.
func TestKickedAltByte(t *testing.T) {
	if sc := newScenTable(t, v.NewTable(), Config{NoElide: true}); coldKickAlt.of(sc.s) != nil {
		t.Fatal("a table with no PredKickedAlt filter allocated coldKickAlt")
	}
	tbl, dual := kickedAltTable()
	for _, tc := range []struct {
		name   string
		kicked bool
		bits   uint8
		want   []uint8
	}{
		{"unkicked", false, 0, nil},
		{"single-cost kicked, no alternative bit", true, 0, nil},
		{"first alternative", true, 1, []uint8{0}},
		{"second alternative", true, 2, []uint8{1}},
		{"both alternatives", true, 3, []uint8{0, 1}},
	} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if len(coldKickAlt.of(sc.s)) != MaxObj {
			t.Fatalf("coldKickAlt has %d entries, want MaxObj %d", len(coldKickAlt.of(sc.s)), MaxObj)
		}
		got, o := sc.kickedAltEnter(dual, tc.kicked, tc.bits, nil)
		if !sameAbs(got, tc.want...) {
			t.Errorf("%s: queued abilities %v, want %v", tc.name, got, tc.want)
		}
		if b := coldKickAlt.of(sc.s)[o]; b != tc.bits {
			t.Errorf("%s: byte %d, want %d (the entering permanent keeps it)", tc.name, b, tc.bits)
		}
	}

	// an entry that is not the resolving spell (a reanimated copy of a
	// card that was kicked) is a new object: nothing fires
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.obj(dual, 0)
	sc.s.setKickAlt(o, 3)
	sc.s.npend = 0
	sc.s.enter(sc.k, o, 0, false, kop.ZoneGraveyard)
	if sc.s.npend != 0 || coldKickAlt.of(sc.s)[o] != 0 {
		t.Errorf("an entry that is not the resolving spell queued %d triggers, byte %d, want 0 and 0", sc.s.npend, coldKickAlt.of(sc.s)[o])
	}

	// a card cast again, and a reused object id, end the record
	sc = newScenTable(t, tbl, Config{NoElide: true})
	o = sc.obj(dual, 0)
	sc.s.setKickAlt(o, 3)
	if coldKickAlt.of(sc.s)[o] != 3 {
		t.Fatal("setKickAlt did not store the bits")
	}
	sc.s.beginCast(sc.k, o, 0, 0)
	if b := coldKickAlt.of(sc.s)[o]; b != 0 {
		t.Errorf("byte %d after a new cast began, want 0", b)
	}
	sc.s.setKickAlt(o, 3)
	sc.s.reuseObj(o)
	if b := coldKickAlt.of(sc.s)[o]; b != 0 {
		t.Errorf("byte %d after the id was reused, want 0", b)
	}
	got, _ := sc.kickedAltEnter(dual, true, 0, nil)
	if len(got) != 0 {
		t.Errorf("a cleared record queued %v", got)
	}
}
