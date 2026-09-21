package deck

import (
	"strings"
	"testing"
)

const twoPolicyDeck = `{
  "name": "Probe Deck",
  "format": "custom",
  "policies": {
    "aggro-tuned": {"version": 1, "cast": {"CastThreshold": -50}},
    "default":     {"version": 1, "cast": {"CreatureBase": 30}}
  },
  "cards": [{"name": "Mountain", "count": 60}]
}`

// The inertness pin, and the reason this whole field is safe to add: a deck
// that declares no policies parses to a NIL map and gains no behaviour. The
// golden acceptance games in rules/heads_test.go are bot-answered, and three
// of the twelve legacy golden decks (mono-blue-tempo, mono-black-aggro,
// mono-green-stompy) are also botbench decks -- so anything that made a deck
// policy apply without a caller naming it would move all four pinned chain
// heads. Nothing may auto-apply.
func TestDeckWithoutPoliciesIsInert(t *testing.T) {
	f, err := Parse([]byte(`{"name":"Plain","format":"custom","cards":[{"name":"Mountain","count":60}]}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Policies != nil {
		t.Errorf("Policies = %v, want nil for a deck declaring none", f.Policies)
	}
	if got := f.PolicyNames(); got != nil {
		t.Errorf("PolicyNames() = %v, want nil", got)
	}
	// And asking for one names the situation truthfully rather than
	// pretending a default exists.
	_, err = f.Policy("aggro-tuned")
	if err == nil {
		t.Fatalf("Policy on a policy-less deck succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "declares none") {
		t.Errorf("error = %q, want it to say the deck declares none", err)
	}
}

func TestDeckPolicyLookup(t *testing.T) {
	f, err := Parse([]byte(twoPolicyDeck))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	raw, err := f.Policy("aggro-tuned")
	if err != nil {
		t.Fatalf("Policy(aggro-tuned): %v", err)
	}
	if !strings.Contains(string(raw), "-50") {
		t.Errorf("policy document = %s, want the CastThreshold it declared", raw)
	}
}

// A missing name is a hard error that LISTS what the deck has. The listing is
// the whole value of the error: the operator's next action is to add the
// name, and a bare "unknown policy" would not say to what.
func TestDeckPolicyMissingNameListsAvailable(t *testing.T) {
	f, err := Parse([]byte(twoPolicyDeck))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = f.Policy("control-tuned")
	if err == nil {
		t.Fatalf("missing policy accepted, want an error")
	}
	msg := err.Error()
	for _, want := range []string{"control-tuned", "Probe Deck", "aggro-tuned", "default"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q is missing %q", msg, want)
		}
	}
	// Sorted, not map order: this string reaches an operator and must be the
	// same on every run.
	if i, j := strings.Index(msg, "aggro-tuned"), strings.Index(msg, "default"); i > j {
		t.Errorf("error %q lists names out of sorted order", msg)
	}
}

// PolicyNames is sorted because a map range that reaches a message (or a
// decision) is a determinism bug. Pinned over enough names that insertion
// order would be visible if it leaked.
func TestPolicyNamesAreSorted(t *testing.T) {
	f, err := Parse([]byte(`{
	  "name": "Sort Probe", "format": "custom",
	  "policies": {
	    "zulu": {"cast":{}}, "alpha": {"cast":{}}, "mike": {"cast":{}},
	    "bravo": {"cast":{}}, "yankee": {"cast":{}}
	  },
	  "cards": [{"name":"Mountain","count":60}]
	}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := f.PolicyNames()
	want := []string{"alpha", "bravo", "mike", "yankee", "zulu"}
	if len(got) != len(want) {
		t.Fatalf("PolicyNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PolicyNames() = %v, want %v", got, want)
		}
	}
	// Repeated calls agree -- the guard against a future implementation that
	// forgets the sort and passes once by luck of the hash seed.
	for i := 0; i < 8; i++ {
		again := f.PolicyNames()
		for j := range want {
			if again[j] != want[j] {
				t.Fatalf("PolicyNames() unstable across calls: %v then %v", got, again)
			}
		}
	}
}

// Parse rejects the two shapes that would otherwise fail later somewhere
// less obvious: an unnamed policy (selection is BY name, so it is
// unreachable) and a value that is not an object.
func TestDeckPolicyMalformedShapes(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "empty name",
			body: `{"name":"D","format":"custom","policies":{"":{"cast":{}}},"cards":[{"name":"Mountain","count":60}]}`,
			want: "empty name",
		},
		{
			name: "non-object value",
			body: `{"name":"D","format":"custom","policies":{"p":"nope"},"cards":[{"name":"Mountain","count":60}]}`,
			want: "not a JSON object",
		},
		{
			name: "array value",
			body: `{"name":"D","format":"custom","policies":{"p":[1,2]},"cards":[{"name":"Mountain","count":60}]}`,
			want: "not a JSON object",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.body))
			if err == nil {
				t.Fatalf("accepted, want rejected")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}
