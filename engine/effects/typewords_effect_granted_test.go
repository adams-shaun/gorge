package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTypeWordVocabularyKnowsEffectGrantedTypes pins the effect-granted arm
// of the filter's type-word vocabulary. The corpus grants type words through
// ability `Types$`/`AddTypes$` parameters that no card's or token's Types
// line carries, so the vocabulary built from Types lines alone misses them
// and a filter naming such a word classified the qualifier unknown and failed
// closed: Blinkmoth Nexus's own pump ({1},{T}: Target Blinkmoth creature
// gets +1/+1) found zero candidates for `Creature.Blinkmoth` while the
// animated Nexus (CR 613.1f: the Animate's Types$ grant) WAS a Blinkmoth
// creature, and the option vanished from the offer.
//
// The three words a live object can carry through an effect are pinned here:
//   - Blinkmoth (Blinkmoth Nexus, Inkmoth Nexus -- and the only one the
//     corpus's filter grammar names, ValidTgts$ Creature.Blinkmoth),
//   - Llama (Restless Prairie's animate),
//   - Beserker (Ursine Champion's Forge-misspelled DBAnimate Types$).
func TestTypeWordVocabularyKnowsEffectGrantedTypes(t *testing.T) {
	for _, w := range []string{"Blinkmoth", "Llama", "Beserker"} {
		if !predicateTypeWords[w] {
			t.Errorf("predicateTypeWords must know the effect-granted type word %q", w)
		}
		if un := UnknownPredicates("Creature." + w); len(un) != 0 {
			t.Errorf("UnknownPredicates(Creature.%s) = %v, want empty (the word is a known type)", w, un)
		}
	}

	// The matcher end: with the word classified, the qualifier reads the
	// object's layer-4 derived types, so an animated manland is a legal
	// candidate and a plain one is not. Blinkmoth Nexus is the corpus card
	// whose offer the lockstep game lost.
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	nexus := corpusObject(t, reg, g, "Blinkmoth Nexus")
	if nexus.Zone != state.ZBattlefield || nexus.Controller != 0 {
		t.Fatal("precondition: the fixture must be a controlled battlefield object")
	}
	if hasType(nexus, "Blinkmoth") {
		t.Fatal("precondition: the printed Nexus is not a Blinkmoth creature; the word comes from the animate grant")
	}
	derived := []ObjectTypes{{ID: nexus.ID, Types: []string{"Land", "Creature", "Artifact", "Blinkmoth"}}}
	sc := SpecContext{You: 0, Layers: LayerTables{DerivedTypes: derived}}
	if !MatchesObjectCtx(g, "Creature.Blinkmoth", nexus, sc) {
		t.Error("Creature.Blinkmoth must match the animated Nexus (derived Blinkmoth, CR 613.1f)")
	}
	plain := SpecContext{You: 0}
	if MatchesObjectCtx(g, "Creature.Blinkmoth", nexus, plain) {
		t.Error("Creature.Blinkmoth must not match the unanimated Nexus")
	}
	if MatchesObjectCtx(g, "Creature.Llama", nexus, sc) {
		t.Error("Creature.Llama must not match a Blinkmoth creature")
	}
}
