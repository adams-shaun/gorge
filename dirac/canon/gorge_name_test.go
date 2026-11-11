package canon_test

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestGorgeNameFoldsRoomAndAccent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, c := range []struct{ in, want string }{
		{"Dain's Company", "Dáin's Company"},
		{"Unholy Annex // Ritual Chamber", "Unholy Annex"},
		{"Grizzly Bears", "Grizzly Bears"},
	} {
		differs := c.in != c.want
		if got := canon.GorgeName(reg, c.in); got != c.want {
			t.Errorf("GorgeName(%q) = %q, want %q", c.in, got, c.want)
		}
		if c.in == "Grizzly Bears" && differs {
			t.Errorf("control must be the identity")
		}
		if c.in != "Grizzly Bears" && !differs {
			t.Errorf("precondition: %q must differ from its printed name", c.in)
		}
	}
	if got := canon.GorgeName(nil, "Dain's Company"); got != "Dain's Company" {
		t.Errorf("nil registry changed the name: %q", got)
	}
}

func TestBindSetNamesGorgeFront(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	set := cardsettest.Set(t, "fdn")
	tbl := &kop.Table{Cards: []kop.CardDef{{Name: "Dain's Company"}, {Name: "Unholy Annex // Ritual Chamber"}}}
	want := []string{"Dáin's Company", "Unholy Annex"}
	for i, c := range tbl.Cards {
		if c.Name == want[i] {
			t.Fatalf("precondition: table name %q equals the printed name", c.Name)
		}
	}
	var tr canon.Translator
	tr.BindSet(set, tbl, reg)
	for i := range want {
		if tr.GName[i] != want[i] {
			t.Errorf("GName[%d] = %q, want %q", i, tr.GName[i], want[i])
		}
	}
}
