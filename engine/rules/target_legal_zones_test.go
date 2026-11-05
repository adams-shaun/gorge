package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestChangeZoneMultiZoneTargetZones: a bare ValidTgts$ alternative over a
// multi-zone Origin$ names the same kind of object in every non-stack zone
// the Origin$ names (Aurelia's Vindicator, Angel of Serenity); a single-zone
// Origin$ and an explicit TgtZone$ are unchanged.
func TestChangeZoneMultiZoneTargetZones(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bfGy := []state.Zone{state.ZBattlefield, state.ZGraveyard}

	for _, name := range []string{"Aurelia's Vindicator", "Angel of Serenity"} {
		t.Run(name, func(t *testing.T) {
			c := lookup(t, reg, name)
			var sa *cards.SA
			for _, tr := range c.Faces[0].Triggers {
				if b := cards.ResolveSVar(c.Faces[0].SVars, tr.Params["Execute"]); b != nil && b.API == "ChangeZone" && b.Params["ValidTgts"] != "" {
					sa = b
				}
			}
			if sa == nil {
				t.Fatalf("precondition: %s has a targeted ChangeZone trigger body", name)
			}
			cz := effects.ChangeZoneOf(sa)
			if !cz.OriginPresent || len(cz.OriginOwn) != 2 || sa.Params["TgtZone"] != "" {
				t.Fatalf("precondition: want a two-zone Origin$ and no TgtZone$: %+v", cz)
			}
			if got := targetZones(sa); !slices.Equal(got, bfGy) {
				t.Fatalf("targetZones = %v, want %v", got, bfGy)
			}
		})
	}

	t.Run("single-zone origin unchanged", func(t *testing.T) {
		c := card(t, "Name:Test Recur\nTypes:Creature Elf\nPT:1/1\nA:AB$ ChangeZone | Cost$ T | ValidTgts$ Land.YouOwn | Origin$ Graveyard | Destination$ Hand\nOracle:x\n")
		sa := c.Faces[0].Abilities[0]
		if !effects.ChangeZoneOf(sa).OriginExactly(state.ZGraveyard) {
			t.Fatal("precondition: a one-zone graveyard Origin$")
		}
		if got := targetZones(sa); !slices.Equal(got, []state.Zone{state.ZGraveyard}) {
			t.Fatalf("targetZones = %v, want [graveyard]", got)
		}
	})

	t.Run("explicit TgtZone stays authoritative", func(t *testing.T) {
		c := lookup(t, reg, "Endless Detour")
		var sa *cards.SA
		for _, a := range c.Faces[0].Abilities {
			if a.API == "ChangeZone" {
				sa = a
			}
		}
		if sa == nil || sa.Params["TgtZone"] == "" {
			t.Fatalf("precondition: Endless Detour's ChangeZone carries TgtZone$: %+v", sa)
		}
		want := []state.Zone{state.ZBattlefield, state.ZStack, state.ZGraveyard}
		if got := targetZones(sa); !slices.Equal(got, want) {
			t.Fatalf("targetZones = %v, want %v", got, want)
		}
	})
}
