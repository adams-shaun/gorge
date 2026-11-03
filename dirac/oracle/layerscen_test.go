package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// The CR 613 layer scenarios, played by gorge's own
// oracle scenario runner (engine/rules RunOracleScenarioJSON) on the real
// cards whose shapes kgen lowers. Each scenario's expectations are gorge's
// derived characteristics (the run fails on any mismatch), and the kernel
// mirrors in dirac/static_layerguards_test.go assert the same values on a
// synthetic table carrying the same shapes, so neither side's expected
// answer is hand-computed against the other.

var (
	scenOnce sync.Once
	scenReg  *cards.Registry
	scenErr  string
)

func scenRegistry(t *testing.T) *cards.Registry {
	t.Helper()
	scenOnce.Do(func() {
		dir, err := cardset.CorpusDir()
		if err != nil {
			scenErr = err.Error()
			return
		}
		if _, err := os.Stat(filepath.Join(dir, "cardsfolder")); err != nil {
			scenErr = "no gorge corpus at " + dir
			return
		}
		reg, err := cards.SharedCorpus(dir)
		if err != nil {
			scenErr = err.Error()
			return
		}
		scenReg = reg
	})
	if scenReg == nil {
		t.Skip(scenErr)
	}
	return scenReg
}

type lsStep = map[string]any

// layerScen is one two-seat scenario: p0's battlefield and hand, the steps
// (each cast or activation resolves at once), and the final expectations.
func layerScen(name string, bf, hand []string, steps []lsStep, expect []lsStep) []byte {
	var st []lsStep
	for _, s := range steps {
		st = append(st, s, lsStep{"op": "resolve", "seat": 0})
	}
	b, _ := json.Marshal(map[string]any{
		"name": name, "cr": []string{"613"}, "why": "CR 613 layer scenario",
		"setup":  map[string]any{"p0": map[string]any{"battlefield": bf, "hand": hand}},
		"steps":  st,
		"expect": expect,
	})
	return b
}

func runLayerScen(t *testing.T, sc []byte) {
	t.Helper()
	res, err := rules.RunOracleScenarioJSON(scenRegistry(t), sc)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Errorf("gorge disagrees: %v\n%s", res.Fails, sc)
		for _, l := range res.Transcript {
			t.Log(l)
		}
	}
}

func cast(card, mana string, tgt ...string) lsStep {
	return lsStep{"op": "cast", "seat": 0, "card": "p0:" + card, "mana": mana, "targets": prefixed(tgt)}
}

func activate(card, mana string, tgt ...string) lsStep {
	return lsStep{"op": "activate", "seat": 0, "card": "p0:" + card, "mana": mana, "targets": prefixed(tgt)}
}

func prefixed(s []string) []string {
	var out []string
	for _, x := range s {
		out = append(out, "p0:"+x)
	}
	return out
}

func expectOf(card string, kv ...any) lsStep {
	e := lsStep{"card": "p0:" + card}
	for i := 0; i+1 < len(kv); i += 2 {
		e[kv[i].(string)] = kv[i+1]
	}
	return e
}

// TestL7GorgeSubtypeGrantVsRecordSetter (layer 4, a static subtype grant
// and a one-shot creature-type setter): Dub's "is a Knight" and Nameless
// Inversion's "loses all creature types", in both orders.
func TestL7GorgeSubtypeGrantVsRecordSetter(t *testing.T) {
	t.Parallel()

	runLayerScen(t, layerScen("dub-then-inversion", []string{"Craw Wurm"}, []string{"Dub", "Nameless Inversion"},
		[]lsStep{cast("Dub", "WWW", "Craw Wurm"), cast("Nameless Inversion", "BB", "Craw Wurm")},
		[]lsStep{expectOf("Craw Wurm", "no_types", []string{"Knight", "Wurm"}, "pt", "11/3")}))
	runLayerScen(t, layerScen("inversion-then-dub", []string{"Craw Wurm"}, []string{"Dub", "Nameless Inversion"},
		[]lsStep{cast("Nameless Inversion", "BB", "Craw Wurm"), cast("Dub", "WWW", "Craw Wurm")},
		[]lsStep{expectOf("Craw Wurm", "types", []string{"Creature", "Knight"}, "no_types", []string{"Wurm"})}))
}

// TestL7GorgeTwoStaticSetters (layers 4 and 7b, two override statics):
// Witness Protection (a 1/1 Citizen creature) and Darksteel Mutation (a
// 0/1 Insect artifact creature), in both orders: the newer wins each layer.
func TestL7GorgeTwoStaticSetters(t *testing.T) {
	t.Parallel()

	runLayerScen(t, layerScen("wp-then-mutation", []string{"Craw Wurm"}, []string{"Witness Protection", "Darksteel Mutation"},
		[]lsStep{cast("Witness Protection", "UU", "Craw Wurm"), cast("Darksteel Mutation", "WW", "Craw Wurm")},
		[]lsStep{expectOf("Craw Wurm", "types", []string{"Artifact", "Creature", "Insect"}, "no_types", []string{"Citizen", "Wurm"}, "pt", "0/1", "colours", "GW")}))
	runLayerScen(t, layerScen("mutation-then-wp", []string{"Craw Wurm"}, []string{"Witness Protection", "Darksteel Mutation"},
		[]lsStep{cast("Darksteel Mutation", "WW", "Craw Wurm"), cast("Witness Protection", "UU", "Craw Wurm")},
		[]lsStep{expectOf("Craw Wurm", "types", []string{"Creature", "Citizen"}, "no_types", []string{"Artifact", "Insect", "Wurm"}, "pt", "1/1")}))
}

// TestL7GorgeSubtypeGrantVsStaticSetter (layer 4 on a walked slot): Dub's
// Knight grant and Reprobation's Coward setter, in both orders.
func TestL7GorgeSubtypeGrantVsStaticSetter(t *testing.T) {
	t.Parallel()

	runLayerScen(t, layerScen("reprobation-then-dub", []string{"Craw Wurm"}, []string{"Reprobation", "Dub"},
		[]lsStep{cast("Reprobation", "WW", "Craw Wurm"), cast("Dub", "WWW", "Craw Wurm")},
		[]lsStep{expectOf("Craw Wurm", "types", []string{"Creature", "Coward", "Knight"}, "no_types", []string{"Wurm"}, "pt", "2/3")}))
	runLayerScen(t, layerScen("dub-then-reprobation", []string{"Craw Wurm"}, []string{"Reprobation", "Dub"},
		[]lsStep{cast("Dub", "WWW", "Craw Wurm"), cast("Reprobation", "WW", "Craw Wurm")},
		[]lsStep{expectOf("Craw Wurm", "types", []string{"Creature", "Coward"}, "no_types", []string{"Knight", "Wurm"}, "pt", "2/3")}))
}

// TestL7GorgeColourSetters (layer 5, two colour statics): Thran Lens's
// "all permanents are colourless" and Witness Protection's green-white,
// in both orders.
func TestL7GorgeColourSetters(t *testing.T) {
	t.Parallel()

	runLayerScen(t, layerScen("lens-then-wp", []string{"Craw Wurm", "Thran Lens"}, []string{"Witness Protection"},
		[]lsStep{cast("Witness Protection", "UU", "Craw Wurm")},
		[]lsStep{expectOf("Craw Wurm", "colours", "GW")}))
	runLayerScen(t, layerScen("wp-then-lens", []string{"Craw Wurm"}, []string{"Witness Protection", "Thran Lens"},
		[]lsStep{cast("Witness Protection", "UU", "Craw Wurm"), cast("Thran Lens", "CC")},
		[]lsStep{expectOf("Craw Wurm", "colours", "")}))
}

// TestL7GorgeSubtypeLordVsSetter (layer 4 dependency, CR 613.8): Valentina
// makes other Villains Heroes; Omnibian makes Kree Commandos a Frog, before
// or after Valentina entered, and either way the Commandos is no Hero (the
// setter changes what the lord applies to).
func TestL7GorgeSubtypeLordVsSetter(t *testing.T) {
	t.Parallel()

	runLayerScen(t, layerScen("lord-only", []string{"Kree Commandos", "Valentina Allegra de Fontaine"}, nil, nil,
		[]lsStep{expectOf("Kree Commandos", "types", []string{"Hero", "Villain"})}))
	runLayerScen(t, layerScen("lord-then-omnibian", []string{"Kree Commandos", "Valentina Allegra de Fontaine", "Omnibian"}, nil,
		[]lsStep{activate("Omnibian", "", "Kree Commandos")},
		[]lsStep{expectOf("Kree Commandos", "types", []string{"Frog"}, "no_types", []string{"Hero", "Villain"})}))
	runLayerScen(t, layerScen("omnibian-then-lord", []string{"Kree Commandos", "Omnibian"}, []string{"Valentina Allegra de Fontaine"},
		[]lsStep{activate("Omnibian", "", "Kree Commandos"), cast("Valentina Allegra de Fontaine", "BBB")},
		[]lsStep{expectOf("Kree Commandos", "types", []string{"Frog"}, "no_types", []string{"Hero", "Villain"})}))
}

// TestL7GorgeTypeAddVsRecordSetter (layer 4, a static card-type grant and
// a one-shot creature-type setter): Enchanted Evening and Omnibian's
// "becomes a Frog with base P/T 3/3", in both orders: both apply.
func TestL7GorgeTypeAddVsRecordSetter(t *testing.T) {
	t.Parallel()

	runLayerScen(t, layerScen("evening-then-omnibian", []string{"Craw Wurm", "Omnibian", "Enchanted Evening"}, nil,
		[]lsStep{activate("Omnibian", "", "Craw Wurm")},
		[]lsStep{expectOf("Craw Wurm", "types", []string{"Creature", "Enchantment", "Frog"}, "no_types", []string{"Wurm"}, "pt", "3/3")}))
	runLayerScen(t, layerScen("omnibian-then-evening", []string{"Craw Wurm", "Omnibian"}, []string{"Enchanted Evening"},
		[]lsStep{activate("Omnibian", "", "Craw Wurm"), cast("Enchanted Evening", "WWUUU")},
		[]lsStep{expectOf("Craw Wurm", "types", []string{"Creature", "Enchantment", "Frog"}, "no_types", []string{"Wurm"}, "pt", "3/3")}))
}

// TestL7GorgeLostSourceKeepsEarlierLayers (CR 613.6, 613.8a): Stop Cold on
// a creature whose static begins in layer 4 leaves that whole effect
// applying, parts in later layers included (Kudo's base 2/2, Ambush
// Commander's 1/1 green Forests), while a static with nothing before layer
// 6 (Lord of Atlantis's pump and landwalk) stops.
func TestL7GorgeLostSourceKeepsEarlierLayers(t *testing.T) {
	t.Parallel()

	runLayerScen(t, layerScen("stop-cold-on-kudo", []string{"Kudo, King Among Bears", "Craw Wurm"}, []string{"Stop Cold"},
		[]lsStep{cast("Stop Cold", "UUUU", "Kudo, King Among Bears")},
		[]lsStep{expectOf("Craw Wurm", "types", []string{"Creature", "Bear", "Wurm"}, "pt", "2/2")}))
	runLayerScen(t, layerScen("stop-cold-on-commander", []string{"Ambush Commander", "Forest"}, []string{"Stop Cold"},
		[]lsStep{cast("Stop Cold", "UUUU", "Ambush Commander")},
		[]lsStep{expectOf("Forest", "types", []string{"Land", "Creature", "Elf"}, "pt", "1/1", "colours", "G")}))
	runLayerScen(t, layerScen("stop-cold-on-lord", []string{"Lord of Atlantis", "Merfolk Looter"}, []string{"Stop Cold"},
		[]lsStep{cast("Stop Cold", "UUUU", "Lord of Atlantis")},
		[]lsStep{expectOf("Merfolk Looter", "pt", "1/1")}))
}
