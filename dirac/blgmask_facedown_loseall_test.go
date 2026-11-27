package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBlgMaskFaceDownLoseAllTapsForNothing: the CR 305.6 exemption that
// keeps the intrinsic "{T}: Add {G}" on a face-down Forest covers the CR
// 708.2 blank alone. A face-down Forest that also lost all abilities to a
// real layer-6 effect (a Humility-style lose-all) taps for nothing: gorge
// (rules/mana_activation.go) handles the lost case before the face-down
// one, so the kernel must refuse the source too (blgMask 0, manaPCGen
// noManaPC), or the payment taps a source gorge will not pay from (the
// tap-source-swap class).
func TestBlgMaskFaceDownLoseAllTapsForNothing(t *testing.T) {
	set := cardsettest.Set(t, "facedownfamily")
	// Yedora's FaceDownOnly grant is what builds the intrinsic-only planner
	// views (prepLandSet's fdLand) a face-down basic land is read through.
	deck := []cardset.Entry{
		{Name: "Yedora, Grave Gardener", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 16}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	if sc.k.gen == nil || sc.k.gen.setIdx == nil {
		t.Fatal("precondition: setIdx unset: the table mints no face-down land")
	}
	k := sc.k
	sl := sc.slot(sc.perm(id["Forest"], 0))
	if sc.s.typesAt(k, sl)&kop.TLand == 0 || sc.s.genLost(k, sl) {
		t.Fatal("precondition: the face-up Forest is not a land with its abilities")
	}
	if sc.s.blgMask(k, sl)&(1<<kop.G) == 0 {
		t.Fatal("precondition: the face-up Forest carries no Forest intrinsic colour")
	}
	// the CR 708.2 blank alone keeps the intrinsic ability (CR 305.6): the
	// Forest land a FaceDownSetType$ return put onto the battlefield pays
	// from it while nothing but the blank took its abilities
	sc.s.perms[sl].gty |= gtyFaceDown
	if !sc.s.faceDown(k, sl) || !sc.s.genLost(k, sl) {
		t.Fatal("precondition: the Forest is not a lost face-down permanent")
	}
	if sc.s.blgMask(k, sl)&(1<<kop.G) == 0 {
		t.Fatal("the face-down Forest lost the intrinsic {T}: Add {G} its basic subtype grants")
	}
	if sc.s.manaPCGen(k, sl) == &noManaPC {
		t.Fatal("precondition: the face-down Forest is no planner mana source")
	}
	// a real lose-all (the ovr flag genLost reads, as interp.go's one-shot
	// loss sets it) takes the intrinsic ability with the rest
	sc.s.perms[sl].ovr.flags |= ovLoseAll
	if got := sc.s.blgMask(k, sl); got != 0 {
		t.Errorf("a face-down Forest under a lose-all effect still carries mask %05b, want 0", got)
	}
	if got := sc.s.manaPCGen(k, sl); got != &noManaPC {
		t.Errorf("a face-down Forest under a lose-all effect is still a mana source (mask %05b), want noManaPC", got.manaCol)
	}
}
