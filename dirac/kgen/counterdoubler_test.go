package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestDoubleCountersOnKeepsKind: the filtered doubler's ValidCounterType
// must survive lowering and emission as Repl.C. Before, the kind was dropped
// and every filtered doubler doubled every counter kind.
func TestDoubleCountersOnKeepsKind(t *testing.T) {
	lc := &lowerCtx{
		face:  &cards.Face{Name: "Test", SVars: map[string]string{"Amount": "ReplaceCount$CounterNum/Twice"}},
		out:   &lCard{},
		known: func(string) bool { return false },
	}
	rp := newParams(map[string]string{
		"ActiveZones":      "Battlefield",
		"ValidCard":        "Creature.YouCtrl",
		"ValidCounterType": "Lifelink",
	})
	w := &cards.SA{API: "ReplaceCounter", Params: map[string]string{
		"ValidCounterType": "Lifelink",
		"Amount":           "Amount",
	}}
	var rl lRepl
	if err := lc.addCounterRepl(rp, w, &rl); err != nil {
		t.Fatal(err)
	}
	if rl.Kind != "DoubleCountersOn" || rl.Ctr != "Lifelink" {
		t.Fatalf("lowered %s kind %q, want DoubleCountersOn kind Lifelink", rl.Kind, rl.Ctr)
	}
	cb := &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}}}
	ka, err := cb.repl(rl)
	if err != nil {
		t.Fatal(err)
	}
	if ka.Repl.Kind != kop.ReplDoubleCountersOn || ka.Repl.C != int16(kop.CtrLifelink) {
		t.Fatalf("emitted kind %d counter %d, want ReplDoubleCountersOn %d", ka.Repl.Kind, ka.Repl.C, kop.CtrLifelink)
	}
}

// TestTwoExileOnLeaveRefused: State.exileOnLeave holds one filter per
// card, so a face with two dies-to-exile abilities would silently use the
// last filter for both. Lowering must refuse it.
func TestTwoExileOnLeaveRefused(t *testing.T) {
	lc := &lowerCtx{
		face:  &cards.Face{Name: "Test"},
		out:   &lCard{Repls: []lRepl{{Kind: "ExileOnLeave"}}},
		known: func(string) bool { return false },
	}
	r := cards.Repl{
		Event: "Moved",
		Params: map[string]string{
			"Event":       "Moved",
			"ActiveZones": "Battlefield",
			"Origin":      "Battlefield",
			"Destination": "Graveyard",
			"ValidLKI":    "Creature.OppCtrl",
			"ReplaceWith": "Exile",
		},
		With: &cards.SA{API: "ChangeZone", Params: map[string]string{
			"Defined":     "ReplacedCard",
			"Destination": "Exile",
			"Origin":      "All",
			"Hidden":      "True",
		}},
	}
	if err := lc.lowerRepl(r); err == nil || !strings.Contains(err.Error(), "two dies-to-exile") {
		t.Fatalf("lowerRepl = %v, want the two-dies-to-exile refusal", err)
	}
}
