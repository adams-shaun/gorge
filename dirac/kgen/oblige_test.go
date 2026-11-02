package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// obligeSynthetic lowers one synthetic spell whose root Pump targets a
// player (the parent) and whose ChangeZone sub-ability is the oblige
// shuffle shape. It returns the card's refusal reasons.
func obligeSynthetic(t *testing.T, root, sub map[string]string) []refusal {
	t.Helper()
	svars := map[string]string{"DBChangeZone": subAbilityLine(sub)}
	rootSA := &cards.SA{Kind: "SP", API: "Pump", Params: root, Sub: cards.ResolveSVar(svars, "DBChangeZone")}
	if rootSA.Sub == nil {
		t.Fatal("synthetic sub-ability did not resolve")
	}
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "G",
		Types:    []string{"Sorcery"},
		SVars:    svars,
		Abilities: []*cards.SA{
			rootSA,
		},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{
		byName: map[string]*cards.Card{"Synthetic": card},
		nDecks: map[string]int{},
		reg:    &cards.Registry{},
	}
	return lowerCard(p, "Synthetic", card, false).Refusals
}

func subAbilityLine(m map[string]string) string {
	var b strings.Builder
	b.WriteString("DB$ ChangeZone")
	for _, k := range []string{"Origin", "Destination", "Shuffle", "ValidTgts", "TargetMin", "TargetMax", "TargetsWithDefinedController"} {
		if v, ok := m[k]; ok {
			b.WriteString(" | ")
			b.WriteString(k)
			b.WriteString("$ ")
			b.WriteString(v)
		}
	}
	return b.String()
}

// TestObligeParentTargetRefusals pins the ParentTarget shapes kgen may lower
// to OpMayShuffleMove. The kernel reads one player parent slot and only
// graveyard card targets (State.parentTargetOK); gorge's
// filterParentControllerTargets reads every parent target, object parents
// included, and the effect's own zone. Anything wider must refuse by name.
func TestObligeParentTargetRefusals(t *testing.T) {
	parent := func(extra map[string]string) map[string]string {
		m := map[string]string{"ValidTgts": "Player"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	grave := func(extra map[string]string) map[string]string {
		m := map[string]string{
			"Origin": "Graveyard", "Destination": "Library", "Shuffle": "True",
			"ValidTgts": "Card", "TargetMin": "0", "TargetMax": "2",
			"TargetsWithDefinedController": "ParentTarget",
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	with := func(m map[string]string, k, v string) map[string]string {
		out := map[string]string{}
		for kk, vv := range m {
			out[kk] = vv
		}
		out[k] = v
		return out
	}
	for _, tc := range []struct {
		name string
		root map[string]string
		sub  map[string]string
		want string
	}{
		{"parent Max 2", parent(map[string]string{"TargetMax": "2"}), grave(nil), "parent TargetMax$ 2 is not 1"},
		{"parent Min 2", parent(map[string]string{"TargetMin": "2"}), grave(nil), "parent TargetMin$ 2 is not 0 or 1"},
		{"parent not a player", map[string]string{"ValidTgts": "Creature"}, grave(nil), "parent is not a player target"},
		{"target zone not graveyard", parent(nil), with(grave(nil), "Origin", "Battlefield"), "target zone Battlefield is not Graveyard"},
		{"target filter names players", parent(nil), with(grave(nil), "ValidTgts", "Player"), "names players"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refs := obligeSynthetic(t, tc.root, tc.sub)
			if len(refs) == 0 {
				t.Fatalf("lowered; want a refusal containing %q", tc.want)
			}
			hit := false
			for _, r := range refs {
				hit = hit || strings.Contains(r.Reason, tc.want)
			}
			if !hit {
				t.Fatalf("refusals %v; want one containing %q", refs, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name string
		root map[string]string
	}{
		{"parent default Min/Max", parent(nil)},
		{"parent Min 0", parent(map[string]string{"TargetMin": "0"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if refs := obligeSynthetic(t, tc.root, grave(nil)); len(refs) != 0 {
				t.Fatalf("refused: %v", refs)
			}
		})
	}
}
