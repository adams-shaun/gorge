// Lifted from github.com/adams-shaun/gorge-compliance, oracle/mtgish/node.go at
// commit 285a7038 (same owner, Apache-2.0). Changes: gameNumber dropped
// (summary-only).

package mtgish

import (
	"sort"
	"strings"
)

// mtgish.lines.json encodes every Rust enum as a tagged union: an object
// carrying exactly one "_<Enum>" key whose string value names the variant,
// plus either an "args" value (a tuple variant: one value, or an array for
// two or more fields) or named fields (a struct variant, PascalCase). The
// helpers below read that shape generically, so the loader never needs Go
// types for mtgish's ~350 enums: a normalizer asks for the variant of a node
// and the positional arguments it carries.

// node is one decoded JSON object.
type node = map[string]any

// tagged reports the enum key ("_Action") and variant name ("DrawACard") of
// x, or ok=false when x is not a tagged-union node.
func tagged(x any) (enum, variant string, n node, ok bool) {
	m, isMap := x.(map[string]any)
	if !isMap {
		return "", "", nil, false
	}
	// The tag key is the one key starting with '_'; map iteration order does
	// not matter because there is exactly one such key per node.
	for k, v := range m {
		if strings.HasPrefix(k, "_") {
			if s, isStr := v.(string); isStr {
				return k, s, m, true
			}
		}
	}
	return "", "", m, false
}

// variantOf is tagged's variant name, "" for a non-node.
func variantOf(x any) string {
	_, v, _, _ := tagged(x)
	return v
}

// enumOf is tagged's enum key without the leading underscore ("Action").
func enumOf(x any) string {
	e, _, _, _ := tagged(x)
	return strings.TrimPrefix(e, "_")
}

// qualified is "Enum.Variant", the spelling an unmapped token carries.
func qualified(x any) string {
	e, v, _, ok := tagged(x)
	if !ok {
		return "?"
	}
	return strings.TrimPrefix(e, "_") + "." + v
}

// argsOf returns a tuple variant's positional arguments. A variant whose
// single field is itself a Vec serialises as an array too, so callers that
// know the arity use argN/argVec instead of trusting the length.
func argsOf(x any) []any {
	_, _, n, ok := tagged(x)
	if !ok {
		return nil
	}
	a, has := n["args"]
	if !has {
		return nil
	}
	if l, isList := a.([]any); isList {
		return l
	}
	return []any{a}
}

// rawArgs is the "args" value as decoded (nil when absent).
func rawArgs(x any) any {
	_, _, n, ok := tagged(x)
	if !ok {
		return nil
	}
	return n["args"]
}

// argN is the i'th positional argument of a multi-field tuple variant.
func argN(x any, i int) any {
	a := argsOf(x)
	if i < len(a) {
		return a[i]
	}
	return nil
}

// list reads x as a JSON array (nil otherwise).
func list(x any) []any {
	l, _ := x.([]any)
	return l
}

// field reads a struct variant's named field.
func field(x any, name string) any {
	m, _ := x.(map[string]any)
	if m == nil {
		return nil
	}
	return m[name]
}

// intOf reads a JSON number as an int.
func intOf(x any) (int, bool) {
	switch v := x.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	}
	return 0, false
}

// walk visits x depth-first in document order. fn sees every tagged node;
// returning false stops the descent into that node's children.
func walk(x any, fn func(n node, enum, variant string) bool) {
	switch v := x.(type) {
	case []any:
		for _, e := range v {
			walk(e, fn)
		}
	case map[string]any:
		enum, variant, n, ok := tagged(v)
		if ok && !fn(n, enum, variant) {
			return
		}
		// Children in sorted key order: deterministic, and map order never
		// reaches an output.
		keys := make([]string, 0, len(v))
		for k := range v {
			if !strings.HasPrefix(k, "_") {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			walk(v[k], fn)
		}
	}
}
