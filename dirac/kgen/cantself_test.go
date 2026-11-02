package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kcantselfForms: the card's own "you can't cast this spell" static
// ValidCard$ Card.Self | EffectZone$ All lowers with no
// filter and Zone Command; Caster$ Player.Active is the StCasterActive flag.
var kcantselfForms = []formCase{
	{card: "Jace Reawakened", form: "CantBeCast Card.Self | EffectZone$ All | Caster$ Player.Active | CheckSVar$ Count$YourTurns", lir: "static CantBeCast"},
	{card: "Proft, Sinister Mastermind", form: "CantBeCast Card.Self | EffectZone$ All | CheckSVar$ X (graveyard count)", lir: "static CantBeCast"},
}

func kcantselfRefusals(t *testing.T, static string) string {
	t.Helper()
	src := strings.Join([]string{"Name:Probe", "ManaCost:1 U", "Types:Creature Bear", "PT:2/2", static}, "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, r := range lowerPool(p).cards[0].Refusals {
		out = append(out, r.Reason)
	}
	return strings.Join(out, "; ")
}

func TestCantselfLowers(t *testing.T) {
	checkForms(t, kcantselfForms)
	for _, c := range []struct{ name, static, want string }{
		{"self without EffectZone", "S:Mode$ CantBeCast | ValidCard$ Card.Self | Description$ x", "CantBeCast Card.Self without EffectZone$ All"},
		{"active caster on a non-self filter", "S:Mode$ CantBeCast | ValidCard$ Creature | Caster$ Player.Active | Description$ x", `CantBeCast Caster "Player.Active"`},
		{"self with an unread key", "S:Mode$ CantBeCast | ValidCard$ Card.Self | EffectZone$ All | NumLimitEachTurn$ 1 | Description$ x", "NumLimitEachTurn"},
		{"self with the active caster lowers", "S:Mode$ CantBeCast | ValidCard$ Card.Self | EffectZone$ All | Caster$ Player.Active | Description$ x", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := kcantselfRefusals(t, c.static)
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
