package rules

// cr-g229 review round: an Effect copies the host's chosen card name at
// REGISTRATION (Forge's EffectEffect copies the named card onto the effect
// card it creates), so a chain's `DB$ Cleanup | ClearNamedCard$ True` that
// follows it must not retract the name the effect already captured. The
// measured carriers are Cheering Fanatic's Effect-delivered ReduceCost
// (`ValidCard$ Card.NamedCard`) and Medomai's Prophecy's Effect trigger.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const namedBoltSrc = "Name:Lightning Bolt\nManaCost:0\nTypes:Instant\nOracle:x\n"

const namedShockSrc = "Name:Shock\nManaCost:0\nTypes:Instant\nOracle:x\n"

// namedEffectEngine builds a two-seat game whose seat-0 deck leads with the
// corpus protagonist and the two synthetic spells the assertions price/cast.
func namedEffectEngine(t *testing.T, reg *cards.Registry, protagonist *cards.Card, extras ...*cards.Card) (*Engine, Config) {
	t.Helper()
	first := append([]*cards.Card{protagonist}, extras...)
	cfg := seatZeroStart(Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append(first, mountainDeck(t, 40-len(first))...),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	return e, cfg
}

// TestCheeringFanaticEffectKeepsChosenName is the regression the review
// measured: the attack trigger resolves NameCard, then `DB$ Effect` registers
// the ReduceCost static, then `DBCleanup` clears the host's name. The static
// must keep pricing the name it copied (Forge's Effect card), not read the
// now-empty source.
func TestCheeringFanaticEffectKeepsChosenName(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	fanaticCard := lookup(t, reg, "Cheering Fanatic")
	fanaticSrc, ok := fanaticCard.Faces[0].SVars["DBEffect"]
	reduceSrc := fanaticCard.Faces[0].SVars["CFReduceCost"]
	if !ok || !strings.Contains(fanaticSrc, "StaticAbilities$ CFReduceCost") ||
		!strings.Contains(reduceSrc, "Card.NamedCard") ||
		!strings.Contains(fanaticCard.Faces[0].SVars["DBCleanup"], "ClearNamedCard$ True") {
		t.Fatalf("corpus precondition: Cheering Fanatic chain = %q / %q / %q, want Effect NamedCard then ClearNamedCard",
			fanaticSrc, reduceSrc, fanaticCard.Faces[0].SVars["DBCleanup"])
	}
	e, _ := namedEffectEngine(t, reg, fanaticCard, card(t, namedBoltSrc), card(t, namedShockSrc))
	fanatic := moveCorpusCard(t, e, "Cheering Fanatic", 0, state.ZBattlefield)
	bolt := moveSeeded(t, e, 0, namedBoltSrc, state.ZHand)
	shock := moveSeeded(t, e, 0, namedShockSrc, state.ZHand)

	// Seed the name through the real event the NameCard primitive emits.
	e.emit(events.Event{Kind: events.Choose, Obj: fanatic, Counter: "name", Text: "Lightning Bolt"})
	if got := e.G.Obj(fanatic).ChosenName; got != "Lightning Bolt" {
		t.Fatalf("precondition: ChosenName = %q, want Lightning Bolt", got)
	}
	face := e.G.Obj(fanatic).Face()
	effectSA := cards.ResolveSVar(face.SVars, "DBEffect")
	if effectSA == nil {
		t.Fatal("corpus precondition: DBEffect SVar does not resolve")
	}
	effects.Resolve(e, &effects.Ctx{Source: fanatic, Controller: 0, SVars: face.SVars}, effectSA)

	// The chain's own Cleanup must have wiped the host: that is the state the
	// Effect has to survive.
	if got := e.G.Obj(fanatic).ChosenName; got != "" {
		t.Fatalf("Cleanup left ChosenName = %q, want empty (precondition for the regression)", got)
	}
	mods := e.costModifiers(0, bolt, spellScope(""))
	if len(mods.Reduces) != 1 || mods.Reduces[0].Generic != 1 {
		t.Fatalf("Lightning Bolt reductions after the chain's Cleanup = %+v, want one {1}", mods.Reduces)
	}
	if other := e.costModifiers(0, shock, spellScope("")); len(other.Reduces) != 0 {
		t.Fatalf("Shock reductions = %+v, want none (the Effect copied Lightning Bolt)", other.Reduces)
	}
}

// TestMedomaiProphecyTriggerKeepsChosenName is the trigger half of the same
// class: the Effect arms a SpellCast trigger whose ValidCard$ reads
// Card.NamedCard, then Cleanup clears the host. The registration must keep the
// copied name and still fire on the named spell.
func TestMedomaiProphecyTriggerKeepsChosenName(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sagaCard := lookup(t, reg, "Medomai's Prophecy")
	effectBody, ok := sagaCard.Faces[0].SVars["DBEffect"]
	if !ok || !strings.Contains(effectBody, "NamedCardCast") || !strings.Contains(sagaCard.Faces[0].SVars["NamedCardCast"], "Card.NamedCard") {
		t.Fatalf("corpus precondition: Medomai's Prophecy Effect/trigger = %q / %q, want a NamedCard trigger",
			effectBody, sagaCard.Faces[0].SVars["NamedCardCast"])
	}
	e, _ := namedEffectEngine(t, reg, sagaCard, card(t, namedBoltSrc))
	// Keep the Saga in hand: a battlefield Saga would arm its chapter
	// triggers (chapter I is a Scry ask) and interleave with the cast under
	// test. The Effect snapshot is source-independent; the trigger body is
	// the corpus one this test asserts above.
	var saga state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Medomai's Prophecy" {
				saga = id
			}
		}
	}
	if saga == 0 {
		t.Fatal("corpus precondition: Medomai's Prophecy not in seat 0's hand/library")
	}
	if e.G.Obj(saga).Zone != state.ZHand {
		e.emit(events.Event{Kind: events.MoveZone, Obj: saga, From: e.G.Obj(saga).Zone, To: state.ZHand})
		e.pending = nil
		e.priorityRound()
	}
	bolt := moveSeeded(t, e, 0, namedBoltSrc, state.ZHand)

	e.emit(events.Event{Kind: events.Choose, Obj: saga, Counter: "name", Text: "Lightning Bolt"})
	face := e.G.Obj(saga).Face()
	effects.Resolve(e, &effects.Ctx{Source: saga, Controller: 0, SVars: face.SVars}, cards.ResolveSVar(face.SVars, "DBEffect"))
	if got := e.G.Obj(saga).ChosenName; got != "" {
		t.Fatalf("Cleanup left ChosenName = %q, want empty", got)
	}
	var dt *state.DelayedTrigger
	for i := range e.G.Delayed {
		if e.G.Delayed[i].Source == saga && e.G.Delayed[i].EventMode == "SpellCast" {
			dt = &e.G.Delayed[i]
			break
		}
	}
	if dt == nil {
		t.Fatal("Medomai's Prophecy Effect registered no SpellCast trigger")
	}
	if !dt.ChosenNameFrozen || dt.ChosenName != "Lightning Bolt" {
		t.Fatalf("trigger snapshot = (%q, frozen=%v), want (\"Lightning Bolt\", true)", dt.ChosenName, dt.ChosenNameFrozen)
	}

	e.pending = nil
	e.priorityRound()
	hand := len(e.G.Zone(state.ZHand, 0))
	castAndResolve(t, e, bolt)
	// Bolt left the hand and the named-spell trigger drew two.
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand after casting the named spell = %d, want %d (bolt cast, two drawn)", got, hand+1)
	}
}
