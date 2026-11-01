package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Tablet of Discovery's {R} is plain and its {R}{R} only pays for an instant or
// sorcery (CR 106.6). The target ask keeps a target that the second one reaches
// (CR 601.2c): two Mountains, City of Brass and the Tablet pay {4}{R}.
func TestTargetAskCountsARestrictedAbilityBesideAPlainOne(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{
		lookup(t, reg, "Suspicious Detonation"), lookup(t, reg, "Tablet of Discovery"), lookup(t, reg, "City of Brass"), lookup(t, reg, "Grizzly Bears"),
	}, nil)
	if e.G.Active != 0 {
		passUntil(t, e, func() bool { return e.G.Active == 0 && e.G.Step == state.StepMain1 })
	}
	spell := moveByName(t, e, 0, "Suspicious Detonation", state.ZHand)
	for _, name := range []string{"Tablet of Discovery", "City of Brass", "Mountain", "Mountain"} {
		e.G.Obj(moveByName(t, e, 0, name, state.ZBattlefield)).SummonSick = false
	}
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	passUntilStackEmpty(t, e, 10) // the Tablet's mill
	_, action := announceAction(t, e, spell)
	submitPaymentPlan(t, e, e.Pending(), action)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after the cast: %+v, zone %v", d, e.G.Obj(spell).Zone)
	}
	for _, o := range d.Options {
		if o.Obj == bears {
			return
		}
	}
	t.Fatalf("Grizzly Bears not offered: %+v", d.Options)
}
