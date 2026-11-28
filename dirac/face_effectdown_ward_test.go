package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEffectFaceDownHasNoDisguiseWard: the ward {2} of a face-down permanent
// comes with the disguise cast (CR 702.168b), not with the card: the same
// disguise card an effect returns face down has none (CR 708.2). The mark a
// copy keeps (CR 707.2) carries that answer with it.
func TestEffectFaceDownHasNoDisguiseWard(t *testing.T) {
	set := cardsettest.Set(t, "hosted-cmdr")
	entries := []cardset.Entry{{Name: "Experiment Twelve", Count: 4}, {Name: "Forest", Count: 36}}
	tbl, da, err := set.MatchupN([][]cardset.Entry{entries, entries})
	if err != nil {
		t.Fatal(err)
	}
	var cds [MaxSeats][]CardID
	copy(cds[:], da[:2])
	s := new(State)
	s.InitN(tbl, cds, 2, 1, Config{NoElide: true})
	k := s.k()

	enter := func(cast bool) Slot {
		pl := &s.pl[0]
		for i := pl.libLo; i < pl.libHi; i++ {
			o := pl.lib[i]
			if k.t.Cards[s.objs[o].card].Name != "Experiment Twelve" {
				continue
			}
			from := s.detach(o)
			if cast {
				return s.enterFaceDown(k, o, 0, kop.MorphDisguise, from)
			}
			return s.enterFaceDownMoved(k, o, 0, from)
		}
		t.Fatal("no Experiment Twelve left in the library")
		return NoSlot
	}
	cast, returned := enter(true), enter(false)
	for _, sl := range []Slot{cast, returned} {
		if !s.faceDown(k, sl) || k.t.Cards[s.cardAt(sl)].MorphFamily != kop.MorphDisguise {
			t.Fatalf("precondition: slot %d is not a face-down disguise card", sl)
		}
	}
	if !s.faceDownWard(k, cast) || s.kw(k, cast)&kop.KwWard == 0 {
		t.Fatal("the disguise cast has no ward")
	}
	if s.faceDownWard(k, returned) || s.kw(k, returned)&kop.KwWard != 0 {
		t.Fatal("the permanent an effect put face down has ward")
	}
	for _, sl := range []Slot{cast, returned} {
		gty := s.perms[sl].gty
		if got, want := fdMarkBits(fdMark(gty)), gty&(gtyProvMask|gtyEffectDown); got != want {
			t.Fatalf("slot %d: a copy keeps bits %#x, want %#x", sl, got, want)
		}
		if ty := s.typesAt(k, sl); ty != kop.TCreature {
			t.Fatalf("slot %d: types %#x, want a creature alone (no mark read as a type)", sl, ty)
		}
	}
}
