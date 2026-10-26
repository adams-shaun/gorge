package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 509.1c counts only requirements that can be obeyed without breaking a
// restriction, and CR 506.5 forbids a can't-block-alone creature blocking by
// itself: a lone Mogg Flunkies has no Lure requirement.
func lureAloneDecision(t *testing.T, withPartner bool) (*Engine, *decision.Decision, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	e := threeSeatEngine(t)
	unicorn := onBoardCard(t, e, 1, mustCorpusCardT(t, "Prized Unicorn"))
	flunkies := onBoardCard(t, e, 0, mustCorpusCardT(t, "Mogg Flunkies"))
	var partner state.ObjID
	if withPartner {
		partner = onBoard(t, e, 0, lureBlockerSrc)
	}
	attackSeat0(t, e, unicorn)
	d := askBlockersFresh(t, e)
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("precondition: no blockers decision: %#v", d)
	}
	o := findBlockOption(d, flunkies, unicorn)
	if o == nil || !o.AloneRestricted {
		t.Fatalf("precondition: Flunkies pair missing or not AloneRestricted: %+v", d.Options)
	}
	return e, d, unicorn, flunkies, partner
}

func TestCR509_1cLureLoneAloneRestrictedBlockerHasNoRequirement(t *testing.T) {
	t.Parallel()
	e, d, _, _, _ := lureAloneDecision(t, false)
	if got := d.RequiredQuota(); got != 0 {
		t.Fatalf("RequiredQuota = %d, want 0 (a lone Flunkies can't obey the lure)", got)
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	if err := e.validateBlockers(d, in); err != nil {
		t.Fatalf("empty declaration rejected: %v", err)
	}
	if err := e.validateBlockers(d, newTestBot(7).answer(e, d)); err != nil {
		t.Fatalf("bot answer rejected: %v", err)
	}
}

func TestCR509_1cLureAloneRestrictedBlockerWithPartnerMustBlock(t *testing.T) {
	t.Parallel()
	e, d, unicorn, flunkies, partner := lureAloneDecision(t, true)
	if got := d.RequiredQuota(); got != 2 {
		t.Fatalf("RequiredQuota = %d, want 2", got)
	}
	opt := func(b state.ObjID) int {
		o := findBlockOption(d, b, unicorn)
		if o == nil {
			t.Fatalf("pair (%d,%d) not offered", b, unicorn)
		}
		return o.Index
	}
	if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player}); err == nil {
		t.Fatal("empty declaration accepted despite the lure")
	}
	both := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt(flunkies), opt(partner)}}
	if err := e.validateBlockers(d, both); err != nil {
		t.Fatalf("both on the lure rejected: %v", err)
	}
	if err := e.validateBlockers(d, newTestBot(7).answer(e, d)); err != nil {
		t.Fatalf("bot answer rejected: %v", err)
	}
}
