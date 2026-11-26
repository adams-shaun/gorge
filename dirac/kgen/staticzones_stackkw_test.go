package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// staticZonesStackKwCarriers are the AffectedZone$ Stack keyword grants whose
// keyword gorge reads off the derived keyword list at the cast (rebound at
// the resolution exit, ripple at the cast trigger walk, web-slinging and
// offspring at the cast offer): they lower to Kind "StackKw".
var staticZonesStackKwCarriers = []struct {
	card string
	face int // the face that carries the static (Peter Parker's back face, Amazing Spider-Man)
}{
	{"Cast Through Time", 0}, {"Peter Parker", 1}, {"Zinnia, Valley's Voice", 0},
}

// lowerNamedFace lowers face i of the named corpus card (i > 0: the face
// entry a multi-face card's pool carries beside it).
func lowerNamedFace(t *testing.T, name string, i int) *lCard {
	t.Helper()
	if i == 0 {
		return lowerNamedCard(t, name)
	}
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	return lowerFaceEntry(p, faceKey(card.Faces[0].Name, i))
}

// staticZonesStackKwFaceOnly are the grants of a keyword gorge reads only off
// a printed face (replicateCost, the Prowl and Freerunning alternative modes,
// kwUndaunted): a grant gorge never reads is inert there, so lowering it
// would let the kernel act on a keyword gorge ignores. Each keeps its named
// refusal until a gorge ticket teaches that reader the derived list.
var staticZonesStackKwFaceOnly = []struct{ card, want string }{
	{"Djinn Illuminatus", `AffectedZone Stack grant of "Replicate:CardManaCost"`},
	{"Hatchery Sliver", `AffectedZone Stack grant of "Replicate`},
	{"Hugh, First of Many", `AffectedZone Stack grant of "Replicate`},
	{"Ian Chesterton", `AffectedZone Stack grant of "Replicate`},
	{"Threefold Signal", `AffectedZone Stack grant of "Replicate`},
	{"Hunting Velociraptor", `AffectedZone Stack grant of "Prowl`},
	{"Ezio Auditore da Firenze", `AffectedZone Stack grant of "Freerunning`},
	{"Vox, Child of the Collective", `AffectedZone Stack grant of "Undaunted`},
}

func TestStaticZonesStackKeywordForms(t *testing.T) {
	for _, cr := range staticZonesStackKwCarriers {
		name := cr.card
		if front := lowerNamedCard(t, name); len(front.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, front.Refusals)
		}
		c := lowerNamedFace(t, name, cr.face)
		if len(c.Refusals) != 0 {
			t.Errorf("%s face %d refusals = %v, want none", name, cr.face, c.Refusals)
		}
		n := 0
		for _, st := range c.Statics {
			if st.Kind == "StackKw" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s lowers %d StackKw statics, want 1", name, n)
		}
	}
	for _, r := range staticZonesStackKwFaceOnly {
		c := lowerNamedCard(t, r.card)
		hit := false
		for _, rf := range c.Refusals {
			hit = hit || strings.Contains(rf.Reason, r.want)
		}
		if !hit {
			t.Errorf("%s refusals %v: none carries %q", r.card, c.Refusals, r.want)
		}
	}
}
