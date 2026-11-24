package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// defined_x_mana_test.go pins CR 107.3a/c for an activated ability whose
// printed {X} the text defines (the face's SVar X is not Count$xPaid): the
// controller announces nothing, and the cost is that value of generic mana.
// Soul Foundry ("X is the mana value of that card",
// SVar:X:Imprinted$CardManaCost) and Voodoo Doll ({X}{X}, "X is the number
// of pin counters", SVar:X:Count$CardCounters.PIN). pay.FixLifeXCost folds
// the defined {X} into Generic.

const definedXBearSrc = "Name:Defined X Bear\nManaCost:2 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// definedXGame builds a two-seat game at seat 0's first main phase with the
// corpus card src on the battlefield and lands untapped Mountains beside it.
func definedXGame(t *testing.T, name string, lands int) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	src := mustCorpusCard(t, reg, name)
	bear := card(t, definedXBearSrc)
	cfg := seatZeroStart(Config{Seed: 11, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{bear}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	srcID := onBoardReadyCard(t, e, 0, src)
	placed := 0
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if placed == lands {
			break
		}
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
			placed++
		}
	}
	bearID := findByName(e, "Defined X Bear", 0)
	if bearID == 0 {
		t.Fatal("fixture Bear not in seat 0's zones")
	}
	e.pending = nil
	e.Advance()
	if got := untappedNamed(e, 0, "Mountain"); got != lands {
		t.Fatalf("setup: %d untapped Mountains, want %d", got, lands)
	}
	return e, srcID, bearID
}

func untappedNamed(e *Engine, p state.PlayerID, name string) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name && !o.Tapped {
			n++
		}
	}
	return n
}

// activateDefinedX activates srcID's ability 0 and answers every decision
// the activation poses (target: the opponent), failing on an X announcement.
// It returns the number of Mountains the payment tapped.
func activateDefinedX(t *testing.T, e *Engine, srcID state.ObjID) int {
	t.Helper()
	before := untappedNamed(e, 0, "Mountain")
	opt := abilityOption(t, e, srcID, 0)
	submitChoices(t, e, opt.Index)
	for range 8 {
		d := e.Pending()
		if d == nil || d.Player != 0 || e.cast == nil {
			break
		}
		pick := -1
		for _, o := range d.Options {
			if o.Kind == "x" {
				t.Fatalf("activation posed an X announcement (%q, %d options); the text defines X (CR 107.3c)", d.Prompt, len(d.Options))
			}
			if pick < 0 && o.Kind == "player" && o.Player == 1 {
				pick = o.Index
			}
		}
		if pick < 0 && len(d.Options) > 0 {
			pick = d.Options[0].Index
		}
		if pick < 0 {
			t.Fatalf("unanswerable decision during activation: %+v", d)
		}
		submitChoices(t, e, pick)
	}
	if e.cast != nil {
		t.Fatalf("activation still pending after the decision budget: %+v", e.Pending())
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the ability is not on the stack: the activation never completed")
	}
	return before - untappedNamed(e, 0, "Mountain")
}

func TestSoulFoundryDefinedX(t *testing.T) {
	t.Parallel()
	t.Run("nothing imprinted", func(t *testing.T) {
		t.Parallel()
		e, foundry, _ := definedXGame(t, "Soul Foundry", 4)
		if got := e.G.Obj(foundry).Face().SVars["X"]; got != "Imprinted$CardManaCost" {
			t.Fatalf("corpus Soul Foundry SVar X = %q, want Imprinted$CardManaCost", got)
		}
		if paid := activateDefinedX(t, e, foundry); paid != 0 {
			t.Fatalf("paid %d mana with nothing imprinted, want 0 (X is 0, CR 107.3c)", paid)
		}
		if !e.G.Obj(foundry).Tapped {
			t.Fatal("Soul Foundry untapped after activating: the {T} part was never paid")
		}
	})
	t.Run("three-drop imprinted", func(t *testing.T) {
		t.Parallel()
		e, foundry, bear := definedXGame(t, "Soul Foundry", 5)
		e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: e.G.Obj(bear).Zone, To: state.ZExile})
		e.emit(events.Event{Kind: events.Imprint, Obj: foundry, IDs: []state.ObjID{bear}})
		e.pending = nil
		e.Advance()
		if o := e.G.Obj(foundry); len(o.Imprinted) != 1 || o.Imprinted[0] != bear {
			t.Fatalf("setup: Soul Foundry imprinted %v, want [%d]", o.Imprinted, bear)
		}
		if paid := activateDefinedX(t, e, foundry); paid != 3 {
			t.Fatalf("paid %d mana, want exactly 3 (the imprinted card's mana value)", paid)
		}
	})
}

func TestVoodooDollDefinedX(t *testing.T) {
	t.Parallel()
	e, doll, _ := definedXGame(t, "Voodoo Doll", 6)
	if got := e.G.Obj(doll).Face().SVars["X"]; got != "Count$CardCounters.PIN" {
		t.Fatalf("corpus Voodoo Doll SVar X = %q, want Count$CardCounters.PIN", got)
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: doll, Counter: "PIN", Amount: 2})
	e.pending = nil
	e.Advance()
	if got := e.G.Obj(doll).Counter("PIN"); got != 2 {
		t.Fatalf("setup: %d pin counters, want 2", got)
	}
	if paid := activateDefinedX(t, e, doll); paid != 4 {
		t.Fatalf("paid %d mana, want exactly 4 ({X}{X} with X = 2 pin counters)", paid)
	}
	if !e.G.Obj(doll).Tapped {
		t.Fatal("Voodoo Doll untapped after activating: the {T} part was never paid")
	}
}

// TestReflectingMirrorDefinedX pins CR 107.3c/601.2f for Reflecting Mirror's
// "{X}, {T}" ability: its SVar:X is TargetBound (Targeted$CardManaCost/Twice),
// so the value is the target spell's mana value twice, fixed when the cost is
// determined. Nothing is announced, and the offer gate prices the folded
// cost, so five Mountains cannot activate it and six can.
func TestReflectingMirrorDefinedX(t *testing.T) {
	t.Parallel()
	const probeSrc = "Name:Defined X Probe\nTypes:Sorcery\nManaCost:2 R\nA:SP$ DealDamage | ValidTgts$ Player | NumDmg$ 1\nOracle:x\n"
	setup := func(t *testing.T, lands int) (*Engine, state.ObjID, state.ObjID) {
		t.Helper()
		e, mirror, _ := definedXGame(t, "Reflecting Mirror", lands)
		spell := e.G.AddObject(card(t, probeSrc), 1)
		e.emit(events.Event{Kind: events.MoveZone, Obj: spell.ID, From: state.ZLibrary, To: state.ZStack})
		e.emit(events.Event{Kind: events.TargetsChosen, Obj: spell.ID, Player: 0, Amount: 1})
		e.pending = nil
		e.Advance()
		if got := e.G.Obj(spell.ID).Targets; len(got) != 1 || !got[0].IsPlayer || got[0].Player != 0 {
			t.Fatalf("setup: stack spell targets = %+v, want player 0", got)
		}
		if got := e.G.Obj(spell.ID).Face().ManaValue(); got != 3 {
			t.Fatalf("setup: probe mana value = %d, want 3", got)
		}
		return e, mirror, spell.ID
	}
	t.Run("no announcement and the folded cost is paid", func(t *testing.T) {
		t.Parallel()
		e, mirror, spell := setup(t, 6)
		var pc cost.Cost
		for _, sa := range e.G.Obj(mirror).Face().Abilities {
			if cc := pay.AbilityCostOf(sa); cc.Cost.X > 0 {
				pc = cc.Cost
				break
			}
		}
		if pc.X != 1 {
			t.Fatalf("setup: printed ability X = %d, want 1 (Cost$ X T)", pc.X)
		}
		addMana(t, e, 0, "RRRRRR")
		if got := e.G.Players[0].Pool.Total(); got != 6 {
			t.Fatalf("setup: pool = %d, want 6", got)
		}
		opt := abilityOption(t, e, mirror, 0)
		submitChoices(t, e, opt.Index)
		d := e.Pending()
		if d == nil {
			t.Fatal("activating the Mirror posed no decision: no target ask")
		}
		for _, o := range d.Options {
			if o.Kind == "x" {
				t.Fatalf("activation posed an X announcement (%q, %d options); the text defines X (CR 107.3c)", d.Prompt, len(d.Options))
			}
		}
		pick := -1
		for _, o := range d.Options {
			if o.Obj == spell {
				pick = o.Index
				break
			}
		}
		if pick < 0 {
			t.Fatalf("no target option for the stack spell %d: %+v", spell, d.Options)
		}
		submitChoices(t, e, pick)
		for range 8 {
			d := e.Pending()
			if d == nil || d.Player != 0 || e.cast == nil {
				break
			}
			if len(d.Options) == 0 {
				break
			}
			submitChoices(t, e, d.Options[0].Index)
		}
		if e.cast != nil {
			t.Fatalf("activation still pending: %+v", e.Pending())
		}
		if got := e.G.Players[0].Pool.Total(); got != 0 {
			t.Fatalf("pool after activation = %v, want empty (the folded 6 was paid)", e.G.Players[0].Pool)
		}
		if !e.G.Obj(mirror).Tapped {
			t.Fatal("Reflecting Mirror untapped after activating: the {T} part was never paid")
		}
	})
	t.Run("offer prices the folded x", func(t *testing.T) {
		t.Parallel()
		e5, mirror5, _ := setup(t, 5)
		if _, ok := findAbilityOption(e5, mirror5, 0); ok {
			t.Fatal("Mirror offered with five untapped Mountains: X folds to 6 (2 x mana value 3)")
		}
		e6, mirror6, _ := setup(t, 6)
		if _, ok := findAbilityOption(e6, mirror6, 0); !ok {
			t.Fatal("Mirror not offered with six untapped Mountains: the folded X = 6 is payable")
		}
	})
}

// TestGoblinPolkaBandKeepsAnnouncedX pins the other half of CR 107.3a: a
// printed {X} whose SVar:X is late-bound (SVar$TgtNum, the ability's own
// target count) is not defined at activation, so it keeps the announcement
// as before and is neither folded nor withheld. The ability also carries
// XColor$ R, which the fold excludes: that exclusion must not turn the old
// announcement into a withhold.
func TestGoblinPolkaBandKeepsAnnouncedX(t *testing.T) {
	t.Parallel()
	e, band, _ := definedXGame(t, "Goblin Polka Band", 3)
	var pc cost.Cost
	for _, sa := range e.G.Obj(band).Face().Abilities {
		if cc := pay.AbilityCostOf(sa); cc.Cost.X > 0 {
			pc = cc.Cost
			break
		}
	}
	if pc.X == 0 || pc.XColors == 0 {
		t.Fatalf("setup: X=%d XColors=%#b, want the printed {X} with its XColor$ R", pc.X, pc.XColors)
	}
	out, ok := pay.FixLifeXCost(asPayer(e), 0, band, pc)
	if !ok {
		t.Fatal("the late-bound {X} is withheld: it must keep the announcement it had")
	}
	if out.X != pc.X || out.XColors != pc.XColors || out.Generic != pc.Generic {
		t.Fatalf("cost changed: %+v -> %+v; a late-bound {X} keeps its announcement", pc, out)
	}
}

// TestDeepwoodElderKeepsAnnouncedX pins the boundary of the target-bound
// shape: Deepwood Elder's SVar:X is TargetedObjects$Amount (a count of the
// ability's own targets), not Targeted$..., so it stays announced. Both the
// activation fold and the deferred offer leave the corpus cost unchanged.
func TestDeepwoodElderKeepsAnnouncedX(t *testing.T) {
	t.Parallel()
	e, elder, _ := definedXGame(t, "Deepwood Elder", 3)
	var pc cost.Cost
	var ab *cards.SA
	for _, sa := range e.G.Obj(elder).Face().Abilities {
		if cc := pay.AbilityCostOf(sa); cc.Cost.X > 0 {
			pc, ab = cc.Cost, sa
			break
		}
	}
	if ab == nil || pc.X == 0 {
		t.Fatal("setup: Deepwood Elder has no printed-X ability")
	}
	if got := e.G.Obj(elder).Face().SVars["X"]; got != "TargetedObjects$Amount" {
		t.Fatalf("setup: SVar X = %q, want TargetedObjects$Amount", got)
	}
	out, ok := pay.FixLifeXCost(asPayer(e), 0, elder, pc)
	if !ok {
		t.Fatal("the target-count {X} is withheld: it must keep the announcement it had")
	}
	if out.X != pc.X || out.XColors != pc.XColors || out.Generic != pc.Generic {
		t.Fatalf("FixLifeXCost changed the cost: %+v -> %+v", pc, out)
	}
	folded, symbols, foldedN, ok := deferredManaXOffer(asPayer(e), e, 0, elder, ab, 0, pc)
	if !ok {
		t.Fatal("deferredManaXOffer withheld the target-count {X}")
	}
	if symbols != 0 || foldedN != 0 {
		t.Fatalf("deferredManaXOffer folded %d symbols / %d generic, want none", symbols, foldedN)
	}
	if folded.X != pc.X || folded.XColors != pc.XColors || folded.Generic != pc.Generic {
		t.Fatalf("deferredManaXOffer changed the cost: %+v -> %+v", pc, folded)
	}
}
