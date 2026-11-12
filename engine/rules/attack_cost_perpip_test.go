// Per-pip CR 107.4f payment of a combat tax: each Phyrexian pip of a
// declaration is paid with its colour or two life independently, so a payer
// with one white and 3 life can pay {W/P}{W/P} as W + 2 life, and a payer with
// both faces available is asked once per pip.
package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// phyPayAsk returns the pending payment decision and asserts it is the
// Phyrexian colour-versus-life ask.
func phyPayAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the Phyrexian payment ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "attack_phy_colour" {
			return d
		}
	}
	t.Fatalf("pending KChoose is not the Phyrexian ask: %+v", d.Options)
	return nil
}

// phyPayOption returns the option of the given kind from the pending ask.
func phyPayOption(t *testing.T, d *decision.Decision, kind string) decision.Option {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == kind {
			return o
		}
	}
	t.Fatalf("payment ask has no %s option: %+v", kind, d.Options)
	return decision.Option{}
}

// TestNornsAnnexMixedPhyrexianPayment: two attackers against Norn's Annex with
// one floating white and 3 life. The legal payment is one pip with W and the
// other with 2 life; the window must ask for the first pip, accept the colour
// face, route the remaining pip to life without a second ask, and commit both
// attackers.
func TestNornsAnnexMixedPhyrexianPayment(t *testing.T) {
	t.Parallel()
	e, b1 := attackTaxSeat(t, "Norn's Annex")
	b2 := onBoardReady(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n")
	e.G.Players[1].Life = 3
	e.G.Players[1].Pool[state.MW] = 1
	chosen := []decision.Option{{Obj: b1, Player: 0}, {Obj: b2, Player: 0}}
	ch := e.attackCharge(chosen)

	// PRECONDITION: both bears are on the battlefield under the payer, the
	// charge really carries two W Phyrexian pips, and the pool is one W with
	// exactly 3 life -- so only a MIXED payment is legal.
	for _, id := range []state.ObjID{b1, b2} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
			t.Fatalf("precondition: attacker %d not on seat 1's battlefield: %+v", id, e.G.Obj(id))
		}
	}
	if len(ch.phyrexian) != 2 || ch.mana != 0 || ch.life != 0 {
		t.Fatalf("precondition: attackCharge = %+v, want two W Phyrexian pips", ch)
	}
	if e.G.Players[1].Pool[state.MW] != 1 || e.G.Players[1].Life != 3 {
		t.Fatalf("precondition: pool W=%d life=%d, want 1 and 3", e.G.Players[1].Pool[state.MW], e.G.Players[1].Life)
	}
	attackers := map[state.ObjID]bool{b1: true, b2: true}
	if !e.combatChargeAffordable(1, ch, attackers, attackers) {
		t.Fatal("precondition: the mixed W + 2 life payment must be affordable")
	}

	e.askAttackers()
	d := e.Pending()
	opt1 := findAttackOption(d, b1, 0)
	opt2 := findAttackOption(d, b2, 0)
	if opt1 == nil || opt2 == nil {
		t.Fatalf("mixed-payable declaration not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt1.Index, opt2.Index}}); err != nil {
		t.Fatalf("submit the mixed-payable declaration: %v", err)
	}

	pay := phyPayAsk(t, e)
	colour := phyPayOption(t, pay, "attack_phy_colour")
	if !strings.Contains(colour.Label, "{W/P}") || !strings.Contains(colour.Label, "(1 of 2)") {
		t.Fatalf("first-pip colour option does not name the pip: %+v", colour)
	}
	before := e.G.Players[1].Life
	if err := e.Submit(decision.Intent{Seq: pay.Seq, Player: pay.Player, Choices: []int{colour.Index}}); err != nil {
		t.Fatalf("answer the first pip with colour: %v", err)
	}

	// The second pip has exactly one feasible face (life), so no second ask is
	// owed: the window must have routed it and closed.
	if e.attackPay != nil {
		t.Fatalf("a second Phyrexian ask is still owed: %+v", e.Pending())
	}
	if got := e.G.Players[1].Life; got != before-2 {
		t.Fatalf("life after the mixed payment = %d, want %d (one pip with colour, one with life)", got, before-2)
	}
	if got := e.G.Players[1].Pool[state.MW]; got != 0 {
		t.Fatalf("pool W after the mixed payment = %d, want 0", got)
	}
	if o := e.G.Obj(b1); o == nil || !o.IsAttacking {
		t.Fatal("the first mixed-charged attacker was never declared")
	}
	if o := e.G.Obj(b2); o == nil || !o.IsAttacking {
		t.Fatal("the second mixed-charged attacker was never declared")
	}
	drainCombatPriority(t, e)
}

// TestNornsAnnexPerPipElection: two attackers against Norn's Annex with two
// Plains and 20 life. Every pip has both faces, so the window must ask once
// per pip -- life for the first, colour for the second -- and spend exactly
// one Plains.
func TestNornsAnnexPerPipElection(t *testing.T) {
	t.Parallel()
	e, b1 := attackTaxSeat(t, "Norn's Annex")
	b2 := onBoardReady(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n")
	p1 := onBoardCard(t, e, 1, card(t, "Name:Test Plains\nTypes:Basic Land Plains\nOracle:x\n"))
	p2 := onBoardCard(t, e, 1, card(t, "Name:Test Plains\nTypes:Basic Land Plains\nOracle:x\n"))
	e.G.Players[1].Life = 20
	chosen := []decision.Option{{Obj: b1, Player: 0}, {Obj: b2, Player: 0}}
	ch := e.attackCharge(chosen)

	// PRECONDITION: two W Phyrexian pips, two untapped white sources, and
	// enough life for either face of either pip.
	if len(ch.phyrexian) != 2 || ch.mana != 0 || ch.life != 0 {
		t.Fatalf("precondition: attackCharge = %+v, want two W Phyrexian pips", ch)
	}
	for _, id := range []state.ObjID{p1, p2} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Plains %d is not an untapped battlefield land: %+v", id, e.G.Obj(id))
		}
	}
	if e.G.Players[1].Life != 20 {
		t.Fatalf("precondition: payer life = %d, want 20", e.G.Players[1].Life)
	}
	attackers := map[state.ObjID]bool{b1: true, b2: true}
	if !e.combatChargeAffordable(1, ch, attackers, attackers) {
		t.Fatal("precondition: two Plains plus 20 life must afford the declaration")
	}

	e.askAttackers()
	d := e.Pending()
	opt1 := findAttackOption(d, b1, 0)
	opt2 := findAttackOption(d, b2, 0)
	if opt1 == nil || opt2 == nil {
		t.Fatalf("Phyrexian-charged declaration not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt1.Index, opt2.Index}}); err != nil {
		t.Fatalf("submit the Phyrexian-charged declaration: %v", err)
	}

	first := phyPayAsk(t, e)
	life := phyPayOption(t, first, "attack_phy_life")
	if !strings.Contains(life.Label, "(1 of 2)") {
		t.Fatalf("first-pip life option does not name the pip: %+v", life)
	}
	if err := e.Submit(decision.Intent{Seq: first.Seq, Player: first.Player, Choices: []int{life.Index}}); err != nil {
		t.Fatalf("answer the first pip with life: %v", err)
	}

	second := phyPayAsk(t, e)
	colour := phyPayOption(t, second, "attack_phy_colour")
	if !strings.Contains(colour.Label, "(2 of 2)") {
		t.Fatalf("second-pip colour option does not name the pip: %+v", colour)
	}
	if err := e.Submit(decision.Intent{Seq: second.Seq, Player: second.Player, Choices: []int{colour.Index}}); err != nil {
		t.Fatalf("answer the second pip with colour: %v", err)
	}

	// The colour face needs one real white mana: the window now asks for the
	// source, and tapping exactly one Plains completes the payment.
	mana := e.Pending()
	if mana == nil || mana.Kind != decision.KChoose {
		t.Fatalf("expected the mana-source ask after the colour pip, got %+v", mana)
	}
	manaIdx := -1
	for _, o := range mana.Options {
		if o.Kind == "attack_mana" {
			manaIdx = o.Index
			break
		}
	}
	if manaIdx < 0 {
		t.Fatalf("mana-source ask has no attack_mana option: %+v", mana.Options)
	}
	if err := e.Submit(decision.Intent{Seq: mana.Seq, Player: mana.Player, Choices: []int{manaIdx}}); err != nil {
		t.Fatalf("tap a Plains for the colour pip: %v", err)
	}

	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("life after one life-routed pip = %d, want 18", got)
	}
	tapped := 0
	for _, id := range []state.ObjID{p1, p2} {
		if e.G.Obj(id).Tapped {
			tapped++
		}
	}
	if tapped != 1 {
		t.Fatalf("%d Plains tapped, want exactly 1 (the second pip's colour face)", tapped)
	}
	if o := e.G.Obj(b1); o == nil || !o.IsAttacking {
		t.Fatal("the first per-pip-charged attacker was never declared")
	}
	if o := e.G.Obj(b2); o == nil || !o.IsAttacking {
		t.Fatal("the second per-pip-charged attacker was never declared")
	}
	drainCombatPriority(t, e)
}
