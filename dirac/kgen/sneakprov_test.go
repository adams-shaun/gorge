package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// ksneakLower lowers a synthetic card body and returns its build-level
// refusals (lowerPool + buildPool, the same basis as the acceptance ratchet).
func ksneakLower(t *testing.T, body string) []refusal {
	t.Helper()
	src := "Name:Sneak Probe\nManaCost:1 U\nTypes:Creature Human\nPT:1/1\nK:Sneak:1 U\n" + body + "\n"
	card, diags := cards.ParseBytes("sneak_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Sneak Probe"}, byName: map[string]*cards.Card{"Sneak Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	return lw.cards[0].Refusals
}

// TestSneakProvRefusals: a sneaked read lowers on the stack (a SpellCast
// self trigger's filter, Count$ValidStack Card.Self+sneaked) and on the
// battlefield (the arena's mechSneaked mark), and refuses
// every other zone with a named reason, so a read can never silently
// lower to a filter that matches nothing.
func TestSneakProvRefusals(t *testing.T) {
	accepted := []string{
		"T:Mode$ SpellCast | ValidCard$ Card.Self+sneaked | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1",
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ Sneaks\nSVar:Sneaks:Count$ValidStack Card.Self+sneaked",
		// the battlefield reads
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self+sneaked | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1",
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1 | ConditionDefined$ TriggeredCard | ConditionPresent$ Card.sneaked",
		"T:Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | CombatDamage$ True | Execute$ ProbeDraw | TriggerZones$ Battlefield\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1 | ConditionDefined$ Self | ConditionPresent$ Card.ThisTurnEntered+sneaked",
	}
	for _, body := range accepted {
		if r := ksneakLower(t, body); len(r) != 0 {
			t.Errorf("accepted shape is refused: %v\n%s", r, body)
		}
	}

	refused := []struct {
		name string
		body string
		want string
	}{
		{
			name: "graveyard read",
			body: "A:AB$ ChangeZone | Cost$ 1 U | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Creature.sneaked+YouOwn | TgtPrompt$ Select target creature card",
			want: "sneaked off the battlefield and the stack",
		},
		{
			name: "other ValidStack",
			body: "T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ Sneaks\nSVar:Sneaks:Count$ValidStack Card.Other+sneaked",
			want: "only ValidStack Card.Self+sneaked is read",
		},
	}
	for _, c := range refused {
		r := ksneakLower(t, c.body)
		if len(r) == 0 {
			t.Errorf("%s: lowered instead of refusing\n%s", c.name, c.body)
			continue
		}
		if !strings.Contains(r[len(r)-1].Reason, c.want) {
			t.Errorf("%s: refused %q, want %q", c.name, r[len(r)-1].Reason, c.want)
		}
	}
}
