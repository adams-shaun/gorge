package botpolicy

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
)

// The embedded cast profiles: one JSON file per named weight profile,
// keyed by file stem. Every file carries the schema
//
//	{"version": 1, "cast": { <CastWeights field>: <int32>, ... }}
//
// where the cast object's keys are CastWeights' Go field names exactly
// (encoding/json's default field matching, no tags): a profile is a
// straight dump of the struct the cast scorer dots its features with, so
// a tuner can write one by hand and read one back with the same names.
// The loader is strict -- unknown keys anywhere (top level or inside
// cast) are rejected, the version must be present and 1, and the cast
// object itself is required -- so a stale or mis-spelled file fails at
// load instead of silently playing a half-empty profile.
//
// One documented wrinkle (botpolicy/cast.go's castWeights): a Board's
// zero-value CastWeights is treated as DefaultCastWeights, so an
// all-zero cast object in a profile file is NOT expressible -- the
// parsed profile would be indistinguishable from "nobody configured
// anything". The loader therefore requires the cast object to be
// present, but does not police its contents beyond schema; a profile
// that wants a genuinely inert cast scorer must say so with nonzero
// weights, which is the same trade the Board-level zero value already
// made in L1.
//
//go:embed profiles/default.json
var defaultCastProfileJSON []byte

// DefaultCastProfileName is the embedded profile the cast-profile policy
// plays when no file overrides it (botbench's empty -profile, host's
// factory). Its content is pinned equal to DefaultCastWeights by
// profile_test.go -- that equality is what makes cast-profile with the
// default profile intent-identical to bot over a whole game.
const DefaultCastProfileName = "default"

// castProfileFile is the on-disk schema. Cast is a pointer so a file that
// omits the cast object is an error rather than a silent all-zero profile
// (which Board.castWeights would fold back into DefaultCastWeights).
type castProfileFile struct {
	Version int          `json:"version"`
	Cast    *CastWeights `json:"cast"`
}

// ParseCastProfile parses one cast-profile document. Strict by design:
// unknown fields anywhere are rejected (DisallowUnknownFields applies
// recursively), the version must be present and exactly 1, the cast
// object must be present, and trailing data after the JSON value is an
// error -- a concatenated or truncated file never half-loads.
func ParseCastProfile(data []byte) (CastWeights, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f castProfileFile
	if err := dec.Decode(&f); err != nil {
		return CastWeights{}, fmt.Errorf("botpolicy: cast profile: %w", err)
	}
	// Nothing but whitespace may follow the single JSON value.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return CastWeights{}, fmt.Errorf("botpolicy: cast profile: trailing data after the JSON value")
		}
		return CastWeights{}, fmt.Errorf("botpolicy: cast profile: trailing data: %w", err)
	}
	if f.Version != 1 {
		return CastWeights{}, fmt.Errorf("botpolicy: cast profile: unsupported version %d (want 1)", f.Version)
	}
	if f.Cast == nil {
		return CastWeights{}, fmt.Errorf("botpolicy: cast profile: missing \"cast\" object")
	}
	return *f.Cast, nil
}

// LoadCastProfile returns the named embedded profile's weights. The set is
// closed like every other named-vocabulary table here: an unknown name is
// an error naming what exists, never a silent fallback.
func LoadCastProfile(name string) (CastWeights, error) {
	switch name {
	case DefaultCastProfileName:
		w, err := ParseCastProfile(defaultCastProfileJSON)
		if err != nil {
			return CastWeights{}, fmt.Errorf("botpolicy: embedded profile %q: %w", name, err)
		}
		return w, nil
	default:
		return CastWeights{}, fmt.Errorf("botpolicy: unknown cast profile %q (known: %s)", name, DefaultCastProfileName)
	}
}
