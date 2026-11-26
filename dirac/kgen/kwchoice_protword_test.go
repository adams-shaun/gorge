package kgen

import (
	"reflect"
	"testing"
)

// TestKwChoiceProtOfExactColourWords pins kwChoiceProtOf (textreach.go):
// only the five colour words are a KWChoice$ Pump term's modelled colour
// word; a term like "Protection from d" or "Protection from e" is no colour
// and scans as the list's rest (it must not pass as "d"/"e" by substring).
func TestKwChoiceProtOfExactColourWords(t *testing.T) {
	for _, tc := range []struct {
		v     string
		words []string
		rest  string
	}{
		{"Protection from red,Protection from white", []string{"red", "white"}, ""},
		{"Protection from d", nil, "Protection from d"},
		{"Protection from e,Protection from green", []string{"green"}, "Protection from e"},
		{"First Strike,Protection from blue", []string{"blue"}, "First Strike"},
	} {
		words, rest, ok := kwChoiceProtOf(tc.v)
		if !reflect.DeepEqual(words, tc.words) || rest != tc.rest || ok != (len(tc.words) != 0) {
			t.Errorf("%q: kwChoiceProtOf = (%q, %q, %v), want (%q, %q, %v)",
				tc.v, words, rest, ok, tc.words, tc.rest, len(tc.words) != 0)
		}
	}
}
