package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPinConditionValidate: every condition kind the manifest can judge is
// accepted when complete and refused when a field it reads is missing, and
// readPinDoc refuses a document holding a bad condition before any game is
// played.
func TestPinConditionValidate(t *testing.T) {
	t.Parallel()
	card := func(kind string) pinCondition { return pinCondition{Kind: kind, Card: "Grizzly Bears"} }
	counter := pinCondition{Kind: "counterAtLeast", Card: "Enterprise NX-01", Counter: "CHARGE", N: 1}
	without := func(c pinCondition, f func(*pinCondition)) pinCondition { f(&c); return c }
	cases := []struct {
		name string
		c    pinCondition
		ok   bool
	}{
		{"cardOut", card("cardOut"), true},
		{"cardOut no card", pinCondition{Kind: "cardOut"}, false},
		{"onBattlefield", card("onBattlefield"), true},
		{"onBattlefield no card", pinCondition{Kind: "onBattlefield"}, false},
		{"cardSeen", card("cardSeen"), true},
		{"cardSeen no card", pinCondition{Kind: "cardSeen"}, false},
		{"onStack", card("onStack"), true},
		{"onStack no card", pinCondition{Kind: "onStack"}, false},
		{"activated", card("activated"), true},
		{"activated no card", pinCondition{Kind: "activated"}, false},
		{"counterAtLeast", counter, true},
		{"counterAtLeast no card", without(counter, func(c *pinCondition) { c.Card = "" }), false},
		{"counterAtLeast no counter", without(counter, func(c *pinCondition) { c.Counter = "" }), false},
		{"counterAtLeast n 0", without(counter, func(c *pinCondition) { c.N = 0 }), false},
		{"counterAtLeast n negative", without(counter, func(c *pinCondition) { c.N = -1 }), false},
		{"anyOf", pinCondition{Kind: "anyOf", Conditions: []pinCondition{card("cardOut"), counter}}, true},
		{"anyOf no conditions", pinCondition{Kind: "anyOf"}, false},
		{"anyOf bad alternative", pinCondition{Kind: "anyOf", Conditions: []pinCondition{card("cardOut"), {Kind: "onStack"}}}, false},
		{"anyOf nested bad alternative", pinCondition{Kind: "anyOf", Conditions: []pinCondition{
			{Kind: "anyOf", Conditions: []pinCondition{without(counter, func(c *pinCondition) { c.Counter = "" })}}}}, false},
		{"check", pinCondition{Kind: "check", Check: "hybridCast"}, true},
		{"check stray card", pinCondition{Kind: "check", Check: "hybridCast", Card: "Grizzly Bears"}, false},
		{"check unknown", pinCondition{Kind: "check", Check: "anyOut"}, false},
		{"check no name", pinCondition{Kind: "check"}, false},
		{"check maker", pinCondition{Kind: "check", Check: "castThroughPayLifeGrant", Card: "Inside Information"}, true},
		{"check maker no card", pinCondition{Kind: "check", Check: "castThroughPayLifeGrant"}, false},
		{"unknown kind", card("anyOut"), false},
		{"no kind", pinCondition{Card: "Grizzly Bears"}, false},
	}
	for _, c := range cases {
		err := c.c.validate()
		if c.ok && err != nil {
			t.Errorf("%s: %v, want nil", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: nil, want an error", c.name)
		}
	}

	// The check registry is sorted by name (a lookup is a binary search) and
	// each entry is exactly one of a plain predicate and a field-reading maker.
	for i, ch := range pinChecks {
		if i > 0 && pinChecks[i-1].name >= ch.name {
			t.Errorf("pinChecks not sorted at %q (after %q)", ch.name, pinChecks[i-1].name)
		}
		if ch.pred == nil && ch.maker == nil {
			t.Errorf("check %q: neither pred nor maker", ch.name)
		}
		if ch.pred != nil && ch.maker != nil {
			t.Errorf("check %q: both pred and maker", ch.name)
		}
	}

	// readPinDoc runs validate: the same document reads with a complete
	// condition and is refused with an incomplete one.
	dir := t.TempDir()
	doc := func(cond string) string {
		path := filepath.Join(dir, "TestX.0.json")
		body := `{"pool":"std-trk","config":{},"mulligans":1,"conditions":[` + cond + `],"games":[{"format":"koracle-repro-2","decks":["a","b"]}]}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if _, ok, err := readPinDoc(doc(`{"kind":"counterAtLeast","card":"Enterprise NX-01","counter":"CHARGE","n":1}`)); err != nil || !ok {
		t.Fatalf("complete document: ok %v, err %v; want it read", ok, err)
	}
	_, _, err := readPinDoc(doc(`{"kind":"counterAtLeast","card":"Enterprise NX-01","counter":"CHARGE"}`))
	if err == nil || !strings.Contains(err.Error(), "counterAtLeast") {
		t.Fatalf("document with n missing: err %v, want a counterAtLeast error", err)
	}
	if _, ok, err := readPinDoc(doc(`{"kind":"check","check":"hybridCast"}`)); err != nil || !ok {
		t.Fatalf("document with a check condition: ok %v, err %v; want it read", ok, err)
	}
	_, _, err = readPinDoc(doc(`{"kind":"check","check":"anyOut"}`))
	if err == nil || !strings.Contains(err.Error(), "unknown check") {
		t.Fatalf("document with an unknown check: err %v, want an unknown-check error", err)
	}
}
