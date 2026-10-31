package build_test

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/build"
)

// TestDeckJSONTagsSnakeCase pins the wire spelling of build.Deck: snake_case
// field names with main omitted when empty.
func TestDeckJSONTagsSnakeCase(t *testing.T) {
	d := build.Deck{Main: []string{"a", "b"}, Basics: [5]int{1, 0, 0, 0, 0}}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != `{"main":["a","b"],"basics":[1,0,0,0,0]}` {
		t.Errorf("marshal = %s", got)
	}
	empty := build.Deck{Basics: [5]int{0, 0, 0, 0, 0}}
	raw, err = json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != `{"basics":[0,0,0,0,0]}` {
		t.Errorf("marshal empty main = %s, want main omitted", got)
	}
}

// TestDeckDecodesLegacySpelling proves the pre-E1-8 wire shape
// (capitalised Main/Basics) still decodes, so stored drafts saved by an
// older binary reopen.
func TestDeckDecodesLegacySpelling(t *testing.T) {
	var d build.Deck
	if err := json.Unmarshal([]byte(`{"Main":["x"],"Basics":[2,0,0,0,0]}`), &d); err != nil {
		t.Fatal(err)
	}
	want := build.Deck{Main: []string{"x"}, Basics: [5]int{2, 0, 0, 0, 0}}
	if d.Main == nil || len(d.Main) != 1 || d.Main[0] != "x" || d.Basics != want.Basics {
		t.Errorf("decoded %+v, want %+v", d, want)
	}
}
