package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// recordDamageProvenance folds one DamageProvenance fact through the shared
// events.Apply fold, exactly as rules.Engine.emit emits it post-fold for a
// landed Damage event. Tests use it to set the game-long record up the way a
// real damage route would, without depending on the rules tier.
func recordDamageProvenance(h *fakeHost, source, recipient state.ObjID) {
	events.Apply(h.g, events.Event{Kind: events.DamageProvenance, Obj: source,
		IDs: []state.ObjID{recipient}, Amount: 1})
}

// TestDamageAllValidPlayersTheFallenResolves closes The Fallen's exotic
// compound (brief game-long damage-by-source provenance):
// `ValidPlayers$ Player.Opponent+wasDealtDamageThisGameBy Self` and
// `ValidCards$ Planeswalker.wasDealtDamageByThisGame`. The engine now keeps a
// game-long (recipient, source) record (events.DamageProvenance ->
// state.Player/Object.DamageTakenByGame), so the selector resolves: the sweep
// damages exactly the opponent The Fallen has damaged this game and the
// planeswalker it has damaged, and NOT an opponent it never damaged. Before
// the fix the compound failed closed (nobody) and the walker word was unknown.
func TestDamageAllValidPlayersTheFallenResolves(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	fallen, ok := reg.Lookup("The Fallen")
	if !ok {
		t.Fatal("corpus has no The Fallen")
	}
	face := fallen.Faces[0]
	// Precondition on the real script: the line still spells the compound
	// this test closes, on both halves.
	if line := face.SVars["TrigDamage"]; !strings.Contains(line, "Player.Opponent+wasDealtDamageThisGameBy Self") ||
		!strings.Contains(line, "Planeswalker.wasDealtDamageByThisGame") {
		t.Fatalf("corpus moved: The Fallen's TrigDamage no longer carries the compound: %q", line)
	}
	trig := svarSA(t, face, "TrigDamage")

	h := newHost(t, 3)
	src := putOnBattlefield(t, h.g, reg, "The Fallen", 0)
	// A planeswalker controlled by opponent seat 1, on the battlefield with
	// loyalty > 0 (its precondition for the ValidCards sweep).
	walker := putOnBattlefield(t, h.g, reg, "Jace Beleren", 1)
	walker.AddCounter("LOYALTY", 3)
	if walker.Face() == nil || !walker.Face().IsPlaneswalker() {
		t.Fatalf("precondition: Jace Beleren is not a planeswalker face (%v)", walker.Face())
	}
	if walker.Counter("LOYALTY") <= 0 {
		t.Fatalf("precondition: the walker has no loyalty to lose (%d)", walker.Counter("LOYALTY"))
	}
	// The Fallen's game-long record: it has damaged seat 1 and seat 1's
	// walker this game, but never seat 2.
	recordDamageProvenance(h, src.ID, state.PlayerRef(1))
	recordDamageProvenance(h, src.ID, walker.ID)
	if len(h.g.Players[1].DamageTakenByGame) == 0 {
		t.Fatal("precondition: seat 1's game-long record is empty after recording")
	}
	life2Before := h.g.Players[2].Life
	loyaltyBefore := walker.Counter("LOYALTY")

	c := &Ctx{Source: src.ID, Controller: 0, SVars: face.SVars}
	Resolve(h, c, trig)

	if h.g.Players[1].Life != 19 {
		t.Fatalf("the damaged opponent must take The Fallen's 1 damage, life %d want 19", h.g.Players[1].Life)
	}
	if got := walker.Counter("LOYALTY"); got != loyaltyBefore-1 {
		t.Fatalf("the damaged walker must lose 1 loyalty (%d -> %d, want %d)", loyaltyBefore, got, loyaltyBefore-1)
	}
	if h.g.Players[2].Life != life2Before {
		t.Fatalf("the never-damaged opponent must take nothing, life %d want %d", h.g.Players[2].Life, life2Before)
	}
	for _, e := range h.log {
		if e.Kind == events.Note && (strings.Contains(e.Text, "unresolved") || strings.Contains(e.Text, "unimplemented")) {
			t.Fatalf("no unresolved/unimplemented note may accompany a resolved selector, got %+v", e)
		}
	}
}

// TestPlayerDamageByRefThisGameFailsClosedOnUnboundRef pins the player
// qualifier's fail-closed boundary: with no source bound, `Self` resolves to
// no referent and the qualifier matches no seat -- including under the
// compound spelling, and never by accident from a shared record. It also
// asserts the positive reading works so the test cannot pass with the
// qualifier unregistered.
func TestPlayerDamageByRefThisGameFailsClosedOnUnboundRef(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:S\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	recordDamageProvenance(h, src.ID, state.PlayerRef(1))
	if len(h.g.Players[1].DamageTakenByGame) == 0 {
		t.Fatal("precondition: seat 1's game-long record is empty after recording")
	}
	// Bound source: seat 1 qualifies, seat 0 (the source's controller and no
	// recipient) does not.
	pc := PlayerSpecCtx{Source: src.ID}
	if !MatchesPlayerSpecCtx(h.g, "Opponent.wasDealtDamageThisGameBy Self", 1, 0, pc) {
		t.Fatal("with the source bound, the damaged opponent must qualify")
	}
	if MatchesPlayerSpecCtx(h.g, "Opponent.wasDealtDamageThisGameBy Self", 0, 0, pc) {
		t.Fatal("the undamaged source controller must not qualify")
	}
	// Unbound source (the MatchesPlayerSpec entry point): fail closed.
	if MatchesPlayerSpec(h.g, "Opponent.wasDealtDamageThisGameBy Self", 1, 0) {
		t.Fatal("an unbound Self must match nobody, not the damaged seat")
	}
	// The bare compound spelling used by The Fallen resolves the same way.
	if !MatchesPlayerSpecFrom(h.g, "Player.Opponent+wasDealtDamageThisGameBy Self", 1, 0, src.ID) {
		t.Fatal("the bare compound clause must resolve through the same shared record")
	}
}

// TestDamageProvenanceWordsAreClassified pins that both spellings are
// recognised by the shared classifier (so UnknownPredicates reports neither)
// and that the unbound source is refused under '!' as well as positively --
// a recognised-but-false body would let `!wasDealtDamageByThisGame` match
// every object when no source is bound.
func TestDamageProvenanceWordsAreClassified(t *testing.T) {
	for _, spec := range []string{
		"Creature.wasDealtDamageByThisGame",
		"Planeswalker.wasDealtDamageThisGameBy Self",
	} {
		if got := UnknownPredicates(spec); len(got) != 0 {
			t.Fatalf("spec %q reports unknown predicates %v, want none", spec, got)
		}
	}
	h := newHost(t, 2)
	o := h.g.AddObject(mkCard(t, "Name:P\nTypes:Planeswalker\nOracle:x\n"), 1)
	o.AddCounter("LOYALTY", 3)
	// No source bound: both the positive and the negated spelling must fail
	// closed (ok=false), never invert the absence into a match.
	sc := SpecContext{You: 0}
	if got, ok := matchPredicate(h.g, "wasDealtDamageByThisGame", o, sc); ok {
		t.Fatalf("unbound bare word must be unbound (ok=false), got result=%v ok=%v", got, ok)
	}
	if _, ok := matchPredicate(h.g, "!wasDealtDamageByThisGame", o, sc); ok {
		t.Fatal("unbound negated bare word must fail closed (ok=false)")
	}
	if _, ok := matchPredicate(h.g, "!wasDealtDamageThisGameBy Self", o, sc); ok {
		t.Fatal("unbound negated argument word must fail closed (ok=false)")
	}
}
