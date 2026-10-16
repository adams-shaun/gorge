package draftpersona_test

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/draftpersona"
	"github.com/adams-shaun/gorge/internal/draftpersona/personatest"
	"github.com/adams-shaun/gorge/limited"
	"github.com/adams-shaun/gorge/limited/pickbot"
)

var names = []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot"}

func pack(set string) pickbot.PickView {
	v := pickbot.PickView{Set: set, Round: 1, PickNo: 1}
	for i, n := range names {
		v.Pack = append(v.Pack, limited.CardRef{ID: n + "#" + string(rune('a'+i)), Name: n})
	}
	return v
}

// A net: spec registers draft-net; on the table's set it answers with the
// network (always a card of the pack), on another set with rating-colour.
func TestRegisterNet(t *testing.T) {
	dir := t.TempDir()
	w, err := personatest.Write(dir, "FDN", names)
	if err != nil {
		t.Fatal(err)
	}
	name, err := draftpersona.Register("net:" + w)
	if err != nil || name != draftpersona.NetPersona {
		t.Fatalf("Register: %q %v", name, err)
	}
	lk := func(string) *cards.Card { return &cards.Card{} }
	d, b, ok := pickbot.New(name, lk)
	if !ok || d == nil || b == nil {
		t.Fatal("draft-net not registered")
	}
	v := pack("FDN")
	got := d.Pick(v, rand.New(rand.NewPCG(1, 1)))
	n, tab, err := draftpersona.LoadNet(w)
	if err != nil {
		t.Fatal(err)
	}
	want := (&pickbot.NetDrafter{Net: n, Table: tab}).Pick(v, rand.New(rand.NewPCG(1, 1)))
	if got != want {
		t.Fatalf("persona picked %q, the net picks %q", got, want)
	}
	o := pack("BLB")
	if g, w := d.Pick(o, rand.New(rand.NewPCG(1, 1))), (&pickbot.RatingColour{Lookup: lk}).Pick(o, rand.New(rand.NewPCG(1, 1))); g != w {
		t.Fatalf("off-set pick %q, rating-colour %q", g, w)
	}
}

func TestRegisterRefusals(t *testing.T) {
	if n, err := draftpersona.Register(""); err != nil || n != "rating-colour" {
		t.Fatalf("default: %q %v", n, err)
	}
	for _, bad := range []string{"bogus", "ovr:x.json", "net:/nonexistent/w.dpkw"} {
		if _, err := draftpersona.Register(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	// Weights against another table are refused (draftnet.CheckTable).
	a, b := t.TempDir(), t.TempDir()
	w, err := personatest.Write(a, "FDN", names)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := personatest.Write(b, "FDN", names[:4]); err != nil {
		t.Fatal(err)
	}
	if _, err := draftpersona.Register("net:" + w + "@" + filepath.Join(b, "table.json")); err == nil || !strings.Contains(err.Error(), "table") {
		t.Fatalf("mismatched table: %v", err)
	}
}

func TestRegisterOverride(t *testing.T) {
	p := filepath.Join(t.TempDir(), "values.json")
	if err := os.WriteFile(p, []byte(`{"Alpha":{"gih":0.4},"Bravo":{"gih":0.7}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	name, err := draftpersona.Register("ovr:" + p + "#gih")
	if err != nil || name != draftpersona.OvrPersona {
		t.Fatalf("Register: %q %v", name, err)
	}
	if _, _, ok := pickbot.New(name, nil); !ok {
		t.Fatal("not registered")
	}
}
