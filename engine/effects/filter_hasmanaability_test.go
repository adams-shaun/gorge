package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestHasManaAbilityPredicate is the effects leaf for Forge's
// Card.hasManaAbility filter property. Power Sink's TapAll and Moonsilver
// Key's ChangeZone are the positive corpus carriers; Midnight Arsonist's
// `Artifact.!hasManaAbility` is the negated one. Every case uses a real
// corpus card and first proves the face really has (or lacks) the mana
// ability the result turns on, so it cannot pass vacuously.
func TestHasManaAbilityPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	forest := corpusObject(t, reg, g, "Forest")        // Basic Land: intrinsic AB:Mana
	elves := corpusObject(t, reg, g, "Llanowar Elves") // Creature: printed AB:Mana
	solRing := corpusObject(t, reg, g, "Sol Ring")     // Artifact: printed AB:Mana
	bears := corpusObject(t, reg, g, "Grizzly Bears")  // Creature: no abilities
	sword := corpusObject(t, reg, g, "Bonesplitter")   // Artifact: no mana ability

	for _, tc := range []struct {
		name  string
		obj   *state.Object
		manas bool
	}{
		{"Forest (intrinsic basic-land mana ability)", forest, true},
		{"Llanowar Elves (printed activated mana ability)", elves, true},
		{"Sol Ring (printed activated mana ability)", solRing, true},
		{"Grizzly Bears (no ability)", bears, false},
		{"Bonesplitter (artifact without a mana ability)", sword, false},
	} {
		f := tc.obj.Face()
		if f == nil {
			t.Fatalf("%s: no face", tc.name)
		}
		if got := len(f.ManaAbilities()) != 0; got != tc.manas {
			t.Fatalf("precondition: %s FaceHasManaAbility=%v, want %v (abilities %+v)", tc.name, got, tc.manas, f.Abilities)
		}
		if got := FaceHasManaAbility(f); got != tc.manas {
			t.Errorf("FaceHasManaAbility(%s) = %v, want %v", tc.name, got, tc.manas)
		}
		if got := MatchesObjectCtx(g, "Card.hasManaAbility", tc.obj, SpecContext{You: 0}); got != tc.manas {
			t.Errorf("Card.hasManaAbility(%s) = %v, want %v", tc.name, got, tc.manas)
		}
	}

	// The class's actual filters: a land with a mana ability, and an
	// artifact with one or a basic land.
	if !MatchesObjectCtx(g, "Land.hasManaAbility", forest, SpecContext{You: 0}) {
		t.Error("Land.hasManaAbility must match Forest")
	}
	if !MatchesObjectCtx(g, "Artifact.hasManaAbility", solRing, SpecContext{You: 0}) {
		t.Error("Artifact.hasManaAbility must match Sol Ring")
	}
	if MatchesObjectCtx(g, "Artifact.hasManaAbility", sword, SpecContext{You: 0}) {
		t.Error("Artifact.hasManaAbility must not match Bonesplitter")
	}
	if !MatchesObjectCtx(g, "Artifact.!hasManaAbility", sword, SpecContext{You: 0}) {
		t.Error("Artifact.!hasManaAbility must match Bonesplitter")
	}
	if MatchesObjectCtx(g, "Artifact.!hasManaAbility", solRing, SpecContext{You: 0}) {
		t.Error("Artifact.!hasManaAbility must not match Sol Ring")
	}
	if !MatchesObjectCtx(g, "Artifact.hasManaAbility,Land.Basic", forest, SpecContext{You: 0}) {
		t.Error("Artifact.hasManaAbility,Land.Basic must match a basic land")
	}

	// Recognition: the census must not report a token the matcher evaluates,
	// in either polarity.
	for _, spec := range []string{"Land.hasManaAbility", "Artifact.hasManaAbility,Land.Basic", "Artifact.!hasManaAbility"} {
		if got := UnknownPredicates(spec); len(got) != 0 {
			t.Fatalf("UnknownPredicates(%q) = %v, want empty", spec, got)
		}
	}
	// The fail-closed contract is unchanged for a genuinely unknown token.
	if got := UnknownPredicates("Land.someMechanicWeDoNotModel"); len(got) != 1 {
		t.Fatalf("UnknownPredicates(Land.someMechanicWeDoNotModel) = %v, want one token", got)
	}
}

// TestHasManaAbilityFaceDown pins CR 708.2: a face-down permanent has no
// abilities, so it has no mana ability.
func TestHasManaAbilityFaceDown(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	forest := corpusObject(t, reg, g, "Forest")
	forest.FaceDown = true
	forest.Zone = state.ZBattlefield
	if FaceHasManaAbility(forest.Face()) == false {
		t.Fatal("precondition: Forest's printed face must carry a mana ability")
	}
	if MatchesObjectCtx(g, "Land.hasManaAbility", forest, SpecContext{You: 0}) {
		t.Error("a face-down permanent must not match Land.hasManaAbility (CR 708.2)")
	}
	forest.FaceDown = false
	if !MatchesObjectCtx(g, "Land.hasManaAbility", forest, SpecContext{You: 0}) {
		t.Error("the same Forest must match once face up")
	}
}
