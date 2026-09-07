package view

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// ---------------------------------------------------------------------------
// Defect 1: Forge's CARDNAME/NICKNAME self-reference placeholders.
// ---------------------------------------------------------------------------

// forkedBoltSrc mirrors the user's screenshot card: a spell whose
// SpellDescription$ says "CARDNAME deals 2 damage ...". The placeholder must
// render as the card's own name.
const forkedBoltSrc = `Name:Forked Bolt
ManaCost:R
Types:Instant
A:SP$ DealDamage | ValidTgts$ Any | TgtPrompt$ Select any target | NumDmg$ 2 | SpellDescription$ CARDNAME deals 2 damage divided as you choose among one or two targets.
Oracle:x
`

// ambergrisSrc uses NICKNAME in a trigger's TriggerDescription$, with the
// legend-comma name shape Forge's nickname default is built for.
const ambergrisSrc = `Name:Ambergris, Agent of Destruction
ManaCost:1
Types:Creature
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigGain | TriggerDescription$ Whenever NICKNAME attacks, do a thing.
SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`

// TestSubstitutePlaceholders locks the word-boundary, punctuation and
// nickname semantics of the placeholder substitution itself, independent of
// any stack projection.
func TestSubstitutePlaceholders(t *testing.T) {
	cases := []struct {
		name string
		text string
		nm   string
		want string
	}{
		{"cardname full name", "CARDNAME deals 2 damage to any target.", "Forked Bolt", "Forked Bolt deals 2 damage to any target."},
		{"nickname simple", "If NICKNAME is a Scout, do a thing.", "Forked Bolt", "If Forked is a Scout, do a thing."},
		{"nickname legend comma", "Whenever NICKNAME attacks, do a thing.", "Ambergris, Agent of Destruction", "Whenever Ambergris attacks, do a thing."},
		{"possessive", "CARDNAME's power", "Dreadmaw", "Dreadmaw's power"},
		{"punctuation", "Target: CARDNAME.", "Forked Bolt", "Target: Forked Bolt."},
		{"word boundary prefix", "XCARDNAME", "Forked Bolt", "XCARDNAME"},
		{"word boundary suffix", "CARDNAMES", "Forked Bolt", "CARDNAMES"},
		{"no placeholder", "Bolt deals 3 damage.", "Forked Bolt", "Bolt deals 3 damage."},
		{"both placeholders", "CARDNAME gets +1/+1 and NICKNAME gains menace.", "Forked Bolt", "Forked Bolt gets +1/+1 and Forked gains menace."},
		{"empty text", "", "Forked Bolt", ""},
		{"empty name", "CARDNAME cannot be targeted.", "", "CARDNAME cannot be targeted."},
	}
	for _, tc := range cases {
		if got := substitutePlaceholders(tc.text, tc.nm); got != tc.want {
			t.Errorf("%s: substitutePlaceholders(%q, %q) = %q, want %q", tc.name, tc.text, tc.nm, got, tc.want)
		}
	}
}

// TestStackViewSpellTextSubstitutesCardName is the defect-1 end-to-end path
// for a SPELL: the stack entry's Text must render "Forked Bolt deals ..."
// with the placeholder gone.
func TestStackViewSpellTextSubstitutesCardName(t *testing.T) {
	g, id := twoSeatWith(t, forkedBoltSrc)
	events.Apply(g, events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Stack) != 1 {
		t.Fatalf("stack %+v", v.Stack)
	}
	sv := v.Stack[0]
	const want = "Forked Bolt deals 2 damage divided as you choose among one or two targets."
	if sv.Text != want {
		t.Fatalf("Text = %q, want %q (CARDNAME substituted)", sv.Text, want)
	}
	if strings.Contains(sv.Text, "CARDNAME") || strings.Contains(sv.Text, "NICKNAME") {
		t.Fatalf("Text %q still carries a placeholder", sv.Text)
	}
	if sv.Name != "Forked Bolt" {
		t.Fatalf("Name = %q, want the card's face name", sv.Name)
	}
}

// TestStackViewAbilityTextSubstitutesNickname is the defect-1 path for an
// ABILITY: the substituted name is the SOURCE's face name (abilityName), and
// the trigger line's NICKNAME renders as the first word of that name.
func TestStackViewAbilityTextSubstitutesNickname(t *testing.T) {
	g, id := twoSeatWith(t, ambergrisSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	events.Apply(g, events.Event{Kind: events.TriggerPush, Player: 0, Obj: id, Amount: 0})
	v := Project(g, flatChars{g}, 1, nil)
	if len(v.Stack) != 1 || v.Stack[0].Kind != "trigger" {
		t.Fatalf("stack %+v", v.Stack)
	}
	sv := v.Stack[0]
	const want = "Whenever Ambergris attacks, do a thing."
	if sv.Text != want {
		t.Fatalf("Text = %q, want %q (NICKNAME substituted)", sv.Text, want)
	}
	if strings.Contains(sv.Text, "CARDNAME") || strings.Contains(sv.Text, "NICKNAME") {
		t.Fatalf("Text %q still carries a placeholder", sv.Text)
	}
	if sv.Name != "Ambergris, Agent of Destruction" {
		t.Fatalf("Name = %q, want the source's face name", sv.Name)
	}
}

// ---------------------------------------------------------------------------
// Defect 3: stack entries for abilities/triggers carry no artwork.
// ---------------------------------------------------------------------------

// TestStackViewAbilityCardFilledFromSource pins the defect-3 fix: an ability
// object (here a non-trigger shape) on the stack gets its Card from the
// SOURCE permanent's cardView, without changing Kind.
func TestStackViewAbilityCardFilledFromSource(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	ab := g.AddObject(nil, 0)
	events.Move(g, ab.ID, state.ZLibrary, state.ZStack)
	ab.Ability = &cards.SA{Kind: "AB", API: "GainLife", Params: map[string]string{"SpellDescription": "Gain 1 life."}}
	ab.Source = id
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Stack) != 1 {
		t.Fatalf("stack %+v", v.Stack)
	}
	sv := v.Stack[0]
	if sv.Kind != "ability" {
		t.Fatalf("Kind = %q, want \"ability\" (the Kind must not change)", sv.Kind)
	}
	if sv.Card == nil || sv.Card.Name != "Watcher" || sv.Card.ID != id {
		t.Fatalf("Card = %+v, want the source permanent's CardView", sv.Card)
	}
}

// TestStackViewAbilityCardNilWhenSourceHidden pins the redaction half of the
// fix: when the source has left the battlefield for a hidden zone (its
// owner's hand), the stack entry must not leak its card via CardView.
func TestStackViewAbilityCardNilWhenSourceHidden(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZHand})
	ab := g.AddObject(nil, 0)
	events.Move(g, ab.ID, state.ZLibrary, state.ZStack)
	ab.Ability = &cards.SA{Kind: "AB", API: "GainLife", Params: map[string]string{"SpellDescription": "Gain 1 life."}}
	ab.Source = id
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Stack) != 1 {
		t.Fatalf("stack %+v", v.Stack)
	}
	if v.Stack[0].Card != nil {
		t.Fatalf("Card = %+v, want nil for a source in a hidden zone (redaction)", v.Stack[0].Card)
	}
}

// TestStackViewAbilityCardNilWhenSourceAbsent pins the "GONE" half of the
// fix: when the source object cannot be resolved, no Card is fabricated.
func TestStackViewAbilityCardNilWhenSourceAbsent(t *testing.T) {
	g, _ := twoSeatWith(t, watcherSrc)
	ab := g.AddObject(nil, 0)
	events.Move(g, ab.ID, state.ZLibrary, state.ZStack)
	ab.Ability = &cards.SA{Kind: "AB", API: "GainLife", Params: map[string]string{"SpellDescription": "Gain 1 life."}}
	ab.Source = state.ObjID(9999)
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Stack) != 1 {
		t.Fatalf("stack %+v", v.Stack)
	}
	if v.Stack[0].Card != nil {
		t.Fatalf("Card = %+v, want nil for an unresolvable source", v.Stack[0].Card)
	}
}
