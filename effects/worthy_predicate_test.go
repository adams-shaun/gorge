package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestWorthyPredicateMatchesMjolnirEquipEligibility(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	akiri := corpusObject(t, reg, g, "Akiri, Fearless Voyager")        // legendary, red and white, not a Villain
	villain := corpusObject(t, reg, g, "Killmonger, Ruthless Usurper") // legendary and red, but a Villain
	blue := corpusObject(t, reg, g, "Jace, the Mind Sculptor")         // legendary, but neither red nor white
	nonlegendary := corpusObject(t, reg, g, "Shivan Dragon")           // red, but not legendary

	for _, tc := range []struct {
		name string
		obj  *state.Object
	}{{"Akiri", akiri}, {"Villain", villain}, {"blue", blue}, {"nonlegendary", nonlegendary}} {
		if tc.obj.Zone != state.ZBattlefield {
			t.Fatalf("%s is in zone %v, want battlefield", tc.name, tc.obj.Zone)
		}
	}
	if ColorsOf(akiri) == ColorsOf(blue) || hasType(akiri, "Villain") == hasType(villain, "Villain") {
		t.Fatal("test corpus preconditions do not distinguish eligible, blue, and Villain cases")
	}
	if !hasType(akiri, "Legendary") || hasType(nonlegendary, "Legendary") {
		t.Fatalf("test corpus precondition: want Akiri legendary and Shivan Dragon nonlegendary; types %v / %v", akiri.Face().Types, nonlegendary.Face().Types)
	}

	for _, tc := range []struct {
		name string
		obj  *state.Object
		want bool
	}{
		{"eligible legendary red/white creature", akiri, true},
		{"Villain excluded", villain, false},
		{"neither red nor white", blue, false},
		{"not legendary", nonlegendary, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchesObjectCtx(g, "Creature.YouCtrl+Worthy", tc.obj, SpecContext{You: 0}); got != tc.want {
				t.Errorf("Creature.YouCtrl+Worthy on %s = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
	if got := UnknownPredicates("Creature.YouCtrl+Worthy"); len(got) != 0 {
		t.Fatalf("Worthy remains in UnknownPredicates: %v", got)
	}

	// Confirm the real corpus Equip keyword that consumes this qualifier has
	// the expected Worthy shape, rather than testing an unrelated invented spec.
	mjolnir, ok := reg.Lookup("Mjölnir, Hammer of Thor")
	if !ok {
		t.Fatal("corpus has no Mjölnir, Hammer of Thor")
	}
	found := false
	for _, kw := range mjolnir.Faces[0].Keywords {
		if strings.Contains(kw, "Creature.YouCtrl+Worthy") {
			found = true
		}
	}
	if !found {
		t.Fatal("Mjölnir's real Equip keyword no longer carries Creature.YouCtrl+Worthy")
	}
}
