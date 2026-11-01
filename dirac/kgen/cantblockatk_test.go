package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kcantblockatkLower builds the Gossip's Talent level-2 shape (an
// AttackersDeclared trigger whose Effect remembers an attacking target and
// grants CantBlockBy to it) with the given parameters and lowers it.
func kcantblockatkLower(t *testing.T, validTgts, remember, extra string) *lCard {
	t.Helper()
	exec := "AB$ Effect | RememberObjects$ " + remember + " | ValidTgts$ " + validTgts
	if extra != "" {
		exec += " | " + extra
	}
	exec += " | StaticAbilities$ Unblockable"
	svars := map[string]string{
		"Trig":        exec,
		"Unblockable": "Mode$ CantBlockBy | ValidAttacker$ Card.IsRemembered",
	}
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "1 G",
		Types:    []string{"Enchantment"},
		SVars:    svars,
		Triggers: []cards.Trigger{{
			Mode:   "AttackersDeclared",
			Params: map[string]string{"AttackingPlayer": "You", "TriggerZones": "Battlefield", "Execute": "Trig"},
			Effect: cards.ResolveSVar(svars, "Trig"),
		}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

func kcantblockatkReasons(c *lCard) string {
	var reasons []string
	for _, r := range c.Refusals {
		reasons = append(reasons, r.Node+": "+r.Reason)
	}
	return strings.Join(reasons, "; ")
}

// TestCantblockatkLowers: the no-ExileOnMoved$ CantBlockBy Effect lowers
// only when its remembered object is its own attacking target (Gossip's
// Talent); the same shape on a non-attacking target or a remembered trigger
// source keeps refusing.
func TestCantblockatkLowers(t *testing.T) {
	t.Run("gossip attacking target", func(t *testing.T) {
		c := kcantblockatkLower(t, "Creature.attacking+powerLE3", "Targeted", "")
		if got := kcantblockatkReasons(c); got != "" {
			t.Fatalf("refused: %s", got)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), "GrantKeyword obj=Target0 CS=Unblockable") {
			t.Errorf("lowered IR lacks the unblockable grant:\n%s", sb.String())
		}
	})
	t.Run("non-attacking target", func(t *testing.T) {
		c := kcantblockatkLower(t, "Creature.powerLE3", "Targeted", "")
		if got := kcantblockatkReasons(c); !strings.Contains(got, "Effect CantBlockBy shape") {
			t.Fatalf("want Effect CantBlockBy shape, got %q", got)
		}
	})
	t.Run("remembered trigger source", func(t *testing.T) {
		c := kcantblockatkLower(t, "Creature.attacking+powerLE3", "TriggeredAttacker", "")
		if got := kcantblockatkReasons(c); !strings.Contains(got, `Effect RememberObjects "TriggeredAttacker"`) {
			t.Fatalf("want the RememberObjects refusal, got %q", got)
		}
	})
	t.Run("explicit duration", func(t *testing.T) {
		c := kcantblockatkLower(t, "Creature.attacking+powerLE3", "Targeted", "Duration$ UntilYourNextEndStep")
		if got := kcantblockatkReasons(c); !strings.Contains(got, "Effect CantBlockBy shape") {
			t.Fatalf("want Effect CantBlockBy shape, got %q", got)
		}
	})
	t.Run("mixed target alternatives", func(t *testing.T) {
		c := kcantblockatkLower(t, "Creature.attacking,Creature.powerLE3", "Targeted", "")
		if got := kcantblockatkReasons(c); !strings.Contains(got, "Effect CantBlockBy shape") {
			t.Fatalf("want Effect CantBlockBy shape, got %q", got)
		}
	})
}
