package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// forageManaProbe lowers and builds a synthetic card and returns its refusals.
func forageManaProbe(t *testing.T, src string) ([]refusal, []built) {
	t.Helper()
	card, diags := cards.ParseBytes("foragemana_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Forage Mana Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: testutil.CorpusRegistry(t)} // (the corpus registry knows the Food subtype)
	lc := lowerCard(p, name, card, false)
	if len(lc.Refusals) != 0 {
		return lc.Refusals, nil
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return lc.Refusals, all
}

// TestForageManaLowers: Thornvault Forager's "{T}, Forage: Add two mana in any
// combination of colors" lowers with no refusal and builds the kop.ForageComboAnyAb
// shape beside its plain {T} ability; a forage on any other mana ability (a fixed
// colour, one mana, a mana fee beside it) keeps a refusal that names the cause.
func TestForageManaLowers(t *testing.T) {
	lc := exileGraveLower(t, "Thornvault Forager")
	for _, rf := range lc.Refusals {
		t.Errorf("Thornvault Forager refused: %s", rf.Reason)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	saw := false
	for i := range all[0].def.Abilities {
		if a := &all[0].def.Abilities[i]; kop.ForageComboAnyAb(a) {
			saw = true
			if a.ManaColors != kop.ColAny || a.ManaN != 2 || a.Cost.Flags != kop.CostTap|kop.CostForage || a.Cost.Filter == 0 {
				t.Errorf("mana %b x%d, cost %+v: want two of any colour, {T} + forage with the Food filter", a.ManaColors, a.ManaN, a.Cost)
			}
		}
	}
	if !saw {
		t.Error("Thornvault Forager built with no forage Combo Any mana ability")
	}
	if len(all[0].def.Abilities) == 0 {
		t.Fatal("Thornvault Forager built with no abilities")
	}

	const body = "Name:Forage Mana Probe\nManaCost:0\nTypes:Artifact Creature Squirrel\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ G | Amount$ 1\nA:AB$ Mana | Cost$ %s | Produced$ %s | Amount$ %s\n"
	for _, c := range []struct {
		cost, produced, amount string
		ok                     bool
	}{
		{"T Forage", "Combo Any", "2", true},
		{"T Forage", "Combo Any", "3", true},
		{"T Forage", "G", "1", false},
		{"T Forage", "Combo Any", "1", false},
		{"1 T Forage", "Combo Any", "2", false},
		{"Forage", "Combo Any", "2", false},
		{"T Forage Sac<1/CARDNAME>", "Combo Any", "2", false},
	} {
		src := strings.NewReplacer("%s", "\x00").Replace(body)
		for _, v := range []string{c.cost, c.produced, c.amount} {
			src = strings.Replace(src, "\x00", v, 1)
		}
		got, _ := forageManaProbe(t, src)
		reason := keclReason(got)
		if c.ok != (reason == "") {
			t.Errorf("cost %q produces %q x%s: refusals %q, want refused=%v", c.cost, c.produced, c.amount, reason, !c.ok)
		}
		if !c.ok && !strings.Contains(reason, "forage") && !strings.Contains(reason, "cost") {
			t.Errorf("cost %q produces %q x%s: refusal %q names neither the forage nor the cost", c.cost, c.produced, c.amount, reason)
		}
	}
}
