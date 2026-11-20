package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrSpellFeeScen: p0's hand holds a Serra Angel under a live build whose
// payment is that creature spell. The caller adds the board's permanents.
func restrSpellFeeScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "restrspellfee")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, n := range []string{"Forest", "Giada, Font of Hope", "Mossfire Valley", "Rumble Arena", "Serra Angel"} {
		i, ok := set.Lookup(n)
		if !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the restrspellfee table", n)
		}
		id[n] = CardID(set.PoolID(i))
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	o := sc.hand(id["Serra Angel"], 0)
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, card: id["Serra Angel"], ctrl: 0, obj: o}}
	sc.s.ctl = ctlBuild
	return sc, id
}

// restrSpellFeeBoard checks the board both tests reason about: the filter at
// filt charges one, and the restricted source at kar admits the Angel spell
// being built but not an activated ability's payment (CR 106.6).
func restrSpellFeeBoard(t *testing.T, sc *scen, id map[string]CardID, filt, kar Slot) {
	t.Helper()
	s, k := sc.s, sc.k
	d := s.payDescNow(NoObj)
	if d.class != pdSpell || d.card != id["Serra Angel"] {
		t.Fatalf("precondition: the payment is %+v, want the Serra Angel spell", d)
	}
	rab := &k.t.Cards[id["Giada, Font of Hope"]].Abilities[k.gen.restr(id["Giada, Font of Hope"])]
	if !s.restrSrcAdmits(k, 0, kar, rab, &d) {
		t.Fatal("precondition: Giada's {W} does not admit the Angel spell")
	}
	dAct := payDesc{class: pdAct, card: s.cardAt(filt), src: filt}
	if s.restrSrcAdmits(k, 0, kar, rab, &dAct) {
		t.Fatal("precondition: Giada's {W} admits the filter's activation")
	}
	if fee := s.filtFee(k, filt); fee != 1 {
		t.Fatalf("precondition: the filter's fee is %d, want 1", fee)
	}
	if !s.scriptFilt(k, 0).has(filt) {
		t.Fatal("precondition: the filter is not in the mana window's filter list")
	}
	own := s.manaPCGen(k, kar)
	if v, _, _ := s.restrView(k, 0, kar, own, &d); v == nil {
		t.Fatal("precondition: the restricted source has no view for the Angel spell")
	}
}

// TestFaceReachSpellRestrDoesNotPayFilterFee: Giada, Font of Hope's {W} may
// pay an Angel spell, not Rumble Arena's {1} activation fee (CR 106.6). The
// empty pool leaves Rumble's own {C} as the only mana, and the filter's tap
// cannot pay its own fee, so the {R} face is not reachable even though the
// restricted {W} could pay it after a fee it may not fund.
func TestFaceReachSpellRestrDoesNotPayFilterFee(t *testing.T) {
	sc, id := restrSpellFeeScen(t)
	s, k := sc.s, sc.k
	rumble := sc.perm(id["Rumble Arena"], 0)
	kar := sc.slot(sc.perm(id["Giada, Font of Hope"], 0))
	sl := sc.slot(rumble)
	restrSpellFeeBoard(t, sc, id, sl, kar)
	var av avail
	s.availFor(k, 0, &av)
	if av.total != 1 {
		t.Fatalf("precondition: av holds %d units, want Rumble Arena's one {C}", av.total)
	}
	var face kop.ManaCost
	face.Pips[kop.R] = 1
	if av.affords(&face, pipMaskOf(&face), 0) {
		t.Fatal("precondition: the {R} face is payable without the filter")
	}
	if s.faceReachable(k, 0, &av, &face, 0) {
		t.Error("the {R} face is reachable only by charging Giada's spell-only {W} to Rumble Arena's fee (CR 106.6)")
	}
}

// TestFaceReachSpellRestrPaysFaceAfterFilterFee: Mossfire Valley's {1} fee is
// paid by the pool, its {R}{G} makes the generic part, and Giada's untapped
// restricted {W} pays the face's {W} pip: the restricted unit is the spell's,
// so it may pay the face though not the fee (CR 106.6, 601.2g).
func TestFaceReachSpellRestrPaysFaceAfterFilterFee(t *testing.T) {
	sc, id := restrSpellFeeScen(t)
	s, k := sc.s, sc.k
	moss := sc.slot(sc.perm(id["Mossfire Valley"], 0))
	kar := sc.slot(sc.perm(id["Giada, Font of Hope"], 0))
	s.pl[0].pool[kop.C] = 1 // funds the first-only filter's fee for filtFirstMask
	restrSpellFeeBoard(t, sc, id, moss, kar)
	var av avail
	s.availFor(k, 0, &av)
	var face kop.ManaCost
	face.Pips[kop.W] = 1
	if av.affords(&face, pipMaskOf(&face), 2) {
		t.Fatal("precondition: the {W} face plus two generic is payable without the filter")
	}
	if !s.faceReachable(k, 0, &av, &face, 2) {
		t.Error("Mossfire Valley's {R}{G} and Giada's restricted {W}: the face is payable, reported unreachable")
	}
}
