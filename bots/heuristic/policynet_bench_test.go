package heuristic

import (
	"bytes"
	"context"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// BenchmarkPolicyNetDecideAttackers is one scored PolicyNetBot decision (a
// random checkpoint scoring an attackers declaration, the default kind) at
// a fixed position, with no note sink installed. Its allocs/op and B/op must
// not move when a diagnostic lands on the seat (ticket T3's zero-cost-off
// check); compare it against the same file at the base revision.
func BenchmarkPolicyNetDecideAttackers(b *testing.B) {
	m := policynet.NewModel(policynet.TableRows, 16, 8, rand.New(rand.NewPCG(77, 77^0x5eed)))
	var buf bytes.Buffer
	if err := policynet.WriteCheckpoint(m, &buf); err != nil {
		b.Fatal(err)
	}
	sc, err := policynet.LoadScorer(bytes.NewReader(buf.Bytes()))
	if err != nil {
		b.Fatal(err)
	}
	names, decks := testutil.SampleDecks(b, 2)
	e := rules.New(rules.Config{Seed: 101, Names: names, Decks: decks})
	e.Advance()
	def := []*Bot{NewBot(1), NewBot(2)}
	var v view.View
	var d *decision.Decision
	for n := 0; n < 4000 && !e.G.Over; n++ {
		d = e.Pending()
		v = view.Project(e.G, e, d.Player, d)
		if d.Kind == decision.KAttackers && len(d.Options) >= 2 {
			break
		}
		in, err := def[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			b.Fatal(err)
		}
		if err := e.Submit(in); err != nil {
			b.Fatal(err)
		}
	}
	if d == nil || d.Kind != decision.KAttackers {
		b.Fatal("no attackers decision with two options")
	}
	dd := *d
	bot := NewPolicyNetBot(1, sc)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := bot.Decide(context.Background(), v, dd); err != nil {
			b.Fatal(err)
		}
	}
}
