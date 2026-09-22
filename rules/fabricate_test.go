package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// fabricateEnters seats the named Fabricate card under seat 0, puts it onto
// the battlefield through events.MoveZone (the Afterlife/Mobilize genesis-seed
// / event-driven shape) and drives the ETB trigger to its modal ask. It
// returns the engine, its config and the permanent's ObjID with the Fabricate
// KModes decision pending.
func fabricateEnters(t *testing.T, name string, seats int) (*Engine, Config, state.ObjID) {
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
	var did state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			did = id
		}
	}
	if did == 0 {
		t.Fatalf("%s was not dealt", name)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: did, From: state.ZLibrary, To: state.ZBattlefield})
	e.priorityRound()
	return e, cfg, did
}

// fabricateServos returns seat 0's Servo tokens on the battlefield.
func fabricateServos(t *testing.T, e *Engine) []*state.Object {
	t.Helper()
	var out []*state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Servo Token" {
			out = append(out, o)
		}
	}
	return out
}

// fabricateCounters returns the number of +1/+1 counters on the object.
func fabricateCounters(t *testing.T, e *Engine, id state.ObjID) int {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil {
		t.Fatalf("object %d is gone", id)
	}
	for _, c := range o.Counters {
		if c.Kind == "P1P1" {
			return int(c.N)
		}
	}
	return 0
}

// TestFabricateAngelOfInventionChoosesServosOrCounters drives Angel of
// Invention's real K:Fabricate:2 line end to end in both branches. Before the
// fix the printed keyword never expanded, so the ETB made no ask and the
// permanent entered with no counters and no tokens.
func TestFabricateAngelOfInventionChoosesServosOrCounters(t *testing.T) {
	// Precondition: the modal ask is pending with exactly its two modes in
	// Choices$ order (counters then Servos) -- the assertion below depends on
	// the choice actually being offered, so a vacuous setup fails here.
	newPending := func(t *testing.T) (*Engine, Config, state.ObjID) {
		e, cfg, did := fabricateEnters(t, "Angel of Invention", 3)
		if o := e.G.Obj(did); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Angel of Invention did not enter the battlefield")
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no Fabricate decision pending on entry")
		}
		if d.Kind != decision.KModes || d.ResumeKind != "modes" {
			t.Fatalf("pending kind=%q resume=%q, want modes/modes", d.Kind, d.ResumeKind)
		}
		if len(d.Options) != 2 {
			t.Fatalf("Fabricate offered %d modes, want 2", len(d.Options))
		}
		return e, cfg, did
	}

	t.Run("counters", func(t *testing.T) {
		e, cfg, did := newPending(t)
		submitChoices(t, e, 0) // Choices$ order: 0 = counters, 1 = Servos
		passUntilStackEmpty(t, e, 30)
		if got := fabricateCounters(t, e, did); got != 2 {
			t.Fatalf("Angel of Invention has %d +1/+1 counters, want 2", got)
		}
		if toks := fabricateServos(t, e); len(toks) != 0 {
			t.Fatalf("counters branch minted %d Servo tokens, want 0", len(toks))
		}
		replayCheck(t, e, cfg)
	})

	t.Run("servos", func(t *testing.T) {
		e, cfg, did := newPending(t)
		submitChoices(t, e, 1)
		passUntilStackEmpty(t, e, 30)
		if got := fabricateCounters(t, e, did); got != 0 {
			t.Fatalf("Servos branch left %d +1/+1 counters, want 0", got)
		}
		toks := fabricateServos(t, e)
		if len(toks) != 2 {
			t.Fatalf("Servos branch minted %d Servo tokens, want 2", len(toks))
		}
		for _, tok := range toks {
			f := tok.Face()
			if !tok.IsToken {
				t.Fatalf("Servo %d is not a token", tok.ID)
			}
			if f.Power() != 1 || f.Toughness() != 1 {
				t.Fatalf("Servo %d has PT %d/%d, want 1/1", tok.ID, f.Power(), f.Toughness())
			}
			if effects.ColorsOf(tok) != "" {
				t.Fatalf("Servo %d colours=%q, want colorless", tok.ID, effects.ColorsOf(tok))
			}
			if !hasTypeWord(f.Types, "Artifact") || !hasTypeWord(f.Types, "Creature") || !hasTypeWord(f.Types, "Servo") {
				t.Fatalf("Servo %d types=%q, want Artifact Creature Servo", tok.ID, f.Types)
			}
			if tok.Controller != 0 {
				t.Fatalf("Servo %d controller %d, want seat 0", tok.ID, tok.Controller)
			}
		}
		replayCheck(t, e, cfg)
	})
}

// TestFabricateCountScalesWithTheKeyword: a single-count card passing by
// coincidence is the failure mode a spliced-count bug hides behind, so
// K:Fabricate:1 (Glint-Sleeve Artisan), K:Fabricate:2 (Maulfist Squad) and
// K:Fabricate:3 (Marionette Master) must each produce exactly their N.
func TestFabricateCountScalesWithTheKeyword(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{{"Glint-Sleeve Artisan", 1}, {"Cultivator of Blades", 2}, {"Marionette Master", 3}} {
		for _, mode := range []int{0, 1} {
			e, _, did := fabricateEnters(t, tc.name, 2)
			d := e.Pending()
			if d == nil || d.Kind != decision.KModes || len(d.Options) != 2 {
				t.Fatalf("%s did not pose the two-mode Fabricate ask", tc.name)
			}
			submitChoices(t, e, mode)
			passUntilStackEmpty(t, e, 30)
			if mode == 0 {
				if got := fabricateCounters(t, e, did); got != tc.count {
					t.Fatalf("%s counters branch: %d +1/+1 counters, want %d", tc.name, got, tc.count)
				}
				if toks := fabricateServos(t, e); len(toks) != 0 {
					t.Fatalf("%s counters branch minted %d Servos, want 0", tc.name, len(toks))
				}
			} else {
				if got := fabricateCounters(t, e, did); got != 0 {
					t.Fatalf("%s servos branch left %d counters, want 0", tc.name, got)
				}
				if toks := fabricateServos(t, e); len(toks) != tc.count {
					t.Fatalf("%s servos branch minted %d Servos, want %d", tc.name, len(toks), tc.count)
				}
			}
		}
	}
}
