package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestCastFromGraveyardZone is gorge's ObjectWasCastFromGraveyard is the
// keyword flags OR the latest cast zone. A flashback cast and a may-play cast
// from the graveyard (no flag) both satisfy ExCastFromGrave; a hand cast and
// a copy do not.
func TestCastFromGraveyardZone(t *testing.T) {
	var s State
	fb := stackEntry{kind: seSpell, obj: 0, flags: sfFlashback, prov: uint8(kop.ZoneGraveyard)}
	if !s.castFromGrave(&fb) {
		t.Fatal("a flashback cast from the graveyard read false")
	}
	may := stackEntry{kind: seSpell, obj: 0, prov: uint8(kop.ZoneGraveyard)}
	if !s.castFromGrave(&may) {
		t.Fatal("a may-play cast from the graveyard read false")
	}
	hand := stackEntry{kind: seSpell, obj: 0, prov: uint8(kop.ZoneHand)}
	if s.castFromGrave(&hand) {
		t.Fatal("a hand cast read true")
	}
	s.objs[0].of = ofCopy
	if s.castFromGrave(&may) {
		t.Fatal("a copy read true (CR 707.10)")
	}
	s.objs[0].of = 0

}

// castGraveTrigTable is vanilla's table plus a permanent with "whenever you
// <ev>" (a generated table shape, so prep.gen is set).
func castGraveTrigTable(ev kop.TrigEvent) (*kop.Table, CardID) {
	tb, ids := kbargainTestTable(kop.CardDef{
		Name: "Test Watcher", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: ev, Whose: kop.WhoseYou}}},
	})
	return tb, ids[0]
}

// TestScryZeroNoTrigger is scry 0 and surveil 0 are no scry
// and no surveil (CR 701.22b, 701.25c), so "whenever you scry/surveil"
// does not fire; a scry or surveil of an empty library still does.
func TestScryZeroNoTrigger(t *testing.T) {
	for _, tc := range []struct {
		op kop.Op
		ev kop.TrigEvent
	}{{kop.OpScry, kop.TrigScry}, {kop.OpSurveil, kop.TrigSurveil}} {
		tb, w := castGraveTrigTable(tc.ev)
		sc := newScenTable(t, tb, Config{})
		s, k := sc.s, sc.k
		if !k.gen.anyScryTrig {
			t.Fatal("the watcher's table has no scry/surveil trigger")
		}
		sc.perm(w, 0)
		s.scrySurveilStart(k, tc.op, 0, 0, 0)
		if s.npend != 0 {
			t.Errorf("op %d of 0: %d triggers fired, want none", tc.op, s.npend)
		}
		s.npend = 0
		s.scrySurveilStart(k, tc.op, 0, 0, 1) // one card, empty library
		if s.npend != 1 {
			t.Errorf("op %d of 1 over an empty library: %d triggers fired, want 1", tc.op, s.npend)
		}
		s.npend = 0
	}
}

// TestBargainWithCreatureToken is gorge's Bargain spec is
// "Artifact,Enchantment,Card.token", so a creature token can pay bargain.
func TestBargainWithCreatureToken(t *testing.T) {
	tb, ids := kbargainTestTable(kop.CardDef{
		Name: "Test Bargain", Cost: kop.ManaCost{Generic: 1}, Types: kop.TSorcery, Bargain: true,
	})
	bargain := ids[0]
	sc := newScenTable(t, tb, Config{})
	s, k := sc.s, sc.k
	bears := sc.perm(v.GrizzlyBears, 0)
	if s.bargainOffer(k, 0, bargain) {
		t.Fatal("bargainOffer with only a nontoken creature")
	}
	s.objs[bears].of |= ofToken
	if !s.bargainOffer(k, 0, bargain) {
		t.Fatal("bargainOffer with a creature token: want a candidate")
	}
	var d Decision
	s.bargainCands(k, 0, &d)
	if d.N != 1 || Obj(d.Options[0].Source()) != bears {
		t.Fatalf("bargainCands: %d options, want the token %d", d.N, bears)
	}
}

// TestCastgraveCopyReadsFalse: gorge's state.ObjectWasCastFromGraveyard
// excludes a stack copy (CR 707.10), and the kernel's copy path both clears
// sfFlashback (copy_gen.go mintSpellCopy) and marks the copied token ofCopy.
// Either test is enough; this pins both without needing a copy spell in the
// oracle pool.
func TestCastgraveCopyReadsFalse(t *testing.T) {
	var s State
	spell := stackEntry{kind: seSpell, obj: 0, flags: sfFlashback}
	if !s.castFromGrave(&spell) {
		t.Fatal("a flashback spell read false")
	}
	s.objs[0].of = ofCopy
	if s.castFromGrave(&spell) {
		t.Fatal("a copied spell read true (CR 707.10)")
	}
	s.objs[0].of = 0
	spell.flags &^= sfFlashback
	if s.castFromGrave(&spell) {
		t.Fatal("a spell with no graveyard-origin flag read true")
	}
	if s.castFromGrave(nil) {
		t.Fatal("a nil entry read true")
	}
	if s.castFromGrave(&stackEntry{kind: seTrigger, obj: 0, flags: sfFlashback}) {
		t.Fatal("a triggered ability read true")
	}
}
