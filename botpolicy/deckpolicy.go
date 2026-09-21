package botpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// DeckPolicyVersion is the only policy-document version this build reads. A
// document may omit the field (it defaults to this value); any other value is
// an error rather than a best-effort read, the same closed-vocabulary rule
// ParseCastProfile applies to the standalone profile files.
const DeckPolicyVersion = 1

// PolicySet is one deck policy resolved into the weights a seat takes. It is
// a CONTAINER keyed by decision class, with only the cast class implemented:
// the schema carries the other classes from day one so adding a combat or
// arrange policy later is not a breaking change to every deck file, and the
// classes this build cannot read are accepted and ignored rather than
// rejected (UnknownClasses records them so a caller can say so out loud).
//
// CastSet distinguishes "the document set the cast weights" from "the cast
// weights happen to be the zero value". The distinction is load bearing:
// Board.castWeights folds a zero-value CastWeights back into
// DefaultCastWeights, so a seat must be told explicitly that a profile was
// supplied (seat.NewCastProfileBotWithWeights sets its own castSet from
// exactly this bit).
type PolicySet struct {
	Cast    CastWeights
	CastSet bool
	// UnknownClasses is the sorted list of decision-class keys the document
	// carried that this build does not implement. Sorted because it reaches
	// an error or log message, and a map range would make that message
	// nondeterministic.
	UnknownClasses []string
}

// knownPolicyClasses is the closed set of decision-class keys with a reader.
// Adding a class means adding its parse here and its field above; the
// container tolerates the key before the reader exists, which is the whole
// point of the container.
var knownPolicyClasses = map[string]bool{"cast": true}

// ParseDeckPolicy parses one deck policy document -- the value of a
// deck.File.Policies entry.
//
// The document is an object of decision-class keys plus the reserved
// "version" key:
//
//	{"version": 1, "cast": { <CastWeights field>: <int32>, ... }}
//
// Strictness is deliberately split. The CLASS keys are permissive (an
// unimplemented class is carried, not rejected) so a deck authored against a
// later build still loads here. The WEIGHTS inside a class this build does
// read are strict -- an unknown knob name is an error, not a silently dropped
// field -- because a typo'd weight name that parsed to zero would look like a
// deliberately-zero weight and quietly un-tune the profile. That is the same
// contract ParseCastProfile applies, pinned against it by
// TestDeckPolicyAndProfileShareCastStrictness.
func ParseDeckPolicy(raw []byte) (PolicySet, error) {
	var doc map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&doc); err != nil {
		return PolicySet{}, fmt.Errorf("botpolicy: deck policy: %w", err)
	}
	// Nothing but whitespace may follow the single JSON value, so a
	// concatenated or truncated document never half-loads.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return PolicySet{}, fmt.Errorf("botpolicy: deck policy: trailing data after the JSON value")
		}
		return PolicySet{}, fmt.Errorf("botpolicy: deck policy: trailing data: %w", err)
	}
	if doc == nil {
		return PolicySet{}, fmt.Errorf("botpolicy: deck policy: null document")
	}

	out := PolicySet{}
	if rawVer, ok := doc["version"]; ok {
		var ver int
		if err := json.Unmarshal(rawVer, &ver); err != nil {
			return PolicySet{}, fmt.Errorf("botpolicy: deck policy: version: %w", err)
		}
		if ver != DeckPolicyVersion {
			return PolicySet{}, fmt.Errorf("botpolicy: deck policy: unsupported version %d (want %d)", ver, DeckPolicyVersion)
		}
	}

	if rawCast, ok := doc["cast"]; ok {
		w, err := decodeCastWeights(rawCast)
		if err != nil {
			return PolicySet{}, fmt.Errorf("botpolicy: deck policy: cast: %w", err)
		}
		out.Cast = w
		out.CastSet = true
	}

	for key := range doc {
		if key == "version" || knownPolicyClasses[key] {
			continue
		}
		out.UnknownClasses = append(out.UnknownClasses, key)
	}
	sort.Strings(out.UnknownClasses)

	if !out.CastSet && len(out.UnknownClasses) == 0 {
		return PolicySet{}, fmt.Errorf("botpolicy: deck policy: document sets no decision class")
	}
	return out, nil
}

// decodeCastWeights is the ONE strict decode of a CastWeights object. Both
// the standalone profile files and the deck-embedded documents route their
// cast object through it, so the two can never drift into disagreeing about
// which weight names exist.
func decodeCastWeights(raw []byte) (CastWeights, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var w CastWeights
	if err := dec.Decode(&w); err != nil {
		return CastWeights{}, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return CastWeights{}, fmt.Errorf("trailing data after the cast object")
		}
		return CastWeights{}, fmt.Errorf("trailing data: %w", err)
	}
	return w, nil
}
