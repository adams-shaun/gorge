package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kvcyoStatic lowers a probe permanent carrying one static and returns its
// refusal reasons and the dumped LIR.
func kvcyoStatic(t *testing.T, types, static string) (string, string) {
	t.Helper()
	src := strings.Join([]string{"Name:Probe", "ManaCost:1 U", "Types:" + types, "PT:2/2", static}, "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerPool(p).cards[0]
	var out []string
	for _, r := range lc.Refusals {
		out = append(out, r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, lc)
	return strings.Join(out, "; "), sb.String()
}

// TestStaticValidCardYourOwn: UntapOtherPlayer, CanAttackDefender and
// CantAttack lower with a ValidCard$ over the controller's own permanents
// (Unwinding Clock, Ghalta the Immovable, Eriette of the Charmed Apple);
// the shapes the kernel cannot read keep refusing.
func TestStaticValidCardYourOwn(t *testing.T) {
	for _, c := range []struct{ name, types, static, want, lir string }{
		{"untap other, own artifacts", "Artifact", "S:Mode$ UntapOtherPlayer | ValidCard$ Artifact.YouCtrl | Description$ x", "", "static UntapOtherPlayer"},
		{"untap other, self", "Artifact", "S:Mode$ UntapOtherPlayer | ValidCard$ Card.Self | Description$ x", "", "static UntapOtherPlayer"},
		{"untap other, absent ValidCard", "Artifact", "S:Mode$ UntapOtherPlayer | Description$ x", "", "static UntapOtherPlayer"},
		{"defender, own creatures", "Creature Bear", "S:Mode$ CanAttackDefender | ValidCard$ Creature.YouCtrl | Description$ x", "", "static CanAttackDefender"},
		{"defender, ValidAttacked unread", "Creature Bear", "S:Mode$ CanAttackDefender | ValidCard$ Creature.YouCtrl | ValidAttacked$ Player | Description$ x", "ValidAttacked", ""},
		{"defender, no ValidCard", "Creature Bear", "S:Mode$ CanAttackDefender | Description$ x", "CanAttackDefender without ValidCard$", ""},
		{"cant attack, enchanted by own aura", "Creature Bear", "S:Mode$ CantAttack | ValidCard$ Creature.EnchantedBy Aura.YouCtrl | Target$ You,Planeswalker.YouCtrl | Description$ x", "", "static CantAttack"},
		{"cant attack, creature side controller", "Creature Bear", "S:Mode$ CantAttack | ValidCard$ Creature.YouCtrl | Target$ You,Planeswalker.YouCtrl | Description$ x", "CantAttack Target$ with a controller filter", ""},
		{"cant attack, opponent controller restated", "Creature Bear", "S:Mode$ CantAttack | ValidCard$ Creature.OppCtrl | Target$ You,Planeswalker.YouCtrl | Description$ x", "", "static CantAttack"},
		{"cant attack, other target", "Creature Bear", "S:Mode$ CantAttack | ValidCard$ Creature.EnchantedBy Aura.YouCtrl | Target$ Player | Description$ x", "CantAttack Target$", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, lir := kvcyoStatic(t, c.types, c.static)
			if c.want == "" {
				if got != "" {
					t.Fatalf("refused: %s", got)
				}
				if !strings.Contains(lir, c.lir) {
					t.Fatalf("lowered IR lacks %q:\n%s", c.lir, lir)
				}
				return
			}
			if !strings.Contains(got, c.want) {
				t.Fatalf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
