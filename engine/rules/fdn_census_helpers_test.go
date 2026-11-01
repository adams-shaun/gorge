package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Shared fixtures for the dirac FDN census regressions: a corpusEngine
// over named real corpus cards for seat 0, and a stack drain that passes
// priority and answers any other ask with its first option.

func fdnCensusEngine(t *testing.T, names ...string) (*Engine, *cards.Registry) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var cs []*cards.Card
	for _, n := range names {
		cs = append(cs, mustCorpusCard(t, reg, n))
	}
	return corpusEngine(t, reg, cs, nil), reg
}

func fdnCensusDrain(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while draining")
		}
		if d.Kind == decision.KPriority {
			submitPass(t, e)
			continue
		}
		submitChoices(t, e, 0)
	}
}

// fdnCensusAnswerMin answers a non-priority decision with its first Min options.
func fdnCensusAnswerMin(t *testing.T, e *Engine, d *decision.Decision) {
	t.Helper()
	var ch []int
	for i := 0; i < d.Min; i++ {
		ch = append(ch, d.Options[i].Index)
	}
	submitChoices(t, e, ch...)
}
