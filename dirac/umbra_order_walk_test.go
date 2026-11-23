package dirac

import "testing"

// TestUmbraWalkVictimAuraKeepsOwnPlaceWhenFirst: a destroy-all that destroys
// an umbra Aura and the creature it enchants together. gorge's victim walk
// (seat order, then battlefield order) destroys the Aura at ITS place when it
// comes before the bearer, and at the bearer's place when the bearer comes
// first (replaceUmbraArmor never moves an Aura twice); an Aura that is not a
// victim always goes at the bearer's place.
func TestUmbraWalkVictimAuraKeepsOwnPlaceWhenFirst(t *testing.T) {
	sc, id := umbraScen(t)
	s, k := sc.s, sc.k

	walk := func(hit slotMask, u *umbraOrder) []Slot {
		var out [MaxPerm]Slot
		n := s.umbraWalk(hit, u, out[:], 0)
		return append([]Slot(nil), out[:n]...)
	}
	check := func(name string, got, want []Slot) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: walk %v, want %v", name, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: walk %v, want %v", name, got, want)
			}
		}
	}

	// Aura first: Aura in seat 0, bearer in seat 1 behind an earlier seat-1
	// victim. The Aura is a victim and comes first, so it stays at its own
	// place, ahead of that victim.
	v1 := sc.perm(id["Gray Ogre"], 1)
	bearer := sc.perm(id["Hill Giant"], 1)
	aura := sc.perm(id["Bear Umbra"], 0)
	s.attachTo(k, sc.slot(aura), sc.slot(bearer))
	s.refreshTy(k)
	vs, bs, as := sc.slot(v1), sc.slot(bearer), sc.slot(aura)
	if s.perms[as].attached != bs || !s.ctrlM[0].has(as) || !s.ctrlM[1].has(bs) || !s.ctrlM[1].has(vs) {
		t.Fatal("precondition: Aura in seat 0 attached to the seat-1 bearer, with a seat-1 victim beside it")
	}
	var u umbraOrder
	u.add(bs, as)
	u.victim = slotBit(as)
	if s.umbraMoves(&u, 0) {
		t.Fatal("an Aura that is a victim and comes before its bearer must not move")
	}
	check("aura first", walk(slotBit(vs).with(as), &u), []Slot{as, vs})

	// Not a victim: the same pair moves to the bearer's place, behind v1.
	u.victim = slotMask{}
	check("aura not a victim", walk(slotBit(vs).with(as), &u), []Slot{vs, as})

	// Bearer first: Aura in seat 1, bearer in seat 0 ahead of another seat-0
	// victim. Victim or not, the Aura takes the bearer's place, ahead of it.
	b2 := sc.perm(id["Typhoid Rats"], 0)
	v2 := sc.perm(id["Grizzly Bears"], 0)
	a2 := sc.perm(id["Snake Umbra"], 1)
	s.attachTo(k, sc.slot(a2), sc.slot(b2))
	s.refreshTy(k)
	b2s, v2s, a2s := sc.slot(b2), sc.slot(v2), sc.slot(a2)
	if s.perms[a2s].attached != b2s || !s.ctrlM[1].has(a2s) || !s.ctrlM[0].has(b2s) || !s.ctrlM[0].has(v2s) {
		t.Fatal("precondition: Aura in seat 1 attached to the seat-0 bearer, with a seat-0 victim behind it")
	}
	var u2 umbraOrder
	u2.add(b2s, a2s)
	u2.victim = slotBit(a2s)
	if !s.umbraMoves(&u2, 0) {
		t.Fatal("an Aura whose bearer comes first must move to the bearer's place")
	}
	check("bearer first, aura a victim", walk(slotBit(v2s).with(a2s), &u2), []Slot{a2s, v2s})
}
