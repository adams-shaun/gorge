package kgen

import "testing"

// Each LIR kind has a name in its keyed table: a kind added to an iota block
// without its table entry (or one keyed past the sentinel) fails here rather
// than printing "" or panicking in a dump.
func TestLIRKindNamesComplete(t *testing.T) {
	check := func(table string, names []string, n int) {
		if len(names) != n {
			t.Errorf("%s has %d entries, want %d (one per kind before the sentinel)", table, len(names), n)
		}
		for i, s := range names {
			if s == "" {
				t.Errorf("%s[%d] has no name", table, i)
			}
		}
	}
	check("refNames", refNames[:], int(rCount))
	check("valNames", valNames[:], int(vCount_))
	check("condNames", condNames[:], int(cCount_))
}
