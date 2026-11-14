package dirac_test

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
)

// TestPayLifeAltCanon: a pay-life may-play cast's target is the 1-based
// AltCostIndex of gorge's option: 0 (the sole static) is index 1, target 2 is
// index 2, and the canonical forms differ.
func TestPayLifeAltCanon(t *testing.T) {
	s, o := dirac.PayLifeAltScenario(t, 2)
	if !s.FeatMayPayLifeCast(0, o) {
		t.Fatalf("not a pay-life cast: the precondition")
	}
	d := &dirac.Decision{Kind: dirac.DkPriority, Player: 0}
	tr := &canon.Translator{S: s, D: d}
	mp, _ := canon.CastModeCode("mayplay")
	const may = 0x18                                                         // castMayPlay|castFree: the mode a pay-life alternative is offered with
	plain := tr.Action(dirac.MakeAction(dirac.AkCast, dirac.TCode(o), 0, 8)) // the plain cast beside the alternatives: AltCostIndex 0
	if plain.Mode() != mp {
		t.Fatalf("plain mayplay cast mode %#x, want %#x", plain.Mode(), mp)
	}
	c0 := tr.Action(dirac.MakeAction(dirac.AkCast, dirac.TCode(o), 0, may))
	c1 := tr.Action(dirac.MakeAction(dirac.AkCast, dirac.TCode(o), 1, may))
	c2 := tr.Action(dirac.MakeAction(dirac.AkCast, dirac.TCode(o), 2, may))
	if c0.Mode() != mp|1<<4 || c1 != c0 || c2.Mode() != mp|2<<4 || c2 == c0 {
		t.Fatalf("modes target0=%#x target1=%#x target2=%#x, want %#x %#x %#x", c0.Mode(), c1.Mode(), c2.Mode(), mp|1<<4, mp|1<<4, mp|2<<4)
	}
}
