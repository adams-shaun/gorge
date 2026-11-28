package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestCopyStackValidFamilyAcrossIncarnations: the ability sweep's
// otherAbility exclusion is gorge validStackAdmits' family (same Source
// object, same printed ability), which an older incarnation's trigger still
// belongs to: gorge's ObjID survives a zone change, so after the source is
// flickered its earlier trigger is not copied. The kernel's object index
// survives the same way while its version moves, so the version is not part
// of the family.
func TestCopyStackValidFamilyAcrossIncarnations(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	o := sc.hand(fc, 0)
	s := sc.s
	resolving := stackEntry{kind: seTrigger, obj: o, card: fc, ab: abCascade, ctrl: 0, srcVer: 2, trigObj: NoObj}
	older := resolving
	older.srcVer = 1 // the trigger the source's previous incarnation put on the stack
	if older.srcVer == resolving.srcVer {
		t.Fatal("precondition: the two incarnations' versions must differ")
	}
	sweep := func(below stackEntry) int {
		s.stack[0], s.stack[1], s.sp = below, resolving, 2
		s.res.e, s.res.left = resolving, 1
		return s.copyStackValidFrom(sc.k, true, 0)
	}
	// precondition: an ability of the same controller outside the family is swept
	other := older
	other.ab = abCascade + 1
	if got := sweep(other); got != 0 {
		t.Fatalf("precondition: another ability you control is not swept (got %d, want 0)", got)
	}
	if got := sweep(older); got != -1 {
		t.Fatalf("the older incarnation's trigger was swept (got %d): gorge's otherAbility family excludes it", got)
	}
}
