package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The api:Incubate pin. Chrome Host Seedshark is the deck carrier
// (Counter Intelligence census): its real corpus trigger
// (T:Mode$ SpellCast | ValidCard$ Card.nonCreature | ... Execute$ TrigIncubate)
// fires on a noncreature cast and its body
// (SVar:TrigIncubate:DB$ Incubate | Amount$ TriggeredSpellAbility$CardManaCostLKI)
// incubates X, where X is that spell's mana value. Before api:Incubate was
// registered the trigger fired but the body no-opped with the generic
// "unimplemented API Incubate" Note and no token appeared.
//
// The token script is the REAL corpus tokenscript (reg.Tokens carries every
// compiled .cards/tokenscripts entry); no token text is copied into this
// file.

// seedsharkDeck builds seat 0's deck: the carrier, one authored MV-2
// noncreature spell (no targets, so the cast asks nothing), Mountains.
func seedsharkDeck(t *testing.T, reg *cards.Registry) []*cards.Card {
	t.Helper()
	src, ok := reg.Lookup("Chrome Host Seedshark")
	if !ok {
		t.Fatalf("Chrome Host Seedshark missing from corpus")
	}
	if d := src.Link(); len(d) != 0 {
		t.Fatalf("link Chrome Host Seedshark: %v", d)
	}
	spell := card(t, "Name:Test Rite\nManaCost:1 U\nTypes:Instant\nOracle:x\n")
	bear := card(t, "Name:Test Bear\nManaCost:U\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	out := []*cards.Card{src, spell, bear}
	return append(out, mountainDeck(t, 37)...)
}

// incubatorTokens returns seat p's battlefield Incubator tokens.
func incubatorTokens(t *testing.T, e *Engine, p state.PlayerID) []*state.Object {
	t.Helper()
	var out []*state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Incubator Token" {
			out = append(out, o)
		}
	}
	return out
}

func TestChromeHostSeedsharkIncubatesTheCastSpellValue(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{"a", "b"}
	decks := [][]*cards.Card{seedsharkDeck(t, reg), mountainDeck(t, 40)}
	cfg := seatZeroStart(Config{Seed: 47, Names: names, Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	shark := findByName(e, "Chrome Host Seedshark", 0)
	if shark == 0 {
		t.Fatal("precondition: Chrome Host Seedshark was not dealt")
	}
	spell := findByName(e, "Test Rite", 0)
	if spell == 0 {
		t.Fatal("precondition: Test Rite was not dealt")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: shark, From: e.G.Obj(shark).Zone, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: spell, From: e.G.Obj(spell).Zone, To: state.ZHand})
	e.priorityRound()

	// Cast the MV-2 noncreature spell (1 generic + one U).
	addMana(t, e, 0, "CU")
	castFirst(t, e, "cast")
	drainTriggerAsks(t, e, 30)
	passUntilStackEmpty(t, e, 20)

	toks := incubatorTokens(t, e, 0)
	if len(toks) != 1 {
		t.Fatalf("Seedshark's cast minted %d Incubator tokens, want 1", len(toks))
	}
	tok := toks[0]
	if tok.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Incubator token zone = %s, want battlefield", tok.Zone)
	}
	if got := tok.Counter("P1P1"); got != 2 {
		t.Fatalf("Incubator token carries %d +1/+1 counters, want 2 (the spell's mana value)", got)
	}
	if len(tok.Card.Faces) != 2 {
		t.Fatalf("precondition: Incubator token script has %d faces, want 2", len(tok.Card.Faces))
	}

	// The "{2}: Transform this token." half: fund and activate the token's
	// own ability, then let it resolve.
	addMana(t, e, 0, "CC")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending decision %+v, want priority before the transform activation", d)
	}
	castFirst(t, e, "ability")
	passUntilStackEmpty(t, e, 20)
	after := e.G.Obj(tok.ID)
	if after == nil || after.Zone != state.ZBattlefield {
		t.Fatalf("precondition: token left the battlefield (obj %+v)", after)
	}
	if after.FaceIdx != 1 || after.Face() == nil || after.Face().Name != "Phyrexian Token" {
		t.Fatalf("token did not transform: FaceIdx %d face %+v", after.FaceIdx, after.Face())
	}
	if got := after.Counter("P1P1"); got != 2 {
		t.Fatalf("transformed token carries %d +1/+1 counters, want 2 (counters survive the transform; it is a 2/2)", got)
	}
	replayCheck(t, e, cfg)
}

// TestSeedsharkCreatureCastMintsNothing guards the ValidCard$ side: a
// creature spell must not incubate.
func TestSeedsharkCreatureCastMintsNothing(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{"a", "b"}
	decks := [][]*cards.Card{seedsharkDeck(t, reg), mountainDeck(t, 40)}
	cfg := seatZeroStart(Config{Seed: 47, Names: names, Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	shark := findByName(e, "Chrome Host Seedshark", 0)
	if shark == 0 {
		t.Fatal("precondition: Chrome Host Seedshark was not dealt")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: shark, From: e.G.Obj(shark).Zone, To: state.ZBattlefield})
	e.priorityRound()

	// A creature spell (the authored Test Bear, MV 1) -- ValidCard$
	// Card.nonCreature keeps the trigger silent.
	bear := findByName(e, "Test Bear", 0)
	if bear == 0 {
		t.Fatal("precondition: Test Bear was not dealt")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: e.G.Obj(bear).Zone, To: state.ZHand})
	e.priorityRound()
	addMana(t, e, 0, "U")
	castFirst(t, e, "cast")
	drainTriggerAsks(t, e, 30)
	passUntilStackEmpty(t, e, 20)
	if toks := incubatorTokens(t, e, 0); len(toks) != 0 {
		t.Fatalf("a creature cast minted %d Incubator tokens, want 0", len(toks))
	}
	replayCheck(t, e, cfg)
}
