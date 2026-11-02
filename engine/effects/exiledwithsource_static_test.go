package effects

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMoveZoneEventStampsStaticOnlyExiledWithSource pins the S:-line spelling
// of the provenance need on the shared exile constructor: a source whose ONLY
// ExiledWithSource mention is a static's Affected$ (Maralen, Fae Ascendant,
// exiling through a Dig whose SVar never names it) stamps its exile moves.
func TestMoveZoneEventStampsStaticOnlyExiledWithSource(t *testing.T) {
	static := mkCard(t, "Name:Static Only\nTypes:Creature\nPT:1/1\nOracle:x\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigExile | TriggerDescription$ x\n"+
		"SVar:TrigExile:DB$ Dig | DigNum$ 2 | ChangeNum$ All | DestinationZone$ Exile\n"+
		"S:Mode$ Continuous | Affected$ Card.nonLand+ExiledWithSource | MayPlay$ True | AffectedZone$ Exile | Description$ x\n")
	plain := mkCard(t, "Name:Plain Dig\nTypes:Creature\nPT:1/1\nOracle:x\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigExile | TriggerDescription$ x\n"+
		"SVar:TrigExile:DB$ Dig | DigNum$ 2 | ChangeNum$ All | DestinationZone$ Exile\n")
	f := static.Faces[0]
	if len(f.Statics) != 1 || !strings.Contains(f.Statics[0].Params["Affected"], "ExiledWithSource") {
		t.Fatalf("precondition: static = %+v, want one Affected$ naming ExiledWithSource", f.Statics)
	}
	for k, v := range f.SVars {
		if strings.Contains(v, "ExiledWithSource") {
			t.Fatalf("precondition: SVar %s names ExiledWithSource, so the SVar arm would cover it", k)
		}
	}
	h := newHost(t, 2)
	src := h.g.AddObject(static, 0)
	other := h.g.AddObject(plain, 0)
	if ev := moveZoneEvent(&Ctx{Source: src.ID, Host: h, SVars: f.SVars}, 99, state.ZLibrary, state.ZExile); len(ev.IDs) != 1 || ev.IDs[0] != src.ID {
		t.Fatalf("static-only exile event IDs = %v, want [%d]", ev.IDs, src.ID)
	}
	if ev := moveZoneEvent(&Ctx{Source: other.ID, Host: h, SVars: plain.Faces[0].SVars}, 99, state.ZLibrary, state.ZExile); len(ev.IDs) != 0 {
		t.Fatalf("a source naming no ExiledWithSource stamped %v", ev.IDs)
	}
}

// exiledWithSourceUncovered is every corpus card whose front face names
// ExiledWithSource somewhere in its script yet whose exiles
// exileProvenanceNeeded does NOT stamp: the name lives only on a T: or R:
// line (or a static key faceStaticsNameExiledWithSource does not read). A
// ratchet: the list only shrinks. A new entry means a card whose "exiled
// with" association the engine silently drops.
var exiledWithSourceUncovered = []string{
	"Cemetery Protector (T)",
	"Chrome Mox (T)",
	"Circle of Confinement (T)",
	"Drach'Nyen (T)",
	"Exclusion Ritual (T)",
	"Extraplanar Lens (T)",
	"Ice Cauldron (T)",
	"Invader Parasite (T)",
	"Mirror Golem (T)",
	"Phyrexian Ingester (T)",
	"Prowl, Stoic Strategist (T)",
	"Rasaad, Monk of Selûne (T)",
	"Semblance Anvil (T)",
	"Spellbinder (T)",
	"Strata Scythe (T)",
	"Thought Prison (T)",
	"Voltaic Visionary (T)",
}

// TestExiledWithSourceProvenanceCensus walks the corpus and checks that every
// front face naming ExiledWithSource gets its exile moves stamped, except the
// pinned uncovered remainder. The Dig + S:-only carriers (Maralen, Fae
// Ascendant's shape) are listed in the log; before the S: arm joined
// exileProvenanceNeeded they all sat in the uncovered list.
func TestExiledWithSourceProvenanceCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	h := newHost(t, 2)
	named, staticDig := 0, []string{}
	var uncovered []string
	for _, card := range reg.AllCards() {
		if len(card.Faces) == 0 {
			continue
		}
		f := card.Faces[0]
		where := exiledWithSourceMentions(f)
		if where == "" {
			continue
		}
		named++
		o := h.g.AddObject(card, 0)
		if !exileProvenanceNeeded(&Ctx{Source: o.ID, Host: h, SVars: f.SVars}) {
			uncovered = append(uncovered, f.Name+" ("+where+")")
			continue
		}
		if where == "S" && faceHasExileDig(f) {
			staticDig = append(staticDig, f.Name)
		}
	}
	if named < 100 {
		t.Fatalf("census saw only %d faces naming ExiledWithSource: the scan is not reading the corpus", named)
	}
	sort.Strings(uncovered)
	sort.Strings(staticDig)
	if !containsString(staticDig, "Maralen, Fae Ascendant") {
		t.Errorf("Maralen, Fae Ascendant is not a covered S:-only Dig carrier: %v", staticDig)
	}
	want := map[string]bool{}
	for _, n := range exiledWithSourceUncovered {
		want[n] = true
	}
	got := map[string]bool{}
	for _, n := range uncovered {
		got[n] = true
		if !want[n] {
			t.Errorf("new uncovered ExiledWithSource carrier: %s", n)
		}
	}
	for _, n := range exiledWithSourceUncovered {
		if !got[n] {
			t.Errorf("pinned uncovered carrier %s is now covered: delete it from exiledWithSourceUncovered", n)
		}
	}
	t.Logf("%d faces name ExiledWithSource; %d uncovered; %d S:-only Dig carriers: %v", named, len(uncovered), len(staticDig), staticDig)
}

// exiledWithSourceMentions names which script line kinds of f mention
// ExiledWithSource, joined in a fixed order ("SVar", "A", "S", "T", "R"), or
// "" when none does. Map ranges only feed booleans.
func exiledWithSourceMentions(f *cards.Face) string {
	has := func(m map[string]string) bool {
		for _, v := range m {
			if strings.Contains(v, "ExiledWithSource") {
				return true
			}
		}
		return false
	}
	var kinds []string
	if has(f.SVars) {
		kinds = append(kinds, "SVar")
	}
	for _, sa := range f.Abilities {
		if sa != nil && has(sa.Params) {
			kinds = append(kinds, "A")
			break
		}
	}
	for _, st := range f.Statics {
		if has(st.Params) {
			kinds = append(kinds, "S")
			break
		}
	}
	for _, tr := range f.Triggers {
		if has(tr.Params) {
			kinds = append(kinds, "T")
			break
		}
	}
	for _, r := range f.Repls {
		if has(r.Params) {
			kinds = append(kinds, "R")
			break
		}
	}
	return strings.Join(kinds, "+")
}

// faceHasExileDig reports whether any SVar body of f is a Dig whose primary
// destination is exile.
func faceHasExileDig(f *cards.Face) bool {
	for _, v := range f.SVars {
		if strings.Contains(v, "DB$ Dig") && strings.Contains(v, "DestinationZone$ Exile") {
			return true
		}
	}
	return false
}
