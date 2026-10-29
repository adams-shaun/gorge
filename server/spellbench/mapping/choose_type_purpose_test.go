package mapping_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

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
