package cards

import "testing"

func TestSpliceKeywordAccessor(t *testing.T) {
	for _, tc := range []struct {
		line, quality, cost string
	}{
		{"Splice:Arcane:G", "Arcane", "G"},
		{"Splice:Instant,Sorcery:2 U", "Instant,Sorcery", "2 U"},
		{"Splice:Arcane:Sac<2/Mountain>", "Arcane", "Sac<2/Mountain>"},
	} {
		f := &Face{Keywords: []string{tc.line}}
		q, c, ok := f.Splice()
		if !ok || q != tc.quality || c != tc.cost {
			t.Errorf("%q: Splice() = %q, %q, %v; want %q, %q, true", tc.line, q, c, ok, tc.quality, tc.cost)
		}
	}
	if _, _, ok := (&Face{Keywords: []string{"Flash"}}).Splice(); ok {
		t.Error("a face without the line reported a splice")
	}
	if _, _, ok := (&Face{}).Splice(); ok {
		t.Error("a keywordless face reported a splice")
	}
	f := &Face{Types: []string{"Instant", "Arcane"}}
	if !f.HasTypeWord("Arcane") || f.HasTypeWord("Sorcery") {
		t.Error("HasTypeWord does not read the type line's subtypes")
	}
}
