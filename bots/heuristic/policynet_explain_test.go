package heuristic

import (
	"context"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPolicyNetBotExplains drives one game between two random-checkpoint
// PolicyNetBots scoring attackers and blockers, one pair with a note sink
// and one without, at the same decisions: the answers are identical (a note
// only reads), every Decide emits exactly one note, a policy note carries
// the net's per-option scores and value in [-1,1], and wherever the note
// says the bot answered, the bot's answer is the intent played.
func TestPolicyNetBotExplains(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	sc := randomCheckpoint(t, 77)
	const seed = 101
	kinds := []decision.Kind{decision.KAttackers, decision.KBlockers}
	plain := []*PolicyNetBot{NewPolicyNetBotKinds(seed^1, sc, kinds), NewPolicyNetBotKinds(seed^2, sc, kinds)}
	noted := []*PolicyNetBot{NewPolicyNetBotKinds(seed^1, sc, kinds), NewPolicyNetBotKinds(seed^2, sc, kinds)}
	var notes []explain.Note
	for _, b := range noted {
		b.SetExplain(func(n explain.Note) { notes = append(notes, n) })
	}
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	e.Advance()
	policy := 0
	for n := 0; !e.G.Over && e.Pending() != nil && n < 4000; n++ {
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		want, err := plain[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatal(err)
		}
		before := len(notes)
		got, err := noted[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.Choices, want.Choices) {
			t.Fatalf("intent %d: the noted seat answered %v, the plain one %v", n, got.Choices, want.Choices)
		}
		if len(notes) != before+1 {
			t.Fatalf("intent %d (%s): %d notes for one decision", n, d.Kind, len(notes)-before)
		}
		note := notes[len(notes)-1]
		if note.Seat != int(d.Player) || note.Source == "" {
			t.Fatalf("intent %d: note %+v", n, note)
		}
		switch note.Source {
		case explain.Policy:
			policy++
			if note.Net == nil || len(note.Net.Scores) != len(d.Options) || note.Net.Value < -1 || note.Net.Value > 1 {
				t.Fatalf("intent %d: policy note net %+v for %d options", n, note.Net, len(d.Options))
			}
		case explain.Heuristic:
			if !slices.Equal(note.Bot, got.Choices) {
				t.Fatalf("intent %d: heuristic note's bot %v, intent %v", n, note.Bot, got.Choices)
			}
		default:
			t.Fatalf("intent %d: source %s", n, note.Source)
		}
		if err := e.Submit(got); err != nil {
			t.Fatal(err)
		}
	}
	if policy == 0 {
		t.Fatal("no decision was scored; the policy note never ran")
	}
	t.Logf("%d notes, %d scored", len(notes), policy)
}
