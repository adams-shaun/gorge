package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// pdLower lowers one AddCounter replacement line with the doubling body.
func pdLower(t *testing.T, rpm, wm map[string]string) (lRepl, error) {
	t.Helper()
	lc := &lowerCtx{
		face:  &cards.Face{Name: "Test", SVars: map[string]string{"X": "ReplaceCount$CounterNum/Twice"}},
		out:   &lCard{},
		known: func(string) bool { return false },
	}
	rpm["ActiveZones"] = "Battlefield"
	wm["Amount"] = "X"
	var rl lRepl
	err := lc.addCounterRepl(newParams(rpm), &cards.SA{API: "ReplaceCounter", Params: wm}, &rl)
	return rl, err
}

// TestPutterDoublerLowers: "If you would put counters on a <permanent> you
// control, put twice that many": Dr. Joseph M'Benga's any-kind line with
// ValidObject$ (Repl.C the putter-gated wildcard) and Lt. Cmdr. Geordi La
// Forge's charge-only line with ValidCard$ (Repl.C the kind, Repl.A 1).
func TestPutterDoublerLowers(t *testing.T) {
	cb := &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}}}

	rl, err := pdLower(t, map[string]string{"ValidSource": "You", "ValidObject": "Creature.YouCtrl+inZoneBattlefield"},
		map[string]string{"ValidSource": "You"})
	if err != nil {
		t.Fatal(err)
	}
	ka, err := cb.repl(rl)
	if err != nil {
		t.Fatal(err)
	}
	if ka.Repl.Kind != kop.ReplDoubleCountersOn || ka.Repl.C != kop.ReplCtrAnyYou {
		t.Fatalf("any-kind putter doubler emitted kind %d C %d, want ReplDoubleCountersOn, ReplCtrAnyYou", ka.Repl.Kind, ka.Repl.C)
	}

	rl, err = pdLower(t, map[string]string{"ValidSource": "You", "ValidCard": "Artifact.YouCtrl+inZoneBattlefield", "ValidCounterType": "CHARGE"},
		map[string]string{"ValidCounterType": "CHARGE"})
	if err != nil {
		t.Fatal(err)
	}
	if ka, err = cb.repl(rl); err != nil {
		t.Fatal(err)
	}
	if ka.Repl.Kind != kop.ReplDoubleCountersOn || ka.Repl.C != int16(kop.CtrCharge) || ka.Repl.A != 1 {
		t.Fatalf("charge putter doubler emitted kind %d C %d A %d, want ReplDoubleCountersOn, charge, 1", ka.Repl.Kind, ka.Repl.C, ka.Repl.A)
	}
}

// TestPutterDoublerRefusals: the shapes beside the two lowered ones refuse.
func TestPutterDoublerRefusals(t *testing.T) {
	for _, c := range []struct {
		name string
		rp   map[string]string
		want string
	}{
		{"opponent putter", map[string]string{"ValidSource": "Opponent", "ValidObject": "Creature.YouCtrl"}, "putter shape"},
		{"effect only", map[string]string{"ValidSource": "You", "ValidObject": "Creature.YouCtrl", "EffectOnly": "True"}, "EffectOnly"},
		{"no recipient", map[string]string{"ValidSource": "You"}, "putter recipient"},
		{"two recipients", map[string]string{"ValidSource": "You", "ValidObject": "Creature.YouCtrl", "ValidCard": "Creature.YouCtrl"}, "putter recipient"},
	} {
		if _, err := pdLower(t, c.rp, map[string]string{}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, want %q", c.name, err, c.want)
		}
	}
}
