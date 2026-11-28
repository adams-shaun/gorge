package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestNumStoreStackExit: every way a spell leaves the stack for a zone other
// than the battlefield ends the number it stored (CR 400.7; gorge's wasStack
// clear), the exits that bypass the ordinary resolution finish included.
// spellToLib (ExtSpellToLib) is one of those.
func TestNumStoreStackExit(t *testing.T) {
	sc, id := newRemParamsScen(t)
	cp := *sc.k.t
	cp.SVarNames = []string{"X"}
	s := new(State)
	lib := id["Plains"]
	var deck [20]CardID
	for i := range deck {
		deck[i] = lib
	}
	s.Init(&cp, deck[:], deck[:], 1, Config{NoElide: true})
	k := s.k()
	o := s.pl[0].hand[0]
	s.detach(o)
	s.objs[o].zone = kop.ZoneStack
	s.res = resCtx{e: stackEntry{card: lib, ctrl: 0, obj: o, kind: seSpell, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.numStore(o, 0, 2, true, 0)
	if v, ok := s.numRead(o, 0); !ok || v != 2 {
		t.Fatalf("precondition: the spell's stored number = %d,%v, want 2", v, ok)
	}
	s.spellToLib(k)
	if s.objs[o].zone != kop.ZoneLibrary {
		t.Fatalf("spellToLib left the spell in zone %d, want the library", s.objs[o].zone)
	}
	if v, ok := s.numRead(o, 0); ok {
		t.Errorf("the spell shuffled into its library kept its number %d", v)
	}
}
