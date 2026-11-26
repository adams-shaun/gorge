package cards

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// TestParamKeysSorted enforces that both lists in params.go — the PK const
// block and the paramKeyNames table — are in strictly increasing order by key
// text (bytewise, as sort.Strings), pkNone first. New keys go in their sorted
// position, not at the end: a tail append from two branches always collides,
// while an insert at a different position merges cleanly.
func TestParamKeysSorted(t *testing.T) {
	src, err := os.ReadFile("params.go")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(src), "\n")

	// The PK const block: every declared key except pkNone (fixed first) and
	// paramKeyCount (a count, not a key) must be strictly increasing by key
	// text in source order.
	constStart := -1
	for i, l := range lines {
		if l == "const (" && i+1 < len(lines) && lines[i+1] == "\tpkNone ParamKey = iota" {
			constStart = i
			break
		}
	}
	if constStart < 0 {
		t.Fatal("params.go: PK const block not found (expected `const (` followed by `pkNone ParamKey = iota`)")
	}
	var keys []string
	for i := constStart + 2; i < len(lines); i++ {
		l := lines[i]
		if l == ")" {
			break
		}
		if !strings.HasPrefix(l, "\tPK") || strings.Contains(l, ":") || strings.Contains(l, "=") {
			continue
		}
		keys = append(keys, strings.TrimSpace(l))
	}
	if len(keys) < 100 {
		t.Fatalf("params.go: parsed only %d keys from the const block; parser is broken, not the order", len(keys))
	}
	assertSorted(t, "const block", keys)

	// The paramKeyNames table: the indexed literal's keys must be strictly
	// increasing in source order too, so a new key inserts cleanly.
	tableStart := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "var paramKeyNames = [paramKeyCount]string{") {
			tableStart = i
			break
		}
	}
	if tableStart < 0 {
		t.Fatal("params.go: paramKeyNames table not found")
	}
	var tableKeys []string
	for i := tableStart + 1; i < len(lines); i++ {
		l := lines[i]
		if l == "}" {
			break
		}
		if !strings.HasPrefix(l, "\tPK") {
			continue
		}
		name, _, ok := strings.Cut(strings.TrimSpace(l), ":")
		if !ok {
			continue
		}
		tableKeys = append(tableKeys, name)
	}
	if len(tableKeys) < 100 {
		t.Fatalf("params.go: parsed only %d table keys; parser is broken, not the order", len(tableKeys))
	}
	assertSorted(t, "paramKeyNames table", tableKeys)

	// The two lists describe the same vocabulary.
	if len(keys) != len(tableKeys) {
		t.Fatalf("const block has %d keys but the name table has %d", len(keys), len(tableKeys))
	}
	for i := range keys {
		if keys[i] != tableKeys[i] {
			t.Fatalf("vocabulary mismatch at position %d: const %s vs table %s", i, keys[i], tableKeys[i])
		}
	}
}

func assertSorted(t *testing.T, what string, keys []string) {
	t.Helper()
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	for i := range keys {
		if keys[i] != sorted[i] {
			// Report the first out-of-order pair, not every one.
			t.Errorf("%s: %s should come before %s (key text is not in sorted position)", what, keys[i], sorted[i])
			break
		}
	}
}
