package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

const delveGrant = "S:Mode$ Continuous | Affected$ %s | AffectedZone$ Stack | AddKeyword$ %s%s | EffectZone$ %s\n"

func delveCard(t *testing.T, aff, kw, extra, ez string) (*lCard, error) {
	t.Helper()
	line := strings.NewReplacer().Replace(delveGrant)
	line = strings.Replace(line, "%s", aff, 1)
	line = strings.Replace(line, "%s", kw, 1)
	line = strings.Replace(line, "%s", extra, 1)
	line = strings.Replace(line, "%s", ez, 1)
	src := "Name:Delve Probe\nManaCost:2 B\nTypes:Creature Human\nPT:2/2\n" + line
	card, diags := cards.ParseBytes("delve_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Delve Probe"}, byName: map[string]*cards.Card{"Delve Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, _, _, err := buildPool(lw)
	return lw.cards[0], err
}

// TestStackkwDelveLowering: Teval's AffectedZone$ Stack Delve grant lowers
// to a StackKw static (code kop.StackKwDelve) and builds; the shapes the
// kernel cannot mirror still refuse by their reason.
func TestStackkwDelveLowering(t *testing.T) {
	lc, err := delveCard(t, "Card.YouCtrl+wasCast", "Delve", "", "Battlefield")
	if err != nil {
		t.Fatal(err)
	}
	if len(lc.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", lc.Refusals)
	}
	if len(lc.Statics) != 1 || lc.Statics[0].Kind != "StackKw" || len(lc.Statics[0].Kw) != 1 || lc.Statics[0].Kw[0] != "Delve" {
		t.Fatalf("statics %+v, want one StackKw Delve", lc.Statics)
	}
	if kop.StackKwDelve != 8 {
		t.Fatalf("StackKwDelve = %d", kop.StackKwDelve)
	}
	for _, c := range []struct{ name, aff, kw, extra, ez, want string }{
		{"parameter", "Card.YouCtrl+wasCast", "Delve", ":1", "Battlefield", "AffectedZone Stack grant of"},
		{"effect zone", "Card.YouCtrl+wasCast", "Delve", "", "Graveyard", "EffectZone"},
		{"not the caster", "Card.Other+wasCast", "Delve", "", "Battlefield", "does not name the caster"},
	} {
		lc, _ := delveCard(t, c.aff, c.kw, c.extra, c.ez)
		found := false
		for _, r := range lc.Refusals {
			found = found || strings.Contains(r.Reason, c.want)
		}
		if !found {
			t.Errorf("%s: refusals %v, want one containing %q", c.name, lc.Refusals, c.want)
		}
	}
	// a Condition$ gate refuses
	src := "Name:Delve Probe\nManaCost:2 B\nTypes:Creature Human\nPT:2/2\nS:Mode$ Continuous | Affected$ Card.YouCtrl+wasCast | AffectedZone$ Stack | AddKeyword$ Delve | EffectZone$ Battlefield | CheckSVar$ X | SVarCompare$ GE1\nSVar:X:Count$YourLifeTotal\n"
	card, _ := cards.ParseBytes("delve_probe.txt", []byte(src))
	card.Link()
	p := &pool{names: []string{"Delve Probe"}, byName: map[string]*cards.Card{"Delve Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	found := false
	for _, r := range lw.cards[0].Refusals {
		found = found || strings.Contains(r.Reason, "gate")
	}
	if !found {
		t.Errorf("gated grant: refusals %v, want the gate refusal", lw.cards[0].Refusals)
	}
}
