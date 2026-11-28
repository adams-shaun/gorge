package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAltarVictimTwoAltarsRecord plays the two-altars-one-creature game on
// the fixed planner and re-records it: the script ends at the decision where
// gorge once refused to plan Captivating Gyre's cast, both engines now offer
// that cast there (one altar and four lands fund it, sacrificing the
// announced target), and the game plays on by FirstPolicy. The cast is not
// taken: gorge asks the spell's optional target while the kernel completes
// the scripted cast without asking, an ask-shape divergence of its own (filed
// as a ticket alongside this re-record); taking the cast here would pin that
// bug instead of the offer parity. It runs only with PINS_RECORD=1; the game
// it writes is the committed repro.
type twoAltarsOfferCheck struct {
	first      Policy
	offered336 bool
}

func (p *twoAltarsOfferCheck) Pick(c *PickCtx) Canon {
	if c.Seq == 336 && c.G != nil && c.D.Kind == DPriority {
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && c.G.M.InstanceCard(int(a.Src())) == "Captivating Gyre" {
				p.offered336 = true
			}
		}
	}
	return p.first.Pick(c)
}

func TestAltarVictimTwoAltarsRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips
	}
	const name = "altar_victim_two_altars_one_target_no_cast"
	path := filepath.Join("testdata", "repro", name+".repro.json")
	reg := synthReg(t)
	r, err := ReadRepro(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Shrunk != nil || r.Divergence == nil {
		t.Fatalf("re-record the pinned divergence game, not a shrunk or clean one: %+v", r.Divergence)
	}
	set := globSet(t, r.Pool, r)
	pol := &ScriptPolicy{Actions: r.Actions, Tail: &twoAltarsOfferCheck{first: FirstPolicy()}}
	res := replayDecodedScript(t, reg, set, r, pol)
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("prefix served %d of %d actions (%d skipped)", pol.next, len(r.Actions), pol.Skipped)
	}
	if !pol.Tail.(*twoAltarsOfferCheck).offered336 {
		t.Fatal("decision 336 never offered Captivating Gyre's cast: the fixed planner must offer it there")
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(res.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the re-recorded game kernel-stalled: %s", res.Stall)
	}
	r.setActions(res.Actions)
	r.Divergence = nil
	r.Why = "two Ashnod's Altars and one creature, Captivating Gyre's only target: the planner drops the altar whose step cannot name a fresh victim and re-searches, so one altar and four lands fund the cast and both engines offer it at the pinned decision"
	if err := r.Write(path); err != nil {
		t.Fatal(err)
	}
	t.Logf("re-recorded %s: %d actions", name, len(res.Actions))
}
