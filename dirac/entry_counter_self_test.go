package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEntryCtrSelfExcluded: CR 614.12, a replacement that modifies how a
// permanent enters comes from the permanent itself only if it affects that
// permanent alone. During the entry fold (genState.entrySelf), an AddCounter
// replacement on the entrant is skipped unless its recipient filter is
// self-only; outside the fold the same replacement applies normally. This is
// the predicate the filtered counter paths (replCtrOnX, ctrGather) and the
// any-kind doublers (dblCtrN) share.
func TestEntryCtrSelfExcluded(t *testing.T) {
	const recv Slot = 3
	general := kop.FilterID(2) // a general subset: "creature you control"
	self := kop.FilterID(1)    // Card.Self
	k := &prep{t: &kop.Table{Filters: []kop.Filter{
		{},
		{Flags: kop.FSelf},
		{Types: kop.TCreature, Ctrl: kop.CtrlYou},
	}}}

	s := &State{}
	s.g.entrySelf = uint8(recv) + 1
	if !s.entryCtrSelfExcluded(k, recv, recv, general) {
		t.Fatal("the entrant's general AddCounter replacement was not excluded from its own entry (CR 614.12)")
	}
	if s.entryCtrSelfExcluded(k, recv, recv, self) {
		t.Fatal("a self-only AddCounter replacement was excluded from its own entry (CR 614.12)")
	}
	if s.entryCtrSelfExcluded(k, recv+1, recv, general) {
		t.Fatal("another permanent's replacement was excluded from the entrant's entry")
	}
	s.g.entrySelf = 0
	if s.entryCtrSelfExcluded(k, recv, recv, general) {
		t.Fatal("an own replacement outside the entry fold was excluded")
	}
}
