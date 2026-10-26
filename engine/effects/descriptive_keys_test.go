package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects/params"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDescriptiveKeysSorted: IsDescriptiveKey binary-searches the table, so
// the table order is load-bearing.
func TestDescriptiveKeysSorted(t *testing.T) {
	keys := params.DescriptiveKeys[:]
	if !slices.IsSorted(keys) {
		t.Fatalf("params.DescriptiveKeys must be sorted for BinarySearch: %v", keys)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] == keys[i-1] {
			t.Fatalf("params.DescriptiveKeys holds duplicate key %q", keys[i])
		}
	}
	if !params.IsDescriptiveKey(keys[0]) || !params.IsDescriptiveKey(keys[len(keys)-1]) {
		t.Fatalf("IsDescriptiveKey misses the table's own ends: %v", keys)
	}
}

// TestDescriptiveKeyNoNoteChangeZone compiles Heroic Return's real ChangeZone
// body (.cards/cardsfolder/h/heroic_return.txt), whose ValidTgtsDesc$ is a
// caption: it must not enter the unread report, or resolution writes the
// per-API "ignores unread parameter(s)" Note.
func TestDescriptiveKeyNoNoteChangeZone(t *testing.T) {
	if !params.IsDescriptiveKey("ValidTgtsDesc") {
		t.Fatal("precondition: ValidTgtsDesc must be a descriptive key")
	}
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Heroic Return")
	if !ok {
		t.Fatal("corpus pin moved: Heroic Return is missing")
	}
	face := card.Faces[0]
	sa := cards.ResolveSVar(face.SVars, "DBReturn")
	if sa == nil || sa.API != "ChangeZone" {
		t.Fatalf("corpus pin moved: Heroic Return's DBReturn = %+v, want api:ChangeZone", sa)
	}
	if sa.Params["ValidTgtsDesc"] == "" {
		t.Fatal("precondition: DBReturn no longer carries ValidTgtsDesc$")
	}
	if got := ChangeZoneOf(sa).Unread; len(got) != 0 {
		t.Fatalf("Unread = %v, want empty: the ValidTgtsDesc$ caption would emit a Note", got)
	}
}

// TestDescriptiveKeyNoNoteDraw compiles Dewdrop Cure's real GiftAbility Draw
// body (.cards/cardsfolder/d/dewdrop_cure.txt), whose GiftDescription$ is a
// caption: it must not enter the unread report.
func TestDescriptiveKeyNoNoteDraw(t *testing.T) {
	if !params.IsDescriptiveKey("GiftDescription") {
		t.Fatal("precondition: GiftDescription must be a descriptive key")
	}
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Dewdrop Cure")
	if !ok {
		t.Fatal("corpus pin moved: Dewdrop Cure is missing")
	}
	face := card.Faces[0]
	sa := cards.ResolveSVar(face.SVars, "GiftAbility")
	if sa == nil || sa.API != "Draw" {
		t.Fatalf("corpus pin moved: Dewdrop Cure's GiftAbility = %+v, want api:Draw", sa)
	}
	if sa.Params["GiftDescription"] == "" {
		t.Fatal("precondition: GiftAbility no longer carries GiftDescription$")
	}
	if got := DrawOf(sa).Unread; len(got) != 0 {
		t.Fatalf("Unread = %v, want empty: the GiftDescription$ caption would emit a Note", got)
	}
}

// TestDescriptiveKeyRealGapStillNotes: the skip is caption-only. A parameter
// nobody reads must still be reported and still write the loud-degrade Note.
func TestDescriptiveKeyRealGapStillNotes(t *testing.T) {
	if params.IsDescriptiveKey("MadeUpUnreadKey") {
		t.Fatal("precondition: MadeUpUnreadKey must not be descriptive")
	}
	if _, known := slices.BinarySearch(changeZoneKnownKeys[:], "MadeUpUnreadKey"); known {
		t.Fatal("precondition: MadeUpUnreadKey must not be a known ChangeZone key")
	}
	sa := &cards.SA{API: "ChangeZone", Params: map[string]string{
		"Destination": "Hand", "ValidTgtsDesc": "a card", "MadeUpUnreadKey": "x",
	}}
	cz := ChangeZoneOf(sa)
	if !slices.Equal(cz.Unread, []string{"MadeUpUnreadKey"}) {
		t.Fatalf("Unread = %v, want [MadeUpUnreadKey]", cz.Unread)
	}
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Probe\nTypes:Artifact\nOracle:x\n"), 0)
	cz.noteUnread(h, &Ctx{Source: src.ID, Controller: 0})
	want := "ChangeZone ignores unread parameter(s) MadeUpUnreadKey$"
	if notes := ufNotes(h); len(notes) != 1 || notes[0] != want {
		t.Fatalf("notes = %v, want [%q]", notes, want)
	}
}

// descriptiveProbeSA carries every descriptive key plus one made-up key.
func descriptiveProbeSA(api string) *cards.SA {
	ps := map[string]string{"MadeUpUnreadKey": "x"}
	for _, k := range params.DescriptiveKeys {
		ps[k] = "caption"
	}
	return &cards.SA{API: api, Params: ps}
}

// TestDescriptiveKeySkipIgnoresTable proves both effects-side skip sites drop
// every caption on their own: against an empty table, only the made-up key is
// reported, whatever the per-API known-key tables happen to list.
func TestDescriptiveKeySkipIgnoresTable(t *testing.T) {
	sa := descriptiveProbeSA("ChangeZone")
	if len(sa.Params) != len(params.DescriptiveKeys)+1 {
		t.Fatalf("precondition: probe carries %d params, want %d", len(sa.Params), len(params.DescriptiveKeys)+1)
	}
	want := []string{"MadeUpUnreadKey"}
	if got := changeZoneUnreadIn(sa, nil); !slices.Equal(got, want) {
		t.Fatalf("changeZoneUnreadIn(all captions, nil) = %v, want %v", got, want)
	}
	if got := unreadKeys(sa, nil); !slices.Equal(got, want) {
		t.Fatalf("unreadKeys(all captions, nil) = %v, want %v", got, want)
	}
	if got := ChangeZoneOf(sa).Unread; !slices.Equal(got, want) {
		t.Fatalf("ChangeZoneOf(all captions).Unread = %v, want %v", got, want)
	}
}
