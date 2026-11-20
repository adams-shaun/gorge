package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// An X spell paid through a hybrid-fee filter land: Stolen by the Fae
// ({X}{U}{U}) with X=1 beside an Island, two Plains and Flooded Grove
// ("{G/U}, {T}: Add {G}{G}, {G}{U} or {U}{U}"). The Island pays the Grove's
// fee, the Grove makes {U}{U}, a Plains the X. gorge offers the cast.
func TestHybridFilterFundsXSpell(t *testing.T) {
	set := cardsettest.Synth(t, "tail-15")
	names := []string{"Island", "Plains", "Flooded Grove", "Stolen by the Fae", "Foundry Street Denizen", "Rust Tick"}
	var deck []cardset.Entry
	for _, nm := range names {
		if i, ok := set.Lookup(nm); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the tail-15 table", nm)
		}
		deck = append(deck, cardset.Entry{Name: nm, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, tc := range []struct {
		name   string
		lands  []string
		target string
		want   bool
	}{
		{"x1 grove", []string{"Island", "Plains", "Plains", "Flooded Grove"}, "Foundry Street Denizen", true},
		{"x1 grove 3", []string{"Island", "Plains", "Flooded Grove"}, "Foundry Street Denizen", true},
		{"x1 plain", []string{"Island", "Island", "Plains"}, "Foundry Street Denizen", true},
		{"x1 plain+grove", []string{"Island", "Island", "Plains", "Flooded Grove"}, "Foundry Street Denizen", true},
		{"x1 islands+grove", []string{"Island", "Island", "Flooded Grove"}, "Foundry Street Denizen", true},
		{"x1 island+grove+island+plains+plains", []string{"Island", "Plains", "Plains", "Plains", "Flooded Grove"}, "Foundry Street Denizen", true},
	} {
		sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
		for _, l := range tc.lands {
			sc.perm(id[l], 0)
		}
		if tc.target != "" {
			sc.perm(id[tc.target], 1)
		} else {
			sc.perm(id["Foundry Street Denizen"], 1)
		}
		spell := sc.hand(id["Stolen by the Fae"], 0)
		d := sc.s.Step()
		if got := deferGapOffered(&d, AkCast, spell); got != tc.want {
			t.Errorf("%s: offered %v, want %v: %v", tc.name, got, tc.want, d.Options[:d.N])
		}
	}
}
