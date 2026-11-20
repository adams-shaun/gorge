package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFiltPayFoundPlanDropsSiblingStall pins filtPay's second return on a
// found search: the plan's own spill, never the rule-gap stall a FAILED
// sibling line latched first (filtScript's shape: fs.stall under !found,
// fs.spill under found). The fabricated search is the shape the reviewer's
// board measures (fs.stall and fs.spill differ); the board below exercises
// it end to end.
func TestFiltPayFoundPlanDropsSiblingStall(t *testing.T) {
	fs := filtSearch{stall: StallRestrSrc, census: true}
	if fs.stall == fs.spill {
		t.Fatal("precondition: the fabricated search's failed-line stall and found-plan spill are equal")
	}
	if got := filtPayStall(&fs, true, 0); got != 0 {
		t.Fatalf("a found plan carried a failed sibling's stall: got %s, want none", StallName(got))
	}
	fs.spill = StallTapExtra
	if got := filtPayStall(&fs, true, 0); got != StallTapExtra {
		t.Fatalf("the found plan's own spill was dropped: got %s, want %s", StallName(got), StallName(StallTapExtra))
	}
	if got := filtPayStall(&fs, false, 0); got != StallRestrSrc {
		t.Fatalf("the failed search's latched stall was dropped: got %s, want %s", StallName(got), StallName(StallRestrSrc))
	}
	// The root fork's stall is the failed return's fallback only.
	if got := filtPayStall(&fs, false, StallDynMana); got != StallRestrSrc {
		t.Fatalf("the failed search preferred the walk's stall: got %s, want %s", StallName(got), StallName(StallRestrSrc))
	}
	fs.stall = 0
	if got := filtPayStall(&fs, false, StallDynMana); got != StallDynMana {
		t.Fatalf("the failed search dropped the walk's stall: got %s, want %s", StallName(got), StallName(StallDynMana))
	}
}

// TestFiltPayFoundSpillIsCensusScoped pins the found spill's scope on the
// board the review measured: the full restriction-kind table plus a
// first-only filter and its Island fee. {4}{W} needs the untracked next
// restricted source, whose batch allocation spills into the plain pool and
// latches StallRestrSrc, so the plan is found along a line that spilled.
// The census probe (the unless / ward window's, whose drop sites declare it
// through windowPays) returns that spill; a plain window's probe
// (windowAffords' takeDeferred=false fallback, whose five callers read a
// found probe as final) keeps the pre-found-spill zero.
func TestFiltPayFoundSpillIsCensusScoped(t *testing.T) {
	sc, _ := filtRestrStallScen(t, kop.ColW)
	cost := manaOf(4, 1<<(3*kop.W)) // the untracked source's {W} pays the pip, {C}{C}{C} and {U} the rest
	if sc.s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(sc.s.StallReason()))
	}
	ok, stall := sc.s.filtPay(sc.k, 0, &cost, true)
	if !ok {
		t.Fatal("the census pay probe did not price the {4}{W} its untracked source pays")
	}
	if stall != StallRestrSrc {
		t.Fatalf("the census probe dropped its found plan's spill: got %s, want %s", StallName(stall), StallName(StallRestrSrc))
	}
	ok, stall = sc.s.filtPay(sc.k, 0, &cost, false)
	if !ok {
		t.Fatal("the plain window probe did not price the {4}{W} its untracked source pays")
	}
	if stall != 0 {
		t.Fatalf("the plain window probe returned a found plan's spill (%s); its callers read a found probe as final", StallName(stall))
	}
}
