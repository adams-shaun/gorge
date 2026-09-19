package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// afterlifeDies seats the named Afterlife card under seat 0, puts it on the
// battlefield through events.MoveZone and kills it with a lethal damage
// event -- the same genesis-seed / event-driven shape mobilizeCombat uses,
// minus the attack. The engine ends here with the creature dead and the
// Afterlife trigger drained, which is the state the tests assert from.
func afterlifeDies(t *testing.T, name string, seats int) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	src, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("%s missing from corpus", name)
	}
	if d := src.Link(); len(d) != 0 {
		t.Fatalf("link %s: %v", name, d)
	}
	names := make([]string, seats)
	decks := make([][]*cards.Card, seats)
	for i := range names {
		names[i] = string(rune('a' + i))
		decks[i] = mountainDeck(t, 40)
	}
	decks[0] = append(decks[0], src)
	cfg := seatZeroStart(Config{Seed: 197, Names: names, Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	want := name
	var did state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == want {
			did = id
		}
	}
	if did == 0 {
		t.Fatalf("%s was not dealt", name)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: did, From: state.ZLibrary, To: state.ZBattlefield})
	e.priorityRound()
	e.emit(events.Event{Kind: events.Damage, Obj: did, Amount: 99})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	return e, cfg, did
}

// afterlifeSpiritTokens returns seat 0's Afterlife Spirit tokens on the
// battlefield.
func afterlifeSpiritTokens(t *testing.T, e *Engine) []*state.Object {
	t.Helper()
	var out []*state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Spirit Token" {
			out = append(out, o)
		}
	}
	return out
}

// TestAfterlifeTitheTakerMintsSpiritTokens drives Tithe Taker's real
// K:Afterlife:1 line end to end: the creature dies and exactly one 1/1
// white-and-black Spirit token with flying appears under its controller.
// Before the fix the printed keyword never expanded and the death created
// nothing.
func TestAfterlifeTitheTakerMintsSpiritTokens(t *testing.T) {
	e, cfg, _ := afterlifeDies(t, "Tithe Taker", 3)
	toks := afterlifeSpiritTokens(t, e)
	if len(toks) != 1 {
		t.Fatalf("Afterlife minted %d Spirit tokens, want 1", len(toks))
	}
	tok := toks[0]
	f := tok.Face()
	if f.Power() != 1 || f.Toughness() != 1 || !strings.Contains(f.Colors, "white") || !strings.Contains(f.Colors, "black") || !hasTypeWord(f.Types, "Spirit") {
		t.Fatalf("token face %s colours=%q types=%q PT=%d/%d, want a 1/1 white-and-black Spirit", f.Name, f.Colors, f.Types, f.Power(), f.Toughness())
	}
	if !e.HasKeyword(tok.ID, "Flying") {
		t.Fatalf("token %d lacks flying", tok.ID)
	}
	if tok.Controller != 0 {
		t.Fatalf("token controller %d, want seat 0 (the dying creature's controller)", tok.Controller)
	}
	replayCheck(t, e, cfg)
}

// TestAfterlifeCountScalesWithTheKeyword: a single-count card passing by
// coincidence is the failure mode a TokenAmount$ bug hides behind, so
// K:Afterlife:2 (Seraph of the Scales) must mint exactly two tokens and
// K:Afterlife:3 (Knight of the Last Breath) exactly three.
func TestAfterlifeCountScalesWithTheKeyword(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{{"Seraph of the Scales", 2}, {"Knight of the Last Breath", 3}} {
		e, _, _ := afterlifeDies(t, tc.name, 2)
		toks := afterlifeSpiritTokens(t, e)
		if len(toks) != tc.count {
			t.Fatalf("%s minted %d Spirit tokens, want %d", tc.name, len(toks), tc.count)
		}
		for _, tok := range toks {
			f := tok.Face()
			if f.Power() != 1 || f.Toughness() != 1 || !strings.Contains(f.Colors, "white") || !strings.Contains(f.Colors, "black") || !hasTypeWord(f.Types, "Spirit") {
				t.Fatalf("%s token %d face %s colours=%q types=%q PT=%d/%d", tc.name, tok.ID, f.Name, f.Colors, f.Types, f.Power(), f.Toughness())
			}
			if tok.Controller != 0 {
				t.Fatalf("%s token controller %d, want seat 0", tc.name, tok.Controller)
			}
		}
	}
}
