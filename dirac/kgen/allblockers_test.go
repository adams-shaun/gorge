package kgen

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// abLower lowers one synthetic card with the every-blocker mode on or off.
func abLower(t *testing.T, src string, all bool) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}, allBlockers: all}
	return lowerPool(p).cards[0]
}

func abReasons(c *lCard) string {
	var rs []string
	for _, r := range c.Refusals {
		rs = append(rs, r.Reason)
	}
	return strings.Join(rs, "\n")
}

// TestAllBlockersReportsBothRefusals: one damage node whose target filter and
// whose amount SVar both refuse reports only the filter without the mode, and
// both with it; a second node of the chain refusing for its own reason is
// listed after the first.
func TestAllBlockersReportsBothRefusals(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe",
		"ManaCost:R",
		"Types:Instant",
		"A:SP$ DealDamage | ValidTgts$ Any.!EnchantedBy | NumDmg$ X | SubAbility$ DBDraw | SpellDescription$ Probe.",
		"SVar:DBDraw:DB$ Draw | Defined$ Bogus$Head | NumCards$ 1",
		"SVar:X:Enchanted$CardPower",
	}, "\n")
	first := abReasons(abLower(t, src, false))
	if !strings.Contains(first, `unknown filter head "Any"`) {
		t.Fatalf("first refusal missing: %q", first)
	}
	if strings.Contains(first, "Enchanted$") {
		t.Fatalf("the default mode reads past the first refusal: %q", first)
	}
	all := abReasons(abLower(t, src, true))
	for _, want := range []string{`unknown filter head "Any"`, `SVar X="Enchanted$CardPower"`, "Draw"} {
		if !strings.Contains(all, want) {
			t.Errorf("every-blocker refusals lack %q:\n%s", want, all)
		}
	}
	if len(strings.Split(all, "\n")) <= len(strings.Split(first, "\n")) {
		t.Errorf("every-blocker mode found no more than the default:\n%s", all)
	}
}

// TestAllBlockersLeavesLoweringAlone: a card that lowers cleanly lowers to the
// same program with the mode on, so tables do not move.
func TestAllBlockersLeavesLoweringAlone(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe",
		"ManaCost:1 R",
		"Types:Instant",
		"A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2 | SubAbility$ ProbeDraw | SpellDescription$ Probe.",
		"SVar:ProbeDraw:DB$ Draw | NumCards$ 1 | Defined$ You",
	}, "\n")
	a, b := abLower(t, src, false), abLower(t, src, true)
	if len(a.Refusals) != 0 || len(b.Refusals) != 0 {
		t.Fatalf("fixture refuses: %q / %q", abReasons(a), abReasons(b))
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("the every-blocker mode changed a clean card's lowering")
	}
}
