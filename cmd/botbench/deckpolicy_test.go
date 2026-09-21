package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/deck"
	"github.com/adams-shaun/gorge/internal/testutil"
)

func deckWith(name string, policies map[string]string) deck.File {
	f := deck.File{Name: name}
	if len(policies) > 0 {
		f.Policies = make(map[string]json.RawMessage, len(policies))
		for k, v := range policies {
			f.Policies[k] = json.RawMessage(v)
		}
	}
	return f
}

// A built-in name resolves to the built-in constructor whatever deck the seat
// holds. This is the backward-compatibility property the whole change rests
// on: every existing invocation (-a bot -b policynet, the ten approved mono
// pairs) must keep resolving to exactly the seat it resolved to before, or
// the measured baselines stop comparing.
func TestSeatCtorForDeckBuiltinWinsAndIgnoresDeck(t *testing.T) {
	for _, name := range builtinPolicyNames() {
		plain := deckWith("Plain", nil)
		if _, err := seatCtorForDeck(name, plain); err != nil {
			t.Errorf("built-in %q against a policy-less deck: %v, want resolved", name, err)
		}
	}
}

// The point of the feature: a name the deck declares resolves to a seat
// carrying that deck's weights.
func TestSeatCtorForDeckResolvesADeclaredPolicy(t *testing.T) {
	d := deckWith("Probe", map[string]string{
		"aggro-tuned": `{"version":1,"cast":{"CastThreshold":-50}}`,
	})
	ctor, err := seatCtorForDeck("aggro-tuned", d)
	if err != nil {
		t.Fatalf("seatCtorForDeck: %v", err)
	}
	if s := ctor(1); s == nil {
		t.Fatalf("constructor returned a nil seat")
	}
}

// A name no deck declares is a HARD ERROR naming the deck, the missing name,
// what the deck does declare, and the built-ins. No silent fallback to the
// default bot: a fallback would make side A identical to side B for exactly
// the decks missing the policy under test, so the run would report "no
// effect" when the truth is "the policy was never loaded".
func TestSeatCtorForDeckMissingPolicyIsAHardError(t *testing.T) {
	d := deckWith("Probe", map[string]string{
		"aggro-tuned": `{"cast":{"CastThreshold":-50}}`,
	})
	_, err := seatCtorForDeck("control-tuned", d)
	if err == nil {
		t.Fatalf("missing policy resolved, want an error")
	}
	msg := err.Error()
	for _, want := range []string{"control-tuned", "Probe", "aggro-tuned", "bot"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q is missing %q", msg, want)
		}
	}
}

// A document whose every class is one this build cannot read must FAIL, not
// quietly seat the stock bot: seating the default here would look in the
// report exactly like a tuned policy that made no difference.
func TestSeatCtorForDeckUnreadableClassesFail(t *testing.T) {
	d := deckWith("Probe", map[string]string{
		"combat-only": `{"combat":{"someKnob":1}}`,
	})
	_, err := seatCtorForDeck("combat-only", d)
	if err == nil {
		t.Fatalf("a policy setting only unreadable classes resolved, want an error")
	}
	if !strings.Contains(err.Error(), "combat") {
		t.Errorf("error = %q, want it to name the unread class", err)
	}
}

// A malformed weight inside a class this build DOES read fails the run rather
// than loading a half-tuned profile.
func TestSeatCtorForDeckRejectsBadWeights(t *testing.T) {
	d := deckWith("Probe", map[string]string{
		"typo": `{"cast":{"CastThreshhold":-50}}`, // note the misspelling
	})
	_, err := seatCtorForDeck("typo", d)
	if err == nil {
		t.Fatalf("misspelled weight resolved, want an error")
	}
	if !strings.Contains(err.Error(), "CastThreshhold") {
		t.Errorf("error = %q, want it to name the offending field", err)
	}
}

// Built-ins take precedence, so a deck policy that shadows one would be
// silently unreachable -- the failure mode where a run looks like it
// exercised a tuned profile and actually re-ran the stock bot. Reject it at
// load instead.
func TestDeckPolicyShadowCheck(t *testing.T) {
	ok := deckWith("Fine", map[string]string{"aggro-tuned": `{"cast":{}}`})
	if err := deckPolicyShadowCheck(ok); err != nil {
		t.Errorf("a non-colliding deck was rejected: %v", err)
	}

	for _, name := range builtinPolicyNames() {
		bad := deckWith("Shadow", map[string]string{name: `{"cast":{}}`})
		err := deckPolicyShadowCheck(bad)
		if err == nil {
			t.Errorf("deck shadowing built-in %q accepted, want rejected", name)
			continue
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q should name the colliding policy %q", err, name)
		}
	}
}

// Every repo deck must pass the shadow check as committed, so the bench can
// never be started against a deck whose policy is unreachable. Also the
// inertness pin at the repo level: no committed deck declares a policy yet,
// and nothing may start applying one without a caller naming it (the golden
// acceptance games are bot-answered, and three of the twelve legacy golden
// decks are also botbench decks).
func TestRepoDecksPassPolicyShadowCheck(t *testing.T) {
	for _, name := range testutil.RepoDeckNames() {
		f, err := testutil.LoadRepoDeckFile(name)
		if err != nil {
			t.Fatalf("deck %q: %v", name, err)
		}
		if err := deckPolicyShadowCheck(f); err != nil {
			t.Errorf("deck %q: %v", name, err)
		}
	}
}
