package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// namecardUniverse is the fixed three-card universe every test below pins:
// a nonland creature, a nonbasic land and a basic land. It is built from
// inline scripts (never committed Forge data) and linked so the type walk
// reads the printed type line.
func namecardUniverse(t *testing.T) []*cards.Card {
	t.Helper()
	return []*cards.Card{
		mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"),
		mkCard(t, "Name:Wasteland\nTypes:Land\nOracle:x\n"),
		mkCard(t, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"),
	}
}

func namecardGameWithUniverse(t *testing.T) *state.Game {
	t.Helper()
	g := state.NewGame(names(2))
	g.NameUniverse = namecardUniverse(t)
	return g
}

// TestNameChoicesFiltersBySpec pins the ONE shared builder's filter
// semantics: an empty spec is unrestricted (Pithing Needle names any card,
// a land included), Card.nonLand removes every land (Phyrexian Revoker,
// Cabal Therapy) and a compound spec restricts on both halves.
func TestNameChoicesFiltersBySpec(t *testing.T) {
	g := namecardGameWithUniverse(t)

	all := NameChoices(g, "")
	if !containsName(all, "Wasteland") || !containsName(all, "Forest") || !containsName(all, "Bear") {
		t.Fatalf("unrestricted NameChoices = %v, want Bear, Forest and Wasteland", all)
	}
	if len(all) != 3 {
		t.Fatalf("unrestricted NameChoices = %v, want exactly the 3 universe names", all)
	}

	nonland := NameChoices(g, "Card.nonLand")
	if containsName(nonland, "Wasteland") || containsName(nonland, "Forest") {
		t.Fatalf("Card.nonLand offered a land: %v", nonland)
	}
	if !containsName(nonland, "Bear") {
		t.Fatalf("Card.nonLand omitted the creature: %v", nonland)
	}

	nonbasic := NameChoices(g, "Card.Land+nonBasic")
	if containsName(nonbasic, "Forest") {
		t.Fatalf("Card.Land+nonBasic offered a basic land: %v", nonbasic)
	}
	if !containsName(nonbasic, "Wasteland") {
		t.Fatalf("Card.Land+nonBasic omitted the nonbasic land: %v", nonbasic)
	}
}

// TestNameCardAsksWithUniverseThenRecordsTheAnswer pins the mid-resolution
// effNameCard path: with a corpus universe supplied it ASKS (it no longer
// silently names the top of the caster's library), the offered list is the
// SA's own filter, and the answered name is emitted as the Choose event the
// ChosenName fold records.
func TestNameCardAsksWithUniverseThenRecordsTheAnswer(t *testing.T) {
	h := &askHost{}
	h.g = namecardGameWithUniverse(t)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0).ID
	saLine := sa(t, "SP$ NameCard | ValidCards$ Card.nonLand")

	Resolve(h, &Ctx{Controller: 0, Source: src}, saLine)
	if h.asked == nil {
		t.Fatal("NameCard did not ask with a corpus universe supplied")
	}
	d := h.asked
	if d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 || d.Player != 0 {
		t.Fatalf("name decision = %+v, want a Min==Max==1 KChoose for the controller", d)
	}
	if len(d.Options) == 0 {
		t.Fatalf("name decision offered no options")
	}
	for _, o := range d.Options {
		if o.Label == "Wasteland" || o.Label == "Forest" {
			t.Fatalf("nonland NameCard offered a land name: %+v", o)
		}
	}
	if !containsLabel(d.Options, "Bear") {
		t.Fatalf("nonland NameCard omitted Bear: %+v", d.Options)
	}
	// The precondition the assertion below depends on: no Choose was emitted
	// before the answer arrived.
	if h.g.Obj(src).ChosenName != "" {
		t.Fatalf("source named %q before the ask was answered", h.g.Obj(src).ChosenName)
	}

	// Re-entry, the engine's resume contract: Ctx.NameChoice carries the
	// answer the decision's Label held.
	Resolve(h, &Ctx{Controller: 0, Source: src, NameChoice: "Bear"}, saLine)
	if h.g.Obj(src).ChosenName != "Bear" {
		t.Fatalf("ChosenName = %q, want Bear", h.g.Obj(src).ChosenName)
	}
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.Choose && ev.Counter == "name" && ev.Text == "Bear" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no Choose name event for Bear; log=%+v", h.log)
	}
}

// TestNameCardWithoutUniverseKeepsTheR9StandIn pins the no-host degradation:
// an effects test double with no corpus still completes deterministically
// and names its own source rather than wedging on an unanswerable ask.
func TestNameCardWithoutUniverseKeepsTheR9StandIn(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0).ID
	Resolve(h, &Ctx{Controller: 0, Source: src}, sa(t, "SP$ NameCard"))
	if h.g.Obj(src).ChosenName != "Source" {
		t.Fatalf("no-universe NameCard ChosenName = %q, want the source name fallback", h.g.Obj(src).ChosenName)
	}
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func containsLabel(opts []decision.Option, want string) bool {
	for _, o := range opts {
		if o.Label == want {
			return true
		}
	}
	return false
}
