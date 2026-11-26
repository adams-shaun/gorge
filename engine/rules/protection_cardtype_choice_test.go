package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestProtectionCardTypeChoiceAskAndGrant pins the Choices$ CardType arm of
// Gains$ Choice (Pippin, Guard of the Citadel: "{T}: Another target creature
// you control gains protection from the card type of your choice until end of
// turn", CR 702.16a): the resolution poses a KChoose over the CR 205.1 card
// types, and the chosen type ("Artifact") resolves into a protection keyword
// whose quality sourceHasQuality reads off the source's types, so an artifact
// source's damage to the bearer is prevented while a non-artifact source's is
// not. Before the ask existed the grant fell to the literal first Choices$
// token, "Protection from CardType", which protects from nothing.
func TestProtectionCardTypeChoiceAskAndGrant(t *testing.T) {
	t.Parallel()
	const fixture = "Name:Choice Spell\nManaCost:0\nTypes:Instant\nOracle:x\n"
	const memnite = "Name:Memnite\nManaCost:0\nTypes:Artifact Creature Soldier\nPT:1/1\nOracle:x\n"
	const artifactSrc = "Name:Artifact Bear\nManaCost:2\nTypes:Artifact Creature Bear\nPT:2/2\nOracle:x\n"
	const plainSrc = "Name:Plain Bear\nManaCost:2\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, _ := putCounterTable(t, 91,
		[]*cards.Card{card(t, fixture), card(t, memnite)},
		[]*cards.Card{card(t, artifactSrc), card(t, plainSrc)})
	mem := moveSeeded(t, e, 0, memnite, state.ZBattlefield)
	art := moveSeeded(t, e, 1, artifactSrc, state.ZBattlefield)
	plain := moveSeeded(t, e, 1, plainSrc, state.ZBattlefield)
	source := moveSeeded(t, e, 0, fixture, state.ZStack)
	e.G.Obj(source).Ability = &cards.SA{Kind: "SP", API: "Protection", Params: map[string]string{
		"Defined": "Self", "Gains": "Choice", "Choices": "CardType",
	}}
	e.G.Obj(source).Source = mem
	e.pending = nil
	if e.protectedFrom(mem, art) || e.protectedFrom(mem, plain) {
		t.Fatalf("precondition: Memnite is protected before the grant (art=%v plain=%v)",
			e.protectedFrom(mem, art), e.protectedFrom(mem, plain))
	}

	var asked *decision.Decision
	e.probe(func() { e.resolveTop() })
	asked = e.Pending()
	if asked == nil {
		t.Fatalf("no protection card-type choice was posed; keywords=%v", e.Keywords(mem))
	}
	const wantOpts = "Artifact Battle Creature Enchantment Instant Kindred Land Planeswalker Sorcery"
	var gotLabels []string
	for _, o := range asked.Options {
		gotLabels = append(gotLabels, o.Label)
		if o.Kind != "quality" {
			t.Errorf("option %q kind = %q, want quality", o.Label, o.Kind)
		}
	}
	if got, want := strings.Join(gotLabels, " "), wantOpts; got != want {
		t.Errorf("choice options = %q, want %q", got, want)
	}
	if got := asked.Prompt; got == "" {
		t.Error("the ask carries no prompt")
	}
	if asked.Kind != decision.KChoose || asked.ResumeKind != "protectionchoice" {
		t.Errorf("posed ask = %s/%s, want KChoose/protectionchoice", asked.Kind, asked.ResumeKind)
	}
	artifactIdx := slices.IndexFunc(asked.Options, func(o decision.Option) bool { return o.Label == "Artifact" })
	if artifactIdx < 0 {
		t.Fatalf("no Artifact option: %+v", asked.Options)
	}
	submitChoices(t, e, artifactIdx)

	if !slices.Contains(e.Keywords(mem), "Protection from Artifact") {
		t.Fatalf("chosen Artifact: keywords=%v, want the card-type protection keyword", e.Keywords(mem))
	}
	if !e.protectedFrom(mem, art) {
		t.Errorf("chosen Artifact: Memnite is not protected from the artifact source")
	}
	if e.protectedFrom(mem, plain) {
		t.Errorf("chosen Artifact: Memnite is protected from the non-artifact source")
	}

	// CR 702.16e: the artifact source's damage to the bearer is prevented; a
	// non-artifact source's is not.
	e.damaging = art
	e.emit(events.Event{Kind: events.Damage, Obj: mem, Amount: 2})
	e.damaging = 0
	if o := e.G.Obj(mem); o.Damage != 0 {
		t.Errorf("artifact source's damage was not prevented: Damage=%d", o.Damage)
	}
	e.damaging = plain
	e.emit(events.Event{Kind: events.Damage, Obj: mem, Amount: 1})
	e.damaging = 0
	if o := e.G.Obj(mem); o.Damage != 1 {
		t.Errorf("non-artifact source's damage was prevented too: Damage=%d", o.Damage)
	}
}
