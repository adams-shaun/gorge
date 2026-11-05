package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

var putExistingForms = []formCase{
	{card: "Dathon, His Heart Open", form: "PutCounter CounterType$ ExistingCounter onto a target enchantment you control", lir: "PutCounterExisting obj=Target0 B=1"},
}

func TestPutCounterExistingForms(t *testing.T) { checkForms(t, putExistingForms) }

// TestPutCounterExistingBuilds: Dathon builds with no lowering or build refusal.
func TestPutCounterExistingBuilds(t *testing.T) {
	for _, c := range lowerPoolBuilt(t, []string{"Dathon, His Heart Open"}) {
		if len(c.refusals) > 0 {
			t.Errorf("%s refused: %v", c.name, c.refusals)
		}
	}
}

// putExistingRefusals lowers a synthetic enchantment whose activated
// ability is a PutCounter over a target enchantment you control, with extra
// parameters appended, and returns its refusal reasons.
func putExistingRefusals(t *testing.T, extra string) string {
	t.Helper()
	src := "Name:Existing Probe\nManaCost:1 G\nTypes:Enchantment\n" +
		"A:AB$ PutCounter | Cost$ 2 | ValidTgts$ Enchantment.YouCtrl | CounterType$ ExistingCounter | CounterNum$ 1 | SorcerySpeed$ True" + extra + "\n"
	card, diags := cards.ParseBytes("existing_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Existing Probe"}, byName: map[string]*cards.Card{"Existing Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	var got []string
	for _, r := range lw.cards[0].Refusals {
		got = append(got, r.Reason)
	}
	return strings.Join(got, "; ")
}

func TestPutCounterExistingRefusals(t *testing.T) {
	if got := putExistingRefusals(t, ""); got != "" {
		t.Fatalf("the plain form refused: %s", got)
	}
	for _, tc := range []struct{ extra, want string }{
		{" | EachExistingCounter$ True", "EachExistingCounter"},
		{" | RememberPut$ True", "RememberPut"},
		{" | RememberAmount$ True", "RememberAmount"},
		{" | Placer$ Opponent", "Placer"},
		{" | CounterNum$ X\nSVar:X:Count$Valid Sphinx.OppCtrl+tapped", "CounterNum"},
	} {
		if got := putExistingRefusals(t, tc.extra); !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want one naming %s", tc.extra, got, tc.want)
		}
	}
}
