package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestReplDmgTargetPlayerSpec: ValidTarget$ You is the replacement's
// controller alone in gorge (MatchesPlayerSpecFrom), a read the kernel's
// any-player ReplDmgPlayer bit does not make: the set and minus rewrites
// refuse it rather than lower a line that applies to the wrong players. The
// controls are the same lines with a ValidTarget$ the kernel does read.
func TestReplDmgTargetPlayerSpec(t *testing.T) {
	for _, c := range []struct {
		name   string
		params map[string]string
		with   map[string]string
		want   string // "" lowers clean
	}{
		{"set You", map[string]string{"ValidTarget": "You", "DamageAmount": "GE4"},
			map[string]string{"VarName": "DamageAmount", "VarValue": "3"}, "DamageDone set ValidTarget"},
		{"set Player", map[string]string{"ValidTarget": "Player", "DamageAmount": "GE4"},
			map[string]string{"VarName": "DamageAmount", "VarValue": "3"}, ""},
		{"minus You", map[string]string{"ValidTarget": "You"},
			map[string]string{"VarName": "DamageAmount", "VarValue": "ReplaceCount$DamageAmount/Minus.1"}, "DamageDone minus ValidTarget"},
		{"minus Player", map[string]string{"ValidTarget": "Player"},
			map[string]string{"VarName": "DamageAmount", "VarValue": "ReplaceCount$DamageAmount/Minus.1"}, ""},
		{"minus creature filter", map[string]string{"ValidTarget": "Creature.YouCtrl"},
			map[string]string{"VarName": "DamageAmount", "VarValue": "ReplaceCount$DamageAmount/Minus.1"}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			params := map[string]string{"Event": "DamageDone", "ActiveZones": "Battlefield"}
			for k, v := range c.params {
				params[k] = v
			}
			rl := cards.Repl{Event: "DamageDone", Params: params,
				With: &cards.SA{API: "ReplaceEffect", Params: c.with}}
			face := &cards.Face{Name: "Synthetic", ManaCost: "1 W", Types: []string{"Enchantment"}, Repls: []cards.Repl{rl}}
			card := &cards.Card{Faces: []*cards.Face{face}}
			p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
			lc := lowerCard(p, "Synthetic", card, false)
			if c.want == "" {
				for _, r := range lc.Refusals {
					t.Errorf("refused: %s", r.Reason)
				}
				return
			}
			if len(lc.Refusals) == 0 || !strings.Contains(lc.Refusals[0].Reason, c.want) {
				t.Errorf("refusals %v, want one naming %q", lc.Refusals, c.want)
			}
		})
	}
}
