package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDraftPartnersSeatsTwoMonoColourCommanders: two partnerless mono-colour
// legendary creatures are not a CR 903.13 pair, so a plain Commander game
// rejects the seat whole; Config.DraftPartners (the Commander-draft rule
// that treats such a commander as having partner) seats both.
func TestDraftPartnersSeatsTwoMonoColourCommanders(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	white := lookup(t, reg, "Isamaru, Hound of Konda")
	red := lookup(t, reg, "Krenko, Mob Boss")
	pair := []*cards.Card{white, red}

	plain := New(commanderConfig(t, reg, pair, []int{0, 1}))
	plain.Advance()
	if n := len(plain.G.Players[0].Commanders); n != 0 || !hasCommanderRejectionNote(plain) {
		t.Fatalf("without DraftPartners the pair seated %d commanders (rejection note %v), want 0 and a note", n, hasCommanderRejectionNote(plain))
	}

	cfg := commanderConfig(t, reg, pair, []int{0, 1})
	cfg.DraftPartners = true
	e := New(cfg)
	e.Advance()
	if hasCommanderRejectionNote(e) {
		t.Fatal("DraftPartners: the pair was rejected at seating")
	}
	if n := len(e.G.Players[0].Commanders); n != 2 {
		t.Fatalf("DraftPartners: seat 0 has %d commanders, want 2", n)
	}
}
