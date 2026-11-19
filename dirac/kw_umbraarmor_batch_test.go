package dirac

import "testing"

// TestUmbraAllReadsThePreBatchBoard: a destroy-all that destroys an umbra Aura
// and the creature it enchants together still saves the creature when the Aura
// is visited first. The victims run in seat order, so an Aura controlled by the
// lower seat (cast on an opponent's creature, or a creature stolen from the
// Aura's controller) is the order that used to lose the answer: the Aura was
// skipped as an earlier victim and the bearer died with it. CR 702.89a and the
// 6/7/2024 Hyena Umbra ruling: destroying both at once still saves the creature,
// and the Aura is destroyed once.
func TestUmbraAllReadsThePreBatchBoard(t *testing.T) {
	sc, id := umbraScen(t)
	s, k := sc.s, sc.k

	// Aura controlled by seat 0, attached to a creature controlled by seat 1:
	// the Aura is visited first.
	bearer := sc.perm(id["Hill Giant"], 1)
	aura := sc.wear(id["Bear Umbra"], bearer)
	bs, as := sc.slot(bearer), sc.slot(aura)
	if !s.ctrlM[0].has(as) || !s.ctrlM[1].has(bs) {
		t.Fatalf("precondition: Aura controlled by seat 0 %v, bearer by seat 1 %v; want both", s.ctrlM[0].has(as), s.ctrlM[1].has(bs))
	}
	if s.perms[as].attached != bs {
		t.Fatalf("precondition: Aura attached to slot %d, want the bearer %d", s.perms[as].attached, bs)
	}
	if got := s.umbraAura(k, bs, slotMask{}); got != as {
		t.Fatalf("precondition: the bearer's umbra Aura is %d, want %d", got, as)
	}
	if s.perms[bs].damage != 0 {
		t.Fatalf("precondition: bearer has damage %d, want 0", s.perms[bs].damage)
	}
	out := s.umbraAll(k, slotBit(bs).with(as))
	if !out.has(as) {
		t.Fatal("the Aura is not in the destroy-all's victims")
	}
	if out.has(bs) {
		t.Fatal("the bearer was destroyed although its umbra Aura was also a victim")
	}
	if s.perms[bs].damage != 0 || s.perms[bs].flags&pfDeathtouched != 0 {
		t.Fatalf("the saved bearer has damage %d, mark %v; want 0, false", s.perms[bs].damage, s.perms[bs].flags&pfDeathtouched != 0)
	}

	// The control order: the same board with the Aura controlled by the higher
	// seat, so the bearer is visited first. The answer must not depend on it.
	bearer2 := sc.perm(id["Gray Ogre"], 0)
	aura2 := sc.perm(id["Bear Umbra"], 1)
	s.attachTo(k, sc.slot(aura2), sc.slot(bearer2))
	s.refreshTy(k)
	b2, a2 := sc.slot(bearer2), sc.slot(aura2)
	if !s.ctrlM[1].has(a2) || !s.ctrlM[0].has(b2) {
		t.Fatalf("precondition: Aura controlled by seat 1 %v, bearer by seat 0 %v; want both", s.ctrlM[1].has(a2), s.ctrlM[0].has(b2))
	}
	if s.perms[a2].attached != b2 {
		t.Fatalf("precondition: Aura attached to slot %d, want the bearer %d", s.perms[a2].attached, b2)
	}
	if got := s.umbraAura(k, b2, slotMask{}); got != a2 {
		t.Fatalf("precondition: the bearer's umbra Aura is %d, want %d", got, a2)
	}
	out = s.umbraAll(k, slotBit(b2).with(a2))
	if !out.has(a2) || out.has(b2) {
		t.Fatalf("bearer-first order: Aura in victims %v, bearer in victims %v; want true, false", out.has(a2), out.has(b2))
	}
}
