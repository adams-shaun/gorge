package mapping_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
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
	// ResolveSource touches only the board for a source-less choice, so a
	// zero engine is enough: this test is about projection, not game state.
	env := &mapping.Env{G: gamecfg.Wrap(&rules.Engine{}, nil)}
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
		{"other", "other", "Ajani",
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

// TestChooseTypePurposePerTypeCategory pins the choose_name purpose of a
// choose/type ask to the Type$ category the engine is posing: the mid-resolution
// producer (ResumeKind "choosetype") and the as-enters producer (ResumeKind
// "etb") both carry the category on the decision's server-side ResumeSA.
func TestChooseTypePurposePerTypeCategory(t *testing.T) {
	g := untilPending(t, "Spy", 1, func(d *decision.Decision, e *rules.Engine) bool { return d != nil })
	env := envFor(t, g)

	cases := []struct {
		name       string
		resumeKind string
		cat        string
		label      string
		want       string
	}{
		{"absent", "choosetype", "", "Human", "creature_type"},
		{"Creature", "choosetype", "Creature", "Ally", "creature_type"},
		{"CreatureInTargetedDeck", "choosetype", "CreatureInTargetedDeck", "Goblin", "creature_type"},
		{"Basic Land", "choosetype", "Basic Land", "Forest", "basic_land_type"},
		{"Land", "choosetype", "Land", "Island", "land_type"},
		{"Nonbasic Land", "choosetype", "Nonbasic Land", "Gate", "land_type"},
		{"Card", "choosetype", "Card", "Creature", "card_type"},
		{"Shared", "choosetype", "Shared", "Artifact", "card_type"},
		{"Planeswalker", "choosetype", "Planeswalker", "Jace", "other"},
		{"Letter", "choosetype", "Letter", "A", "other"},
		{"etb", "etb", "Creature", "Angel", "creature_type"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sa := &cards.SA{Params: map[string]string{}}
			if c.cat != "" {
				sa.Params[cards.PKType.String()] = c.cat
			}
			if got := sa.ParamStr(cards.PKType); got != c.cat {
				t.Fatalf("precondition: SA Type$ = %q, want %q", got, c.cat)
			}
			d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
				ResumeKind: c.resumeKind, ResumeSA: sa, Source: 0,
				Options: []decision.Option{
					{Index: 0, Kind: "type", Label: c.label},
					{Index: 1, Kind: "type", Label: "Second"},
				}}
			tx, err := mapping.Begin(env, d)
			if err != nil {
				t.Fatalf("Begin: %v", err)
			}
			p, err := tx.Pose()
			if err != nil {
				t.Fatalf("Pose: %v", err)
			}
			if len(p.Candidates) == 0 {
				t.Fatalf("Pose: no candidates")
			}
			cand := p.Candidates[0]
			if cand.Sem.Kind != "choose_name" {
				t.Fatalf("candidate kind = %q, want choose_name", cand.Sem.Kind)
			}
			if got := cand.Sem.Fields["purpose"]; got != c.want {
				t.Fatalf("%q: purpose = %v, want %q", c.cat, got, c.want)
			}
			if err := cand.Sem.Check(); err != nil {
				t.Fatalf("%q label %q: Check: %v", c.cat, c.label, err)
			}
			if p.Context.Purpose == nil {
				t.Fatalf("%q: context purpose is nil, want %q", c.cat, c.want)
			}
			if got := *p.Context.Purpose; got != c.want {
				t.Fatalf("%q: context purpose = %q, want %q", c.cat, got, c.want)
			}
		})
	}
}
