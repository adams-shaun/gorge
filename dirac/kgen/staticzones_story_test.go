package kgen

import (
	"strings"
	"testing"
)

// staticZonesStoryCarriers are the Condition$ EnduringStory carriers that
// lower whole: Bifur, Melodic Rider (a Panharmonicon static of Dwarf
// abilities), Dáin, Lord of the Iron Hills (an attack tax) and Fíli the
// Pathfinder (a Continuous pump). Each also prints K:Storied (CR 702.195),
// the keyword the kernel's enduring-story scan reads (dirac/kw_storied.go).
// Kíli the Resourceful prints Storied too and carries the EnduringStory
// condition, but its static (an AlternativeCost of activated equip
// abilities, gorge activate_static_altcost.go) lowers nothing until the
// attach-cost alternative does: it must not carry the static Condition "_"
// blocker any more, the class its own gate covers.
var staticZonesStoryCarriers = []string{
	"Bifur, Melodic Rider", "Dáin, Lord of the Iron Hills", "Fíli the Pathfinder",
}

// staticZonesStoryForms are each carrier's gated static mode, whose kernel
// reader consults the Cond2 through statCond: Bifur's Panharmonicon echoes
// (dirac panharmEchoes), Dáin's attack tax (dirac combat_attacktax.go),
// Fíli's Continuous pump (dirac statics_gen.go).
var staticZonesStoryForms = map[string]string{
	"Bifur, Melodic Rider":         "Panharmonicon",
	"Dáin, Lord of the Iron Hills": "CantAttackUnless",
	"Fíli the Pathfinder":          "Continuous",
}

func TestStaticZonesStoryForms(t *testing.T) {
	for _, name := range staticZonesStoryCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		if !c.Storied {
			t.Errorf("%s: CardDef.Storied not set", name)
		}
		want := staticZonesStoryForms[name]
		found := false
		for _, st := range c.Statics {
			if st.Kind == want {
				found = true
				if st.Cond.Kind != cEnduringStory {
					t.Errorf("%s: %s static's condition is %s, want EnduringStory", name, want, condNames[st.Cond.Kind])
				}
			}
		}
		if !found {
			t.Errorf("%s lowers to no %s static", name, want)
		}
	}
	// Kíli's EnduringStory condition rides a gate whose vocabulary covers it
	// (gorge abilityAlternateCostFor reads continuousConditionHolds), so the
	// static Condition "_" class must not name it.
	kili := lowerNamedCard(t, "Kíli the Resourceful")
	if !kili.Storied {
		t.Error("Kíli the Resourceful: CardDef.Storied not set")
	}
	for _, rf := range kili.Refusals {
		if strings.Contains(rf.Reason, `static Condition "EnduringStory"`) {
			t.Errorf("Kíli the Resourceful refusals %v still carry the static Condition \"EnduringStory\" blocker", kili.Refusals)
		}
	}
}
