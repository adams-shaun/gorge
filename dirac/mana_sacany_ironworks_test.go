package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSacAnyTypeFodderKeepsClosedBoundNarrowing: Krark-Clan Ironworks'
// "Sacrifice an artifact: Add {C}{C}" is a kop.SacAnyManaAb source too, but its
// cost sacrifices a permanent TYPE, never dedicated tokens: its fodder can be a
// mana source or the source itself, so the potential bound already prices the
// activations it can make and the closed-bound narrowing must stay (sacDrops).
// A {4} cast the one-admission bound does not pay is not offered even though
// two Ironworks activations (the Scimitar, then the Ironworks itself) would fund
// it. Only the token-fodder shape lifts the narrowing (sacRepeatLive: Jungle
// Patrol's "Sacrifice a token named Wood", mana_sacany_activation_offer_test.go).
func TestSacAnyTypeFodderKeepsClosedBoundNarrowing(t *testing.T) {
	sc, id := synthScen(t, "tail-43", "Krark-Clan Ironworks", "Dancing Scimitar", "Forest", "Firemind Vessel")
	kci := sc.perm(id["Krark-Clan Ironworks"], 0)
	sc.perm(id["Dancing Scimitar"], 0) // the Ironworks' second sacrifice candidate
	sc.perm(id["Forest"], 0)
	vessel := sc.hand(id["Firemind Vessel"], 0) // {4}, no cast target

	// Preconditions: the Ironworks is a live sacrifice-any source whose cost
	// sacrifices a permanent type (not a token), the potential bound is closed,
	// and the board holds the two artifacts two activations would consume.
	ab := sc.k.tapXAbOf(sc.s.cardAt(sc.slot(kci)))
	if !kop.SacAnyManaAb(ab) {
		t.Fatal("Krark-Clan Ironworks' mana ability is not a sacrifice-any one")
	}
	if f := sc.k.t.Filters[ab.Cost.Filter]; f.Flags&kop.FToken != 0 {
		t.Fatalf("the Ironworks' sacrifice filter %v requires a token; this test needs the permanent-type shape", f)
	}
	if closed, open := sc.s.tapXSacAnyBound(sc.k, 0); !closed || open {
		t.Fatalf("the potential bound closed=%v open=%v, want a closed bound", closed, open)
	}
	var cs [MaxPerm]Slot
	if n := sc.s.tapXCands(sc.k, 0, sc.slot(kci), &cs); n < 2 {
		t.Fatalf("the Ironworks has %d sacrifice candidates, want two (it and the Scimitar)", n)
	}
	if mv := sc.k.t.Cards[sc.s.objs[vessel].card].Cost.MV(); mv != 4 {
		t.Fatalf("the cast's cost %d, want {4}", mv)
	}
	cast := MakeAction(AkCast, TCode(vessel), 0, 0)
	if sc.s.sacPotPays(sc.k, 0, cast) {
		t.Fatal("the potential bound already pays the cast; this test needs a play only the lifted search would find")
	}
	// The {4} generic cast is payable by the Forest's {G} and two Ironworks
	// activations ({C}{C}{C}{C}), but the Ironworks sacrifices a permanent type,
	// so the closed bound's narrowing keeps the cast unoffered.
	if d := sc.s.Step(); hasOption(&d, cast) {
		t.Error("the kernel offered the {4} cast off the Ironworks' type fodder: the closed-bound narrowing was lifted")
	}
}
