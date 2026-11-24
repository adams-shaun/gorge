package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmBurntheAccursed reproduces the SUP-s186-supp/s189-supp p1.graveyard
// divergence: Burn the Accursed's 5 damage is marked (DamageMap$ True) and
// flushed later by DB$ DamageResolve, but its Target form of
// ReplaceDyingDefined$ must register the exile-instead replacement over the
// marked recipient too (CR 614.1a, 614.4). At the pin the mark branch returns
// without recording the recipient in the dying slice, so the replacement never
// exists and the lethal target goes to the graveyard instead of exile.
func TestFarmBurntheAccursed(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	burn := mustCorpusCard(t, reg, "Burn the Accursed")
	hound := mustCorpusCard(t, reg, "Blood Hound")
	e, cfg := censusEngine(t, 18611, []*cards.Card{burn}, []*cards.Card{hound})
	cardToHand(t, e, burn)
	houndID := moveOwnerCard(t, e, 1, hound, state.ZBattlefield)
	if o := e.G.Obj(houndID); o.Zone != state.ZBattlefield || e.Toughness(houndID) != 1 {
		t.Fatalf("precondition: target Blood Hound is zone %s with toughness %d, want battlefield 1/1", o.Zone, e.Toughness(houndID))
	}
	addMana(t, e, 0, "RRRRR")
	castFirst(t, e, "cast")
	targetObject(t, e, houndID)
	for n := 0; n < 40 && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the stack (stack depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KTriggerOptional {
			declineOptional(t, e)
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected %s decision while draining the stack: %+v", d.Kind, d)
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	if got := e.G.Obj(houndID).Zone; got != state.ZExile {
		t.Fatalf("Burn the Accursed's lethal target should be exiled (CR 614.1a, 614.4; Forge and XMage both exile it), got %s", got)
	}
	replayCheck(t, e, cfg)
}
