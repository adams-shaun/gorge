package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Aura token entry path must consult the live Aura-shape CantAttach
// statics (cards.CantAttachAuraShape) exactly like the spell path (the target
// census) and the CR 303.4f card-entry path (auraEnchantCandidates with a real
// entering object) do: the token half judges its candidates before the token
// exists, so its aura side is satisfied by construction and only the bearer
// side is judged -- an Aura token whose only candidate is a bearer those
// statics protect is not created (CR 303.4g), and a token minted with the
// protected bearer NAMED never attaches to it.

// TestAuraTokenCannotEnterOntoCantAttachBearer pins the TokenCreate gate: with
// Anti-Magic Aura on the battlefield's only creature, a Cursed Role mint has
// no legal answer left, so the token is not created -- it used to mint
// unattached, because the gate's candidate walk passed no entering object and
// so never asked the CantAttach statics.
func TestAuraTokenCannotEnterOntoCantAttachBearer(t *testing.T) {
	t.Parallel()
	antimagic := corpusCardNamed(t, "Anti-Magic Aura")
	creCard := card(t, testVanillaCre)
	e, cfg := tokenReplGame(t, 7721, antimagic, creCard)
	cre := putCreature(t, e, 0, testVanillaCre)
	id := castAuraOnto(t, e, antimagic, cre, "UUU")
	if got := e.G.Obj(id).AttachedTo; got != cre {
		t.Fatalf("precondition: anti-magic aura attached to %d, want %d", got, cre)
	}
	if _, ok := e.G.Tokens["role_cursed"]; !ok {
		t.Skip("role_cursed token script not in the corpus")
	}
	before := e.G.NextID
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "role_cursed"})
	if e.G.NextID != before {
		t.Fatalf("the Role token was minted although Anti-Magic Aura forbids enchanting its only candidate (NextID %d -> %d)",
			before, e.G.NextID)
	}
	if !hasNote(e, "isn't created (CR 303.4g)") {
		t.Error("no CR 303.4g note for the withheld token")
	}
	replayCheck(t, e, cfg)
}

// TestAuraTokenNamedCantAttachBearerIsNotAttachedTo pins the effect-named
// mint: with a second, legal creature on the battlefield the Cursed Courtier's
// "create a Cursed Role token attached to it" still mints the token (CR 303.4g
// has nothing to withhold), but the token must never enter attached to the
// bearer the Anti-Magic Aura protects -- a cast Aura spell is refused that
// bearer, and so is the token's Attach.
func TestAuraTokenNamedCantAttachBearerIsNotAttachedTo(t *testing.T) {
	t.Parallel()
	antimagic := corpusCardNamed(t, "Anti-Magic Aura")
	courtierCard := corpusCardNamed(t, "Cursed Courtier")
	creCard := card(t, testVanillaCre)
	e, cfg := tokenReplGame(t, 7722, antimagic, courtierCard, creCard)
	toMain1(t, e)
	courtier := auraMoveCorpusCard(t, e, 0, "Cursed Courtier", state.ZBattlefield)
	if len(e.pendingTriggers) == 0 {
		t.Fatal("precondition: the Courtier's ETB trigger did not queue")
	}
	// The static goes live while the trigger is still pending: the Anti-Magic
	// Aura enters the non-cast way and, the Courtier being its only legal
	// candidate, settles attached to it without an ask.
	id := auraMoveCorpusCard(t, e, 0, "Anti-Magic Aura", state.ZBattlefield)
	if got := e.G.Obj(id).AttachedTo; got != courtier {
		t.Fatalf("precondition: anti-magic aura attached to %d, want %d", got, courtier)
	}
	if len(e.pendingTriggers) == 0 {
		t.Fatal("precondition: the Courtier's ETB trigger resolved before the static went live")
	}
	moveSeededCard(t, e, 0, creCard, state.ZBattlefield)
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	var role *state.Object
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.IsToken && o.Face() != nil && strings.Contains(o.Face().Name, "Cursed") {
			role = o
		}
	}
	if role == nil {
		t.Fatal("no Cursed Role token was minted although a legal bearer existed")
	}
	if role.AttachedTo == courtier {
		t.Fatalf("the Role token entered attached to the bearer Anti-Magic Aura protects (zone %s)", role.Zone)
	}
	if role.Zone != state.ZCeased {
		t.Fatalf("unattached Role token zone = %v, want the CR 704.5m sweep (a token ceases to exist, CR 111.7)", role.Zone)
	}
	if !hasNote(e, "cannot attach") {
		t.Error("no refusal note for the protected bearer")
	}
	replayCheck(t, e, cfg)
}
