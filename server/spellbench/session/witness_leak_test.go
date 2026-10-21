package session_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestCawGatesSeed4NoLeak replays the audited uniform walk that first reached
// a transformed CawGates permanent. Its printed front ("The Modern Age") is
// public on the battlefield, but the hidden-name scan used to count only the
// current face, so the public front name scored as a leak whenever another
// copy sat in a library.
func TestCawGatesSeed4NoLeak(t *testing.T) {
	g := startAudited(t, "CawGates", 4000)
	r := rand.New(rand.NewPCG(4, 7))
	turn := uint32(0)
	blockedAttackers := map[string]bool{}
	usedBlockers := map[string]bool{}
	for {
		dec, term := g.Pending()
		if term != nil {
			if term.Classification == "halted" {
				t.Fatalf("CawGates halted: %s", term.Reason)
			}
			break
		}
		obs := dec.SeatDecision.Observation
		if obs.Turn != turn {
			turn = obs.Turn
		}
		if obs.PhaseStep != "declare_blockers" {
			blockedAttackers, usedBlockers = map[string]bool{}, map[string]bool{}
		}
		k := r.IntN(len(dec.SeatDecision.Candidates))
		if obs.PhaseStep == "declare_blockers" {
			for i, cand := range dec.SeatDecision.Candidates {
				if cand.Semantic.Kind != "declare_block" {
					continue
				}
				a, _ := cand.Semantic.Fields["attacker"].(*protocol.ObjectRef)
				b, _ := cand.Semantic.Fields["blocker"].(protocol.ObjectRef)
				if a != nil && b.ObjectID != "" && blockedAttackers[a.ObjectID] && !usedBlockers[b.ObjectID] {
					k = i
					break
				}
			}
		}
		c := dec.SeatDecision.Candidates[k]
		if c.Semantic.Kind == "declare_block" {
			if a, _ := c.Semantic.Fields["attacker"].(*protocol.ObjectRef); a != nil {
				blockedAttackers[a.ObjectID] = true
				if b, _ := c.Semantic.Fields["blocker"].(protocol.ObjectRef); b.ObjectID != "" {
					usedBlockers[b.ObjectID] = true
				}
			}
		}
		if perr := g.Step(&protocol.StepReq{GameID: "g-w", ExpectedStep: dec.Step, CandidateID: uint64(k),
			Echo: echo(t, c.Semantic)}); perr != nil {
			t.Fatalf("CawGates: %v", perr)
		}
	}
	if g.Leaks() != 0 {
		t.Fatalf("CawGates seed (4,7): %d leak-scan hits", g.Leaks())
	}
}
