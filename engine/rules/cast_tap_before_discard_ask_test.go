package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTapCostPaidBeforeDiscardReplacementAsk pins the order of an activated
// ability's non-mana cost parts: the {T} part is paid at the head of the
// payment, before the discard (CR 602.2b lets the payer choose, but dirac's
// payAbilityCosts taps before payPicks, and gorge leads the lockstep). Rest in
// Peace turns the discard's graveyard move into a replacement ask, so the Tap
// event must precede that ask. Before the fix the Tap event followed it, and a
// Fomori Vault activation showed the Vault untapped when the ask was served.
func TestTapCostPaidBeforeDiscardReplacementAsk(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	vault := lookup(t, reg, "Fomori Vault")
	rip := lookup(t, reg, "Rest in Peace")
	fodder := lookup(t, reg, "Grizzly Bears")
	// Two Rest in Peace permanents: with two applicable replacements the
	// engine poses the CR 616.1 order ask, the observable moment between the
	// cost parts.
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{vault, rip, rip, fodder, fodder}, nil)
	vaultID := moveByName(t, e, 0, "Fomori Vault", state.ZBattlefield)
	moveByName(t, e, 0, "Rest in Peace", state.ZBattlefield)
	moveByName(t, e, 0, "Rest in Peace", state.ZBattlefield)
	addMana(t, e, 0, "CCC")

	e.askPriority(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("precondition: no priority for the Vault's controller: %+v", d)
	}
	activate := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == vaultID {
			activate = o.Index
			break
		}
	}
	if activate < 0 {
		t.Fatalf("precondition: Fomori Vault's {3},{T},discard ability was not offered: %+v", d.Options)
	}
	if o := e.G.Obj(vaultID); o == nil || o.Tapped {
		t.Fatalf("precondition: the Vault is not untapped before the activation: %+v", o)
	}
	submitChoices(t, e, activate)

	// The discard cost's card choice, then the replacement order ask.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 {
		t.Fatalf("precondition: no discard choice for the cost: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)

	d = e.Pending()
	if d == nil || d.Kind != decision.KReplacement || len(d.Options) != 2 || !strings.Contains(d.Prompt, "moves") {
		t.Fatalf("precondition: the discard's Rest in Peace order ask was not posed: %+v", d)
	}
	askSeq, tapSeq := uint64(0), uint64(0)
	for _, ev := range e.L.Events {
		switch {
		case ev.Kind == events.DecisionAsk && ev.Seq == d.Seq:
			askSeq = ev.Seq
		case ev.Kind == events.Tap && ev.Obj == vaultID:
			tapSeq = ev.Seq
		}
	}
	if tapSeq == 0 {
		t.Fatal("precondition: the Vault's {T} cost produced no Tap event")
	}
	if tapSeq > askSeq {
		t.Fatalf("the replacement ask between cost parts (seq %d) preceded the Vault's Tap event (seq %d): the {T} cost was not paid before the discard (CR 602.2b)", askSeq, tapSeq)
	}
}
