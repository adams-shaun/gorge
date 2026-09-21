package botpolicy

import (
	"strings"
	"testing"
)

// A deck policy's cast object sets the weights and marks them set, so a
// profile whose knobs happen to be zero is still distinguishable from "no
// profile supplied" (Board.castWeights folds a zero CastWeights back into
// DefaultCastWeights, so the bit is what keeps an intentionally-zero profile
// from silently becoming the default).
func TestParseDeckPolicyReadsCastWeights(t *testing.T) {
	ps, err := ParseDeckPolicy([]byte(`{"version":1,"cast":{"CreatureBase":7,"CastThreshold":-50}}`))
	if err != nil {
		t.Fatalf("ParseDeckPolicy: %v", err)
	}
	if !ps.CastSet {
		t.Fatalf("CastSet = false, want true")
	}
	if ps.Cast.CreatureBase != 7 {
		t.Errorf("CreatureBase = %d, want 7", ps.Cast.CreatureBase)
	}
	if ps.Cast.CastThreshold != -50 {
		t.Errorf("CastThreshold = %d, want -50", ps.Cast.CastThreshold)
	}
	if len(ps.UnknownClasses) != 0 {
		t.Errorf("UnknownClasses = %v, want none", ps.UnknownClasses)
	}
}

// An all-zero cast object still marks CastSet: this is the case the bit
// exists for, and reading it as "unset" would hand the seat the default
// profile while the deck author asked for zeros.
func TestParseDeckPolicyZeroWeightsStillSet(t *testing.T) {
	ps, err := ParseDeckPolicy([]byte(`{"cast":{}}`))
	if err != nil {
		t.Fatalf("ParseDeckPolicy: %v", err)
	}
	if !ps.CastSet {
		t.Fatalf("CastSet = false for an explicit empty cast object, want true")
	}
	if ps.Cast != (CastWeights{}) {
		t.Errorf("Cast = %+v, want the zero value", ps.Cast)
	}
}

// The version field is optional but closed: absent means the current
// version, and any other value is an error rather than a best-effort read.
func TestParseDeckPolicyVersion(t *testing.T) {
	if _, err := ParseDeckPolicy([]byte(`{"cast":{"CreatureBase":1}}`)); err != nil {
		t.Errorf("absent version: %v, want accepted", err)
	}
	_, err := ParseDeckPolicy([]byte(`{"version":2,"cast":{"CreatureBase":1}}`))
	if err == nil {
		t.Fatalf("version 2 accepted, want rejected")
	}
	if !strings.Contains(err.Error(), "unsupported version 2") {
		t.Errorf("error = %q, want it to name the unsupported version", err)
	}
}

// A decision class this build does not implement is CARRIED, not rejected:
// a deck authored against a later build must still load, and the caller is
// told which classes went unread rather than being silently short-changed.
func TestParseDeckPolicyUnknownClassIsCarried(t *testing.T) {
	ps, err := ParseDeckPolicy([]byte(`{"cast":{"CreatureBase":3},"combat":{"x":1},"arrange":{"y":2}}`))
	if err != nil {
		t.Fatalf("ParseDeckPolicy: %v", err)
	}
	if !ps.CastSet || ps.Cast.CreatureBase != 3 {
		t.Errorf("cast half not read: %+v", ps)
	}
	// Sorted, because this list reaches a log or error message and a map
	// range would make that message nondeterministic.
	want := []string{"arrange", "combat"}
	if len(ps.UnknownClasses) != len(want) {
		t.Fatalf("UnknownClasses = %v, want %v", ps.UnknownClasses, want)
	}
	for i, c := range want {
		if ps.UnknownClasses[i] != c {
			t.Fatalf("UnknownClasses = %v, want %v (sorted)", ps.UnknownClasses, want)
		}
	}
}

// A policy that names only unimplemented classes is still a valid document
// (it just does nothing here) -- but one that names NO class at all is a
// mistake worth reporting, since it can only be an authoring error.
func TestParseDeckPolicyRejectsEmptyDocument(t *testing.T) {
	if _, err := ParseDeckPolicy([]byte(`{"version":1}`)); err == nil {
		t.Fatalf("a document setting no class was accepted, want rejected")
	}
	if _, err := ParseDeckPolicy([]byte(`{"combat":{"x":1}}`)); err != nil {
		t.Errorf("a document setting only an unimplemented class: %v, want accepted", err)
	}
}

// An unknown WEIGHT name is rejected. This is the asymmetry with the class
// keys above and it is deliberate: a typo'd knob that parsed to zero would
// be indistinguishable from a deliberately-zero weight, so the profile would
// look tuned while one term was silently missing.
func TestParseDeckPolicyRejectsUnknownWeight(t *testing.T) {
	_, err := ParseDeckPolicy([]byte(`{"cast":{"CreatureBase":1,"NoSuchKnob":9}}`))
	if err == nil {
		t.Fatalf("unknown weight name accepted, want rejected")
	}
	if !strings.Contains(err.Error(), "NoSuchKnob") {
		t.Errorf("error = %q, want it to name the offending field", err)
	}
}

// The deck-embedded path and the standalone profile-file path must agree
// about which weight names exist, or a profile fitted through one and loaded
// through the other would drift. Both route their cast object through
// decodeCastWeights; this pins that they reject the same thing.
func TestDeckPolicyAndProfileShareCastStrictness(t *testing.T) {
	const badKnob = `"CreatureBase":1,"NoSuchKnob":9`

	_, deckErr := ParseDeckPolicy([]byte(`{"cast":{` + badKnob + `}}`))
	_, profErr := ParseCastProfile([]byte(`{"version":1,"cast":{` + badKnob + `}}`))

	if deckErr == nil || profErr == nil {
		t.Fatalf("strictness disagrees: deck err = %v, profile err = %v (both want non-nil)", deckErr, profErr)
	}
	if !strings.Contains(deckErr.Error(), "NoSuchKnob") || !strings.Contains(profErr.Error(), "NoSuchKnob") {
		t.Errorf("both errors should name the field: deck = %q, profile = %q", deckErr, profErr)
	}

	// And they must agree on the positive case too: the same knobs parse to
	// the same weights through either door.
	const good = `"CreatureBase":11,"CreaturePower":2,"CastThreshold":-7`
	dps, err := ParseDeckPolicy([]byte(`{"cast":{` + good + `}}`))
	if err != nil {
		t.Fatalf("deck policy: %v", err)
	}
	pw, err := ParseCastProfile([]byte(`{"version":1,"cast":{` + good + `}}`))
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if dps.Cast != pw {
		t.Errorf("same weights parsed differently:\n deck    = %+v\n profile = %+v", dps.Cast, pw)
	}
}

// Trailing data never half-loads a document.
func TestParseDeckPolicyRejectsTrailingData(t *testing.T) {
	if _, err := ParseDeckPolicy([]byte(`{"cast":{"CreatureBase":1}} {"cast":{}}`)); err == nil {
		t.Fatalf("trailing data accepted, want rejected")
	}
	if _, err := ParseDeckPolicy([]byte(`null`)); err == nil {
		t.Fatalf("null document accepted, want rejected")
	}
}
