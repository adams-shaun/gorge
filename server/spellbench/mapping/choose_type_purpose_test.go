package mapping_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

// TestChooseTypePurposeByFamily pins the v2 choose_name purpose against the
// engine's option universe, not the resolving card: the same "choose a type"
// ask projects as creature_type, basic_land_type, land_type or card_type
// according to the labels offered, and every candidate passes the validator's
// V1 candidate check (which is what rejected a creature type under card_type).
func TestChooseTypePurposeByFamily(t *testing.T) {
	// ResolveSource touches only Env.G.E.G for a source-less choice, so a
	// zero engine is enough: this test is about projection, not game state.
	env := &mapping.Env{G: &gamecfg.Game{E: &rules.Engine{}}}
	cases := []struct {
		name, purpose, wantValue string
		labels                   []string
	}{
		{"creature", "creature_type", "advisor",
			[]string{"Advisor", "Archer", "Artificer", "Avatar", "Bird"}},
		{"basic_land", "basic_land_type", "forest",
			[]string{"Forest", "Island", "Mountain", "Plains", "Swamp"}},
		{"land", "land_type", "cave",
			[]string{"Cave", "Forest", "Gate", "Island", "Tower"}},
		{"card", "card_type", "artifact",
			[]string{"Artifact", "Creature", "Instant", "Land", "Sorcery"}},
		{"other", "other", "ajani",
			[]string{"Ajani", "Bolas", "Chandra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
				Prompt: "Choose a type"}
			for i, label := range tc.labels {
				d.Options = append(d.Options, decision.Option{Index: i, Kind: "type", Label: label})
			}
			tx, err := mapping.Begin(env, d)
			if err != nil {
				t.Fatalf("Begin: %v", err)
			}
			p, err := tx.Pose()
			if err != nil {
				t.Fatalf("Pose: %v", err)
			}
			if len(p.Candidates) != len(tc.labels) {
				t.Fatalf("%d candidates, want one per offered label (%d)", len(p.Candidates), len(tc.labels))
			}
			found := false
			for i, c := range p.Candidates {
				if c.Sem.Kind != "choose_name" {
					t.Fatalf("candidate %d kind %q, want choose_name", i, c.Sem.Kind)
				}
				if got := c.Sem.Fields["purpose"]; got != tc.purpose {
					t.Fatalf("candidate %d purpose %v, want %s", i, got, tc.purpose)
				}
				if err := c.Sem.Check(); err != nil {
					t.Fatalf("candidate %d fails the v2 validator: %v", i, err)
				}
				if c.Sem.Fields["value"] == tc.wantValue {
					found = true
				}
			}
			if !found {
				t.Fatalf("no candidate carries value %q", tc.wantValue)
			}
		})
	}
}
