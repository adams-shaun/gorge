package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const winterScript = `Name:Test Winter
ManaCost:1 B R G
Types:Legendary Creature Human Warlock
PT:3/4
S:Mode$ Continuous | Condition$ Delirium | Affected$ Opponent | SetMaxHandSize$ Y | Description$ Delirium
SVar:X:Count$ValidGraveyard Card.YouOwn$CardTypes
SVar:Y:Number$7/Minus.X
Oracle:Delirium.
`

// TestMaxHandOpp: an Affected$ Opponent SetMaxHandSize$ naming an SVar (Winter,
// Misanthropic Guide) lowers to NoMaxHandSize with the opponents flag and a
// dynamic amount under its Delirium gate; any other param beside it, and a
// literal or Unlimited value for opponents, still refuse.
func TestMaxHandOpp(t *testing.T) {
	if r := lowerScript(t, "Test Winter", winterScript); len(r) != 0 {
		t.Fatalf("Winter refused: %v", r)
	}
	p := lowerScriptCard(t, "Test Winter", winterScript)
	var found bool
	for _, st := range p.Statics {
		if st.Kind == "NoMaxHandSize" {
			found = true
			if len(st.Flags) != 1 || st.Flags[0] != "HandOpp" || st.Amount.isConst() || st.Cond.Kind == cNone {
				t.Errorf("static = flags %v amount %v cond %v, want HandOpp, a dynamic amount and the delirium gate", st.Flags, st.Amount, st.Cond)
			}
		}
	}
	if !found {
		t.Fatalf("no NoMaxHandSize static in %+v", p.Statics)
	}
	for _, tc := range []struct{ name, from, to, want string }{
		{"another param", "SetMaxHandSize$ Y", "SetMaxHandSize$ Y | AddKeyword$ Hexproof", "beside AddKeyword$"},
		{"a literal", "SetMaxHandSize$ Y", "SetMaxHandSize$ 4", "for opponents"},
		{"unlimited", "SetMaxHandSize$ Y", "SetMaxHandSize$ Unlimited", "for opponents"},
	} {
		r := lowerScript(t, "Test Winter", strings.Replace(winterScript, tc.from, tc.to, 1))
		if len(r) == 0 || !strings.Contains(strings.Join(r, "\n"), tc.want) {
			t.Errorf("%s: refusals %v, want one containing %q", tc.name, r, tc.want)
		}
	}
}

// lowerScriptCard lowers an inline script and returns the lowered card.
func lowerScriptCard(t *testing.T, name, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: testutil.CorpusRegistry(t)}
	return lowerCard(p, name, card, false)
}
