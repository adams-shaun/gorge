package events_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCopyExceptionsAbilitiesRoundTrip: an abilities-only payload (a Mercurial
// Pretender copy's whole exception) round-trips and is not Empty, or the
// emitter would drop its CopyExcept event.
func TestCopyExceptionsAbilitiesRoundTrip(t *testing.T) {
	x := events.CopyExceptions{Abilities: [][2]string{{"MercurialBounce", "AB$ ChangeZone | Cost$ 2 U U"}}}
	got, ok := events.ParseCopyExceptions(x.Text())
	if !ok || !reflect.DeepEqual(got, x) {
		t.Fatalf("round trip %+v -> %q -> %+v (ok %v)", x, x.Text(), got, ok)
	}
	if x.Empty() {
		t.Fatal("an abilities-only payload reads Empty")
	}
	if (events.CopyExceptions{}).Text() != "{}" {
		t.Fatalf("an empty payload is %q: omitempty must keep existing payloads byte-identical", (events.CopyExceptions{}).Text())
	}
}

// TestFoldCopyExceptAbilities: the fold appends the resolved AB to the copy's
// face and its SVar table, never to the shared corpus face it value-copied.
func TestFoldCopyExceptAbilities(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bear, ok := reg.Lookup("Grizzly Bears")
	pretender, ok2 := reg.Lookup("Mercurial Pretender")
	effigy, ok3 := reg.Lookup("Machine God's Effigy")
	if !ok || !ok2 || !ok3 {
		t.Skip("corpus lacks Grizzly Bears, Mercurial Pretender or Machine God's Effigy")
	}
	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(bear, 0)
	o.Zone = state.ZBattlefield
	src := o.Face()
	before := len(src.Abilities)
	body := pretender.Faces[0].SVars["MercurialBounce"]
	if body == "" {
		t.Fatal("Mercurial Pretender has no MercurialBounce SVar")
	}
	if _, has := src.SVars["MercurialBounce"]; has || before != 0 {
		t.Fatalf("precondition: the bear already has the ability (%d abilities)", before)
	}
	x := events.CopyExceptions{Abilities: [][2]string{{"MercurialBounce", body}}}
	events.Apply(g, events.Event{Kind: events.CopyExcept, Obj: o.ID, Text: x.Text()})
	f := o.Face()
	if len(f.Abilities) != before+1 || f.Abilities[before].Kind != "AB" || f.Abilities[before].API != "ChangeZone" {
		t.Fatalf("copy abilities %+v, want the one ChangeZone AB appended", f.Abilities)
	}
	if f.SVars["MercurialBounce"] != body {
		t.Fatalf("copy SVars %v lack the grant", f.SVars)
	}
	if len(src.Abilities) != before || len(src.SVars) != 0 {
		t.Fatalf("the shared corpus face was mutated: %d abilities, SVars %v", len(src.Abilities), src.SVars)
	}

	// A mana ability's production summary is refreshed on the copy.
	m := g.AddObject(bear, 0)
	m.Zone = state.ZBattlefield
	if !m.Face().ManaProduction().IsZero() {
		t.Fatal("precondition: the bear produces mana")
	}
	mb := effigy.Faces[0].SVars["ABMana"]
	if mb == "" {
		t.Fatal("Machine God's Effigy has no ABMana SVar")
	}
	mx := events.CopyExceptions{Abilities: [][2]string{{"ABMana", mb}}}
	events.Apply(g, events.Event{Kind: events.CopyExcept, Obj: m.ID, Text: mx.Text()})
	if m.Face().ManaProduction().IsZero() {
		t.Fatal("the copy's mana production summary is stale after a mana ability grant")
	}
	if !bear.Faces[0].ManaProduction().IsZero() {
		t.Fatal("the corpus face's mana production changed")
	}
}

// TestFoldCopyExceptAbilitiesOnceAndMarked: a copy exception whose ability the
// copy's face already has (a copy of a copy of the same exception, CR 707.3)
// adds nothing; the appended ability is marked as a copy's grant, and a
// printed A: line is not, though its text is the body of an SVar of its face
// (Machine God's Effigy).
func TestFoldCopyExceptAbilitiesOnceAndMarked(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bear, ok := reg.Lookup("Grizzly Bears")
	pretender, ok2 := reg.Lookup("Mercurial Pretender")
	effigy, ok3 := reg.Lookup("Machine God's Effigy")
	if !ok || !ok2 || !ok3 {
		t.Skip("corpus lacks Grizzly Bears, Mercurial Pretender or Machine God's Effigy")
	}
	ef := effigy.Faces[0]
	coincides := false
	for _, sa := range ef.Abilities {
		for _, body := range ef.SVars {
			coincides = coincides || sa.Line != "" && strings.TrimSpace(body) == strings.TrimSpace(sa.Line)
		}
		if sa.IsCopyGranted() {
			t.Fatalf("a printed ability of %s is marked as a copy's grant", ef.Name)
		}
	}
	if !coincides {
		t.Fatal("precondition: no printed ability of Machine God's Effigy has the text of an SVar body")
	}
	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(bear, 0)
	o.Zone = state.ZBattlefield
	x := events.CopyExceptions{Abilities: [][2]string{{"MercurialBounce", pretender.Faces[0].SVars["MercurialBounce"]}}}
	events.Apply(g, events.Event{Kind: events.CopyExcept, Obj: o.ID, Text: x.Text()})
	if len(o.Face().Abilities) != 1 || !o.Face().Abilities[0].IsCopyGranted() {
		t.Fatalf("after one exception the face has %d abilities (marked %v), want one marked", len(o.Face().Abilities), len(o.Face().Abilities) == 1 && o.Face().Abilities[0].IsCopyGranted())
	}
	events.Apply(g, events.Event{Kind: events.CopyExcept, Obj: o.ID, Text: x.Text()})
	if n := len(o.Face().Abilities); n != 1 {
		t.Fatalf("the same exception twice left %d abilities, want 1", n)
	}
}
