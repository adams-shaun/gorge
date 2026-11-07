package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSourceRememberedStrictScopesIsRemembered is the effect-registration half
// of the Remembered union rule. A registered continuous effect's
// Card.IsRemembered scope is the set the Effect captured (CR 611.2), so a card
// that later remembers another object for an unrelated ability must not widen
// that scope. The source's event-backed list is still the ordinary union for a
// resolution read, which the second assertion pins as the precondition.
func TestSourceRememberedStrictScopesIsRemembered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	source := corpusObject(t, reg, g, "Snowmelt Stag")
	later := corpusObject(t, reg, g, "Grizzly Bears")

	// Precondition: the source's persistent list holds BOTH the object the
	// Effect captured and an object a later ability remembered (Mobilize's
	// RememberTokens$ appends its Warrior here). g.Obj is re-read because
	// AddObject's returned pointer is stale once a later AddObject grows the
	// object slice.
	g.Obj(source.ID).Remembered = []state.Target{{Obj: source.ID}, {Obj: later.ID}}
	captured := SpecContext{Source: source.ID, Remembered: []state.Target{{Obj: source.ID}}}
	if !MatchesObjectCtx(g, "Card.IsRemembered", source, captured) {
		t.Fatal("precondition: the captured object is not remembered by the non-strict read")
	}
	if !MatchesObjectCtx(g, "Card.IsRemembered", later, captured) {
		t.Fatal("precondition: the union must see the source's later persistent remember")
	}

	// The registered effect's strict read answers only its own capture.
	captured.SourceRememberedStrict = true
	if !MatchesObjectCtx(g, "Card.IsRemembered", source, captured) {
		t.Error("strict IsRemembered lost the Effect's captured object")
	}
	if MatchesObjectCtx(g, "Card.IsRemembered", later, captured) {
		t.Error("strict IsRemembered read the source's persistent list and widened the Effect's scope")
	}
}
