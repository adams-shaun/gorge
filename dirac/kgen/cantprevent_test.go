package kgen

import (
	"strings"
	"testing"
)

// kcantpreventForms: "damage can't be prevented" lowers to
// kop.StatCantPreventDamage, the Effect form ("this turn") as an end-of-turn
// emblem's static and the printed blanket form as the card's own static.
var kcantpreventForms = []formCase{
	{card: "Pyrewood Gearhulk", form: "ETB trigger's DB$ Effect CantPreventDamage (EOT emblem)", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Impractical Joke", form: "SP$ Effect CantPreventDamage (EOT emblem)", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
}

func TestCantPreventDamageLowers(t *testing.T) {
	checkForms(t, kcantpreventForms)
	for _, c := range []struct{ name, line, want string }{
		{"printed blanket lowers", "S:Mode$ CantPreventDamage | Description$ x", ""},
		{"printed source-scoped refuses", "S:Mode$ CantPreventDamage | ValidSource$ Card.Self | Description$ x", "ValidSource"},
		{"printed combat-only lowers", "S:Mode$ CantPreventDamage | IsCombat$ True | Description$ x", ""},
		{"printed stack-zone refuses", "S:Mode$ CantPreventDamage | EffectZone$ Stack | Description$ x", "EffectZone"},
		{"effect blanket lowers", "A:AB$ Effect | Cost$ T | StaticAbilities$ STNoPrev | SpellDescription$ x\nSVar:STNoPrev:Mode$ CantPreventDamage | Description$ x", ""},
		{"effect affected refuses", "A:AB$ Effect | Cost$ T | StaticAbilities$ STNoPrev | SpellDescription$ x\nSVar:STNoPrev:Mode$ CantPreventDamage | Affected$ Creature | Description$ x", "Affected"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := kcantselfRefusals(t, c.line)
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
