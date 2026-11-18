//go:build manabrew

package manabrew

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// TestGameOverPromptShape pins the terminal prompt's wire shape: the fixed
// id, the seat it is addressed to, and the no-response gameOver input.
func TestGameOverPromptShape(t *testing.T) {
	tr := New("t1", 1, nil)
	v := view.View{Viewer: 2}
	pm := tr.GameOver(&v)
	if pm.PromptID != GameOverPromptID {
		t.Fatalf("promptId = %d, want the fixed GameOverPromptID %d", pm.PromptID, GameOverPromptID)
	}
	if pm.DecidingPlayerID != "player-2" {
		t.Fatalf("decidingPlayerId = %q, want player-2 (the seat the view belongs to)", pm.DecidingPlayerID)
	}
	if _, ok := pm.Input.Value.(mb.GameOverInput); !ok {
		t.Fatalf("input type = %s, want gameOver (no response)", pm.Input.Value.PromptType())
	}
	// The response side must keep refusing every answer to it: the mapping's
	// "gameOver carries no response" branch (errors.go) drives from the
	// prompt type, not the transport.
	ack := mb.ClientMessage{Value: mb.ClientResponse{Kind: "response", PromptID: pm.PromptID,
		Action: mb.PromptOutput{Type: "gameOver", Output: mb.PromptOutputData{Value: mb.RevealCardsAcknowledged{}}}}}
	outcome := tr.TranslateResponse(ack, &Pending{Prompt: pm, Decision: &decision.Decision{Player: 2}, View: view.View{Viewer: 2}}, 2)
	if outcome.Err == nil || outcome.Err.Code != mb.CodeWrongPromptType {
		t.Fatalf("TranslateResponse(ack to gameOver) = %+v, want CodeWrongPromptType", outcome)
	}
}

// TestPromptIDSpacesAreDisjoint pins the u32 id namespaces: an ordinary
// promptId (a bare decision.Seq below MaxDecisionSeq) can never equal a
// synthetic promptId (counting down from SyntheticPromptIDTop, never below
// SyntheticPromptIDFloor) or the terminal GameOverPromptID, and every id is
// in [0, 2^32).
func TestPromptIDSpacesAreDisjoint(t *testing.T) {
	if GameOverPromptID != math.MaxUint32 {
		t.Fatalf("GameOverPromptID = %d, want 2^32-1", GameOverPromptID)
	}
	if id := promptID(&decision.Decision{Seq: uint64(MaxDecisionSeq - 1)}); id < 0 || id >= SyntheticPromptIDFloor {
		t.Fatalf("top ordinary promptId = %d, want in [0, %d)", id, SyntheticPromptIDFloor)
	}
	seen := map[int64]bool{}
	check := func(seq uint64) {
		id, ok := syntheticPromptID(seq)
		if !ok {
			t.Fatalf("syntheticPromptID(%d) refused inside the range", seq)
		}
		if id < SyntheticPromptIDFloor || id >= GameOverPromptID {
			t.Fatalf("syntheticPromptID(%d) = %d outside [%d, %d)", seq, id, SyntheticPromptIDFloor, GameOverPromptID)
		}
		if seen[id] {
			t.Fatalf("syntheticPromptID(%d) = %d repeats", seq, id)
		}
		seen[id] = true
	}
	for seq := uint64(0); seq < 1<<20; seq += 7919 {
		check(seq)
	}
	last := uint64(SyntheticPromptIDTop-SyntheticPromptIDFloor) - 1
	check(last)
	if id, _ := syntheticPromptID(0); id != SyntheticPromptIDTop || id >= GameOverPromptID {
		t.Fatalf("first synthetic id = %d, want %d below the gameOver id", id, SyntheticPromptIDTop)
	}
	if _, ok := syntheticPromptID(last + 1); ok {
		t.Fatal("syntheticPromptID past the space must refuse, not wrap")
	}
}

// TestPromptIDSeqGuard: a decision.Seq with no u32-safe id is an explicit
// error, never a wrapped id.
func TestPromptIDSeqGuard(t *testing.T) {
	tr := New("t", 1, nil)
	if _, err := tr.Prompt(&decision.Decision{Seq: MaxDecisionSeq}, nil); err != errSeqExhausted {
		t.Fatalf("Prompt at MaxDecisionSeq: err = %v, want errSeqExhausted", err)
	}
}
