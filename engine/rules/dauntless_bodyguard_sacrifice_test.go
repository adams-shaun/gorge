package rules

// CR 607.2a / 608.2h: an "as this enters, choose another creature" answer is
// linked to the permanent's activated ability, and an ability whose cost
// sacrifices that permanent still reads the choice from the source's
// last-known information. Dauntless Bodyguard's "Sacrifice: the chosen
// creature gains indestructible until end of turn" protects the creature it
// chose even though the sacrifice clears the live object's Chosen list.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func bodyguardFixtureSrc(defined string) string {
	return "Name:Fixture Bodyguard\nManaCost:0\nTypes:Creature Human Knight\nPT:2/1\n" +
		"K:ETBReplacement:Other:PickWard\n" +
		"SVar:PickWard:DB$ ChooseCard | Defined$ You | Choices$ Creature.YouCtrl+Other | Mandatory$ True | " +
		"SpellDescription$ On entry, pick one other creature of yours.\n" +
		"A:AB$ Pump | Cost$ Sac<1/CARDNAME> | Defined$ " + defined + " | KW$ Indestructible | " +
		"SubAbility$ WardClear | SpellDescription$ The picked creature becomes indestructible this turn.\n" +
		"SVar:WardClear:DB$ Cleanup | ClearChosenCard$ True\nOracle:x\n"
}

const (
	bodyguardWardSrc  = "Name:Fixture Ward\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	bodyguardOtherSrc = "Name:Fixture Bystander\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
)

// The card's own spelling (Defined$ Valid Card.ChosenCardStrict, the filter
// read) and the bare Defined$ ChosenCard referent (the resolution read) must
// agree.
func TestDauntlessBodyguardSacrificeProtects(t *testing.T) {
	t.Parallel()
	for _, defined := range []string{"Valid Card.ChosenCardStrict", "ChosenCard"} {
		t.Run(defined, func(t *testing.T) {
			t.Parallel()
			bodyguardSacrificeProtects(t, defined)
		})
	}
}

func bodyguardSacrificeProtects(t *testing.T, defined string) {
	e, cfg, guard := newFixtureDeck(t, 41, bodyguardFixtureSrc(defined), bodyguardWardSrc, bodyguardOtherSrc)
	ward := moveByName(t, e, 0, "Fixture Ward", state.ZBattlefield)
	other := moveByName(t, e, 0, "Fixture Bystander", state.ZBattlefield)
	if o := e.G.Obj(guard); o.Zone != state.ZHand && o.Zone != state.ZLibrary {
		t.Fatalf("precondition: bodyguard zone %v, want hand or library", o.Zone)
	}
	moveByName(t, e, 0, "Fixture Bodyguard", state.ZHand)
	driveToStep(t, e, 1, 0, state.StepMain1)
	submitChoices(t, e, castOptionFor(t, e, guard).Index)
	d := passUntilAsk(t, e)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("entry ask = %+v, want the as-enters creature choice", d)
	}
	pick := -1
	for i, o := range d.Options {
		if o.Obj == guard {
			t.Fatalf("the as-enters choice offers the Bodyguard itself (Other): %+v", d.Options)
		}
		if o.Obj == ward {
			pick = i
		}
	}
	if pick < 0 {
		t.Fatalf("the ward is not among the entry options %+v", d.Options)
	}
	submitChoices(t, e, pick)
	if o := e.G.Obj(guard); o.Zone != state.ZBattlefield || len(o.Chosen) != 1 || o.Chosen[0].Obj != ward {
		t.Fatalf("precondition: bodyguard zone %v chosen %+v, want battlefield and the ward", o.Zone, o.Chosen)
	}
	if e.HasKeyword(ward, "Indestructible") || e.HasKeyword(other, "Indestructible") {
		t.Fatalf("precondition: a creature is indestructible before the activation")
	}
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, guard, 0).Index)
	if o := e.G.Obj(guard); o.Zone != state.ZGraveyard || len(o.Chosen) != 0 {
		t.Fatalf("precondition: bodyguard zone %v chosen %+v, want sacrificed with the live choice cleared", o.Zone, o.Chosen)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("activation did not reach the stack: %v", e.G.Stack)
	}
	passUntilStackEmpty(t, e, 400)
	if !e.HasKeyword(ward, "Indestructible") {
		t.Fatalf("the chosen creature did not gain indestructible after the Bodyguard was sacrificed (CR 608.2h last-known choice)")
	}
	if e.HasKeyword(other, "Indestructible") {
		t.Fatalf("the unchosen creature gained indestructible")
	}
	if diff := diffGames(e.G, replayFromLog(t, cfg, e.L.Events)); diff != "" {
		t.Fatalf("log-only replay differs:\n%s", diff)
	}
}
