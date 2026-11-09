package rules

// A permanent that enters as a copy of an Aura (Copy Enchantment's as-enters
// clone) is a non-cast Aura entrant (CR 303.4f, 707.2, 614.12): its
// controller chooses what it enchants as it enters.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type copyAuraFixture struct {
	e                        *Engine
	cfg                      Config
	copyID, fist, lion, bear state.ObjID
	extra                    []state.ObjID
	ask                      *decision.Decision
}

// copyAuraSetup casts Copy Enchantment with Inferno Fist on seat 0's Lion
// (and the Bears too when withBears, and each extra creature) and returns at
// the clone election.
func copyAuraSetup(t *testing.T, withBears bool, extra ...string) copyAuraFixture {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	names := append([]string{"Copy Enchantment", "Inferno Fist", "Silvercoat Lion", "Grizzly Bears"}, extra...)
	e, cfg := searchEngine(t, reg, names...)
	f := copyAuraFixture{e: e, cfg: cfg}
	f.lion = searchMoveByName(t, e, "Silvercoat Lion", state.ZBattlefield)
	if withBears {
		f.bear = searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	}
	for _, name := range extra {
		f.extra = append(f.extra, searchMoveByName(t, e, name, state.ZBattlefield))
	}
	f.fist = searchMoveByName(t, e, "Inferno Fist", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: f.fist, IDs: []state.ObjID{f.lion}})
	f.copyID = searchMoveByName(t, e, "Copy Enchantment", state.ZHand)
	if o := e.G.Obj(f.fist); o.Zone != state.ZBattlefield || o.AttachedTo != f.lion {
		t.Fatalf("precondition: Inferno Fist not attached to the Lion: %+v", o)
	}
	if d := e.Derived(f.lion); d.Power != 4 {
		t.Fatalf("precondition: Lion with one Fist is %d power, want 4", d.Power)
	}
	addMana(t, e, 0, "UUU")
	castCardByName(t, e, f.copyID)
	f.ask = passUntilAsk(t, e)
	if f.ask == nil || f.ask.Kind != decision.KChoose || f.ask.ResumeKind != "etb" {
		t.Fatalf("Copy Enchantment did not pose its clone election: %+v", f.ask)
	}
	return f
}

func copyAuraCloneOpt(d *decision.Decision, tpl state.ObjID) int {
	for _, o := range d.Options {
		if o.Kind == "clone" && o.Obj == tpl && tpl != 0 {
			return o.Index
		}
	}
	return -1
}

func TestCopyEnchantmentCopyingAuraAsksEnchant(t *testing.T) {
	t.Parallel()
	f := copyAuraSetup(t, true)
	e := f.e
	idx := copyAuraCloneOpt(f.ask, f.fist)
	if idx < 0 {
		t.Fatalf("clone election does not offer Inferno Fist: %+v", f.ask.Options)
	}
	cloneAsks := 1
	submitChoices(t, e, idx)
	d := passUntilAsk(t, e)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("no enchant ask after the clone answer: %+v", d)
	}
	var lion, bear bool
	for _, o := range d.Options {
		if o.Kind == "clone" {
			cloneAsks++
		}
		if o.Kind != "enchant" {
			t.Fatalf("expected only enchant options, got %+v", d.Options)
		}
		lion = lion || o.Obj == f.lion
		bear = bear || o.Obj == f.bear
	}
	if !lion || !bear || len(d.Options) != 2 {
		t.Fatalf("enchant options must offer exactly Lion and Bears: %+v", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == f.bear {
			submitChoices(t, e, o.Index)
		}
	}
	passUntilStackEmpty(t, e, 60)

	co := e.G.Obj(f.copyID)
	if co == nil || co.Zone != state.ZBattlefield || co.AttachedTo != f.bear {
		t.Fatalf("copy should sit on the battlefield attached to the Bears: %+v", co)
	}
	if p := e.Derived(f.bear).Power; p != 4 {
		t.Fatalf("Bears power %d, want 4 (2 + 2 from the copy)", p)
	}
	if p := e.Derived(f.lion).Power; p != 4 {
		t.Fatalf("Lion power %d, want 4 (only the original Fist)", p)
	}
	if cloneAsks != 1 {
		t.Fatalf("clone election asked %d times, want once", cloneAsks)
	}
	replayCheck(t, e, f.cfg)
}

func TestCopyEnchantmentCopyingAuraSingleBearerAttaches(t *testing.T) {
	t.Parallel()
	f := copyAuraSetup(t, false)
	e := f.e
	if n := len(e.G.Zone(state.ZBattlefield, 0)); n < 2 {
		t.Fatalf("precondition: fixture battlefield has %d permanents", n)
	}
	idx := copyAuraCloneOpt(f.ask, f.fist)
	if idx < 0 {
		t.Fatalf("clone election does not offer Inferno Fist: %+v", f.ask.Options)
	}
	submitChoices(t, e, idx)
	if d := passUntilAsk(t, e); d != nil && d.ResumeKind == "etb" {
		t.Fatalf("unexpected ask with a single legal bearer: %+v", d)
	}
	passUntilStackEmpty(t, e, 60)
	co := e.G.Obj(f.copyID)
	if co == nil || co.Zone != state.ZBattlefield || co.AttachedTo != f.lion {
		t.Fatalf("copy should be attached to the Lion: %+v", co)
	}
	if p := e.Derived(f.lion).Power; p != 6 {
		t.Fatalf("Lion power %d, want 6", p)
	}
	replayCheck(t, e, f.cfg)
}

func TestCopyEnchantmentDeclineEntersPlain(t *testing.T) {
	t.Parallel()
	f := copyAuraSetup(t, true)
	e := f.e
	decline := -1
	for _, o := range f.ask.Options {
		if o.Kind == "clone" && o.Obj == 0 {
			decline = o.Index
		}
	}
	if decline < 0 {
		t.Fatalf("decline not offered: %+v", f.ask.Options)
	}
	submitChoices(t, e, decline)
	if d := passUntilAsk(t, e); d != nil && d.ResumeKind == "etb" {
		t.Fatalf("unexpected ask after declining: %+v", d)
	}
	passUntilStackEmpty(t, e, 60)
	co := e.G.Obj(f.copyID)
	if co == nil || co.Zone != state.ZBattlefield || co.AttachedTo != 0 || co.Face().Name != "Copy Enchantment" {
		t.Fatalf("declined copy should be a plain unattached Copy Enchantment: %+v", co)
	}
	replayCheck(t, e, f.cfg)
}

func TestCopyEnchantmentCopyingAuraNoBearerGoesToGraveyard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	mountain := searchCorpusCard(t, reg, "Mountain")
	deck0 := make([]*cards.Card, 40)
	deck1 := make([]*cards.Card, 40)
	for i := range deck0 {
		deck0[i], deck1[i] = mountain, mountain
	}
	deck0[0] = searchCorpusCard(t, reg, "Copy Enchantment")
	deck1[0] = searchCorpusCard(t, reg, "Inferno Fist")
	deck1[1] = searchCorpusCard(t, reg, "Silvercoat Lion")
	cfg := Config{Seed: 9202, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck0, deck1},
		Tokens: reg.Tokens, NameUniverse: reg.Universe()}
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	lion := searchMoveByNameSeat(t, e, 1, "Silvercoat Lion", state.ZBattlefield)
	fist := searchMoveByNameSeat(t, e, 1, "Inferno Fist", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: fist, IDs: []state.ObjID{lion}})
	cp := searchMoveByName(t, e, "Copy Enchantment", state.ZHand)
	if o := e.G.Obj(fist); o.Zone != state.ZBattlefield || o.AttachedTo != lion || o.Controller != 1 {
		t.Fatalf("precondition: opponent's Fist should be attached to its Lion: %+v", o)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Face().IsCreature() {
			t.Fatalf("precondition: seat 0 controls a creature: %s", o.Face().Name)
		}
	}
	addMana(t, e, 0, "UUU")
	castCardByName(t, e, cp)
	d := passUntilAsk(t, e)
	if d == nil || d.ResumeKind != "etb" {
		t.Fatalf("no clone election: %+v", d)
	}
	idx := copyAuraCloneOpt(d, fist)
	if idx < 0 {
		t.Fatalf("clone election does not offer Inferno Fist: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	if d := passUntilAsk(t, e); d != nil && d.ResumeKind == "etb" {
		t.Fatalf("unexpected ask with no legal bearer: %+v", d)
	}
	passUntilStackEmpty(t, e, 60)
	if o := e.G.Obj(cp); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("unattached copy should be put into the graveyard: %+v", o)
	}
	entered := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == cp && ev.From == state.ZStack && ev.To == state.ZBattlefield {
			entered = true
		}
		if ev.Kind == events.Note && ev.Obj == cp {
			t.Fatalf("a withhold note was emitted: %+v", ev)
		}
	}
	if !entered {
		t.Fatal("the copy never entered the battlefield from the stack")
	}
	replayCheck(t, e, cfg)
}

// TestCopyEnchantmentCopyingRedAuraProtection: the copy of Inferno Fist is red
// as it enters (CR 707.2, 614.12), so protection from blue (Copy
// Enchantment's printed colour) does not stop it enchanting Goblin
// Piledriver, while protection from red does stop Auriok Champion (CR
// 702.16c).
func TestCopyEnchantmentCopyingRedAuraProtection(t *testing.T) {
	t.Parallel()
	f := copyAuraSetup(t, false, "Goblin Piledriver", "Auriok Champion")
	e := f.e
	goblin, champ := f.extra[0], f.extra[1]
	if !e.protectedFrom(goblin, f.copyID) || e.protectedFrom(champ, f.copyID) {
		t.Fatalf("precondition: the blue spell should be blocked by the Goblin only")
	}
	if !e.protectedFrom(champ, f.fist) || e.protectedFrom(goblin, f.fist) {
		t.Fatalf("precondition: the red Fist should be blocked by the Champion only")
	}
	idx := copyAuraCloneOpt(f.ask, f.fist)
	if idx < 0 {
		t.Fatalf("clone election does not offer Inferno Fist: %+v", f.ask.Options)
	}
	submitChoices(t, e, idx)
	d := passUntilAsk(t, e)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("no enchant ask after the clone answer: %+v", d)
	}
	var lion, gob bool
	for _, o := range d.Options {
		if o.Kind != "enchant" || o.Obj == champ {
			t.Fatalf("enchant options must be the Lion and the Goblin only: %+v", d.Options)
		}
		lion = lion || o.Obj == f.lion
		gob = gob || o.Obj == goblin
	}
	if !lion || !gob || len(d.Options) != 2 {
		t.Fatalf("enchant options must offer exactly Lion and Goblin: %+v", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == goblin {
			submitChoices(t, e, o.Index)
		}
	}
	passUntilStackEmpty(t, e, 60)
	co := e.G.Obj(f.copyID)
	if co == nil || co.Zone != state.ZBattlefield || co.AttachedTo != goblin {
		t.Fatalf("copy should sit on the battlefield attached to the Goblin: %+v", co)
	}
	replayCheck(t, e, f.cfg)
}
