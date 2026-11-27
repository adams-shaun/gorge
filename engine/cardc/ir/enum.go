package ir

import (
	"encoding/json"
	"fmt"
)

// Every enum in this package follows one shape: a uintN type whose zero value
// is XxxNone ("absent"), consts ending in an unexported xxxCount sentinel, a
// name table indexed by const, String, ParseXxx, and JSON by name. Names, not
// ordinals, are what the codec writes, so appending a const never changes an
// old encoding.

type enumInt interface{ ~uint8 | ~uint16 }

// enumString returns the table name of v, or label(v) for a value with no name.
func enumString[T enumInt](names []string, v T, label string) string {
	if int(v) < len(names) && names[v] != "" {
		return names[v]
	}
	return fmt.Sprintf("%s(%d)", label, uint16(v))
}

// parseEnum finds the value whose table name is s.
func parseEnum[T enumInt](names []string, s string) (T, bool) {
	if s == "" {
		return 0, false
	}
	for i, n := range names {
		if n == s {
			return T(i), true
		}
	}
	return 0, false
}

// unmarshalEnum decodes the JSON string b through parse, naming the bad name
// in its error.
func unmarshalEnum[T enumInt](dst *T, b []byte, label string, parse func(string) (T, bool)) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("ir: %s must be a JSON string: %w", label, err)
	}
	v, ok := parse(s)
	if !ok {
		return fmt.Errorf("ir: unknown %s name %q", label, s)
	}
	*dst = v
	return nil
}
