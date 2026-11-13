package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// escapeBfSynth lowers a synthetic creature with the given K:Escape line and
// returns its refusals.
func escapeBfSynth(t *testing.T, escape string) []refusal {
	t.Helper()
	return exileBfSynth(t, "Name:Exile Bf Probe\nManaCost:2 G\nTypes:Creature Beast\nPT:2/2\nK:Escape:"+escape+"\n")
}

// TestEscapeCostBattlefieldExile: Lunar Hatchling's K:Escape:4 G U Exile<1/Land>
// ExileFromGrave<5/Card.Other/other> lowers, the battlefield exile into
// CardDef.EscBfCost (one CostSac part with ExileBf, a Land filter) beside the
// grave exile in CardDef.EscCost; every other shape of the part still refuses
// by name, the exile token after the ExileFromGrave one among them (gorge asks
// the parts in token order).
func TestEscapeCostBattlefieldExile(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Lunar Hatchling")
	if !ok || card == nil {
		t.Fatal("Lunar Hatchling not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{"Lunar Hatchling": card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, "Lunar Hatchling", card, false)
	if got := keclReason(lc.Refusals); got != "" {
		t.Fatalf("Lunar Hatchling refuses: %q", got)
	}
	if lc.EscBf == nil || lc.EscCost == nil {
		t.Fatalf("lowered EscBf %v EscCost %v, want both", lc.EscBf, lc.EscCost)
	}
	_, bs, pt, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatal(err)
	}
	if len(lc.Refusals) != 0 {
		t.Fatalf("Lunar Hatchling refuses at build: %q", keclReason(lc.Refusals))
	}
	def := bs[0].def
	bf, gr := def.EscBfCost, def.EscCost
	if bf.Flags != kop.CostSac || !bf.ExileBf || bf.N != 1 {
		t.Errorf("EscBfCost flags %b ExileBf %v N %d, want CostSac, ExileBf, 1", bf.Flags, bf.ExileBf, bf.N)
	}
	if f := pt.filters[bf.Filter]; f.Types&kop.TLand == 0 {
		t.Errorf("EscBfCost filter types %b, want Land", f.Types)
	}
	if gr.Flags&kop.CostExileGrave == 0 || gr.Flags&kop.CostSac != 0 || gr.ExileBf || gr.N != 5 {
		t.Errorf("EscCost flags %b ExileBf %v N %d, want the grave exile of 5 alone", gr.Flags, gr.ExileBf, gr.N)
	}
	if !def.Escape {
		t.Error("CardDef.Escape unset")
	}

	// a plain escape card carries no battlefield part
	if rs := escapeBfSynth(t, "2 G ExileFromGrave<3/Card.Other/other>"); len(rs) != 0 {
		t.Errorf("plain escape refused: %q", keclReason(rs))
	}
	for _, tc := range []struct{ cost, want string }{
		{"2 G ExileFromGrave<3/Card.Other/other> Exile<1/Land>", "after the ExileFromGrave part"}, // gorge asks in token order
		{"2 G Sac<1/Land> ExileFromGrave<3/Card.Other/other>", "beside another counted part"},     // the cost lowering's own census
		{"2 G Exile<1/Land> Exile<1/Creature> ExileFromGrave<3/Card.Other/other>", "beside a sacrifice part"},
		{"2 G Exile<1/Land> Exile<1/CARDNAME> ExileFromGrave<3/Card.Other/other>", "exile-self cost"},
		{"2 G Exile<1/Land>", "without an ExileFromGrave<N/Card.Other> part"},
		{"2 G Exile<1/Land> PayLife<2> ExileFromGrave<3/Card.Other/other>", "beside another counted part"},
	} {
		got := keclReason(escapeBfSynth(t, tc.cost))
		if !strings.Contains(got, tc.want) {
			t.Errorf("K:Escape:%s: refusals %q, want %q", tc.cost, got, tc.want)
		}
	}
}
