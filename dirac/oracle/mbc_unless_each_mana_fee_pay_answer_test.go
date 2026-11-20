package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// payAnswerPolicy serves the recorded prefix, then answers the unless-pay ask
// with its pay mode. It fails the test through paid when no pay option was
// posed.
type payAnswerPolicy struct {
	script *ScriptPolicy
	at     int
	paid   bool
}

func (p *payAnswerPolicy) Pick(c *PickCtx) Canon {
	if c.Seq == p.at && c.G != nil {
		for _, a := range c.D.Options {
			if o, ok := c.G.GorgeOption(a); ok && o.Mode == decision.ModeUnlessPay {
				p.paid = true
				return a
			}
		}
	}
	return p.script.Pick(c)
}

// TestMBCUnlessEachManaFeePayAnswer: the recorded pair's pay option is chosen
// explicitly. Gorge's own window taps the sources in its answer order
// (Swamp, Plains, Power Depot, Lantern of Revealing, Overflowing Basin), floats
// {W}{U}{B}{G} with no {R}, and takes the unpaid branch (the Event log's
// "sacrificed"); Lantern of Revealing is asked its first colour, not the red pip
// still owed. The kernel's greedy window makes the same taps and the same
// short, so both engines sacrifice the Dragon; the pay answer must replay
// clean, and a smarter filter plan would diverge from the leader.
func TestMBCUnlessEachManaFeePayAnswer(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/unless_each_mana_fee_offer.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	sp := &ScriptPolicy{Actions: r.Actions}
	pol := &payAnswerPolicy{script: sp, at: len(r.Actions)}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if !pol.paid {
		t.Fatal("the unless-pay ask never offered its pay mode")
	}
	if sp.Skipped != 0 {
		t.Fatalf("the replay skipped %d recorded actions", sp.Skipped)
	}
}
