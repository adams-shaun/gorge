package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The Class enchantment type (CR 702.118), pinned end to end on the REAL
// corpus cards. Before the kw:Class expansion every one of these cards'
// level-up activators and level-granted abilities was inert: the K:Class line
// expanded to nothing, so a Class never gained a level and its level bands
// never turned on. The helpers come from search_library_test.go
// (searchTestRegistry, searchCorpusCard, searchMoveByName), cast_test.go
// (submitChoices, toMain1, addMana) and replacement_updated_test.go
// (passUntilStackEmpty). No Forge script text is committed (the decks are
// compiled corpus cards) and no Class card is in any repo deck or legacy
// golden deck, so no chain head depends on these cards.

// classEngine deals seat 0 a 40-card deck opening with the named fixtures
// (the Class under test first, then any support cards), followed by eight
// Forests and Grizzly Bears; the opponent's deck is all Mountains. seat 0 is
// pinned to the starting seat so its Main1 is the first turn driven to.
func classEngine(t *testing.T, reg *cards.Registry, fixtures ...string) (*Engine, Config) {
	t.Helper()
	forest := searchCorpusCard(t, reg, "Forest")
	mountain := searchCorpusCard(t, reg, "Mountain")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	deck := make([]*cards.Card, 0, 40)
	for _, name := range fixtures {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	for i := 0; i < 8; i++ {
		deck = append(deck, forest)
	}
	for len(deck) < 40 {
		deck = append(deck, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 702118,
		Names: []string{"classPlayer", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// classMove is searchMoveByName for a named card, but it also accepts a
// card already on the battlefield (returns it) so a fixture can be moved and
// then acted on without depending on draw order.
func classMove(t *testing.T, e *Engine, name string, to state.Zone) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZBattlefield, state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				if z != to {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
				}
				e.pending = nil
				e.priorityRound()
				return id
			}
		}
	}
	t.Fatalf("corpus fixture %q absent from seat 0 zones", name)
	return 0
}

// TestPaladinClassEntersAtLevel1LevelsUpAndTurnsOnItsStatic is the brief's
// done shape on Paladin Class: the Class enters at level 1, its level-2
// activator is offered as a sorcery for {2}{W}, and once the level is gained
// the level-2 SPump static ("Creatures you control get +1/+1") is actually
// applied to a creature on the battlefield.
func TestPaladinClassEntersAtLevel1LevelsUpAndTurnsOnItsStatic(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := classEngine(t, reg, "Paladin Class", "Grizzly Bears")

	class := classMove(t, e, "Paladin Class", state.ZBattlefield)
	bear := classMove(t, e, "Grizzly Bears", state.ZBattlefield)

	// CR 702.118a: a Class enters at level 1.
	if got := e.G.Obj(class).Counter("LEVEL"); got != 1 {
		t.Fatalf("precondition: Class entered with LEVEL=%d, want 1", got)
	}
	// Precondition: the lord is NOT live at level 1, and the bear is a real
	// 2/2, so the post-level assertion below is not vacuous.
	if got := e.Power(bear); got != 2 {
		t.Fatalf("precondition: bear power %d at level 1, want 2", got)
	}
	if got := e.Toughness(bear); got != 2 {
		t.Fatalf("precondition: bear toughness %d at level 1, want 2", got)
	}

	// The level-2 activator is offered, as a sorcery, for the K:Class cost.
	addMana(t, e, 0, "WWW")                   // {2}{W}
	opt, ok := findAbilityOption(e, class, 0) // the level-2 activator (first K:Class line)
	if !ok {
		t.Fatalf("level-2 activator not offered: %+v", e.Pending().Options)
	}
	if opt.Label == "" {
		t.Fatal("activator option has no label")
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)

	// CR 702.118b: the level counter is on, the next level's ability is live.
	if got := e.G.Obj(class).Counter("LEVEL"); got != 2 {
		t.Fatalf("after activating level 2: LEVEL=%d, want 2", got)
	}
	if got := e.Power(bear); got != 3 {
		t.Fatalf("level-2 SPump not applied: bear power %d, want 3", got)
	}
	if got := e.Toughness(bear); got != 3 {
		t.Fatalf("level-2 SPump not applied: bear toughness %d, want 3", got)
	}

	// The level-2 activator is withdrawn once the level is reached (a Class
	// at level 2 is no longer below level 2), while the level-3 activator --
	// below level 3 -- still exists.
	e.Advance()
	if _, ok := findAbilityOption(e, class, 0); ok {
		t.Fatalf("level-2 activator still offered at level 2: %+v", e.Pending().Options)
	}
	if !classHasAbility(e, class, 1) {
		t.Fatal("level-3 activator is not a real ability on the face")
	}
}

// TestPaladinClassLevelUpIsSorcerySpeed pins that the activator is withheld
// outside a sorcery window -- the CR 702.118b "only as a sorcery" clause the
// expansion carries as SorcerySpeed$ True.
func TestPaladinClassLevelUpIsSorcerySpeed(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := classEngine(t, reg, "Paladin Class")
	class := classMove(t, e, "Paladin Class", state.ZBattlefield)

	// Positive control: in seat 0's own Main1 the activator IS offered, so
	// the negative below is about the timing window, not a dead activator.
	addMana(t, e, 0, "WWW")
	if _, ok := findAbilityOption(e, class, 0); !ok {
		t.Fatalf("level-up not offered in a sorcery window: %+v", e.Pending().Options)
	}

	// Drive to seat 0's own BeginCombat: seat 0 holds priority there, but it
	// is not a sorcery window, so the activator must be withheld. Mana is
	// deliberately NOT added here -- addMana would drive back to Main1; the
	// SorcerySpeed$ gate runs before the cost gate, so none is needed.
	driveToStep(t, e, e.G.Turn, 0, state.StepBeginCombat)
	e.Advance()
	d := e.Pending()
	if d == nil {
		t.Fatal("no priority decision at BeginCombat")
	}
	if _, ok := findAbilityOption(e, class, 0); ok {
		t.Fatalf("level-up offered outside a sorcery window: %+v", d.Options)
	}
}

// TestCaretakersTalentLevelGainedTriggerFiresOnTheLevelUp pins the
// Mode$ ClassLevelGained trigger (CR 702.118c) on Caretaker's Talent's real
// script: its level-2 grant is a trigger ("When this Class becomes level 2,
// create a token that's a copy of target token you control"), which fires
// from the same CounterChange event the level-up activator emits.
func TestCaretakersTalentLevelGainedTriggerFiresOnTheLevelUp(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := classEngine(t, reg, "Caretaker's Talent")
	class := classMove(t, e, "Caretaker's Talent", state.ZBattlefield)
	if got := e.G.Obj(class).Counter("LEVEL"); got != 1 {
		t.Fatalf("precondition: Class at level %d, want 1", got)
	}
	// Precondition: the level-2 ability here is a TRIGGER, not a static, so
	// the post-assertion below is specifically about the ClassLevelGained
	// matcher.
	if !classHasTriggerMode(e.G.Obj(class).Face(), "ClassLevelGained") {
		t.Fatal("precondition: Caretaker's Talent carries no ClassLevelGained trigger")
	}

	addMana(t, e, 0, "W")
	opt, ok := findAbilityOption(e, class, 0)
	if !ok {
		t.Fatalf("level-2 activator not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Obj(class).Counter("LEVEL"); got != 2 {
		t.Fatalf("LEVEL=%d after level-up, want 2", got)
	}
	// The ClassLevelGained trigger fired: its push names the Class as source.
	fired := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == class {
			fired = true
			break
		}
	}
	if !fired {
		t.Fatal("no TriggerPush for the Class after gaining level 2: ClassLevelGained did not match")
	}
}

// classHasAbility reports whether the live object offers ability index idx.
func classHasAbility(e *Engine, id state.ObjID, idx int) bool {
	o := e.G.Obj(id)
	if o == nil {
		return false
	}
	_, ok := o.PileAbilityAt(idx)
	return ok
}

// classHasTriggerMode reports whether the face carries a trigger with the
// named mode -- used as a precondition so a trigger test that silently lost
// its trigger fails loudly instead of passing.
func classHasTriggerMode(f *cards.Face, mode string) bool {
	if f == nil {
		return false
	}
	for _, tr := range f.Triggers {
		if tr.Mode == mode {
			return true
		}
	}
	return false
}

// TestFortuneTellersTalentLevelTwoGrantsPlayFromTop pins the brief's own
// carrier end to end: Fortune Teller's Talent's level-2 grant is the
// Mode$ Continuous MayPlay$ True static over Affected$ Card.TopLibrary+YouCtrl
// gated on CheckSVar$ X ("as long as you've cast a spell this turn, you may
// play cards from the top of your library"). The level gate the kw:Class
// expansion adds (counters_GE2_LEVEL) AND the static's own CheckSVar$ both
// have to hold before the top card is playable, and the top card must then
// actually be offered.
func TestFortuneTellersTalentLevelTwoGrantsPlayFromTop(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := classEngine(t, reg, "Fortune Teller's Talent", "Grizzly Bears", "Forest", "Forest")
	class := classMove(t, e, "Fortune Teller's Talent", state.ZBattlefield)
	if got := e.G.Obj(class).Counter("LEVEL"); got != 1 {
		t.Fatalf("precondition: Class at level %d, want 1", got)
	}

	addMana(t, e, 0, "UUUU") // {3}{U}
	opt, ok := findAbilityOption(e, class, 0)
	if !ok {
		t.Fatalf("level-2 activator not offered: %+v", e.Pending().Options)
	}
	if !strings.Contains(opt.Label, "Level 2") {
		t.Fatalf("activator label %q does not name level 2", opt.Label)
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(class).Counter("LEVEL"); got != 2 {
		t.Fatalf("LEVEL=%d after level-up, want 2", got)
	}

	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) == 0 {
		t.Fatal("precondition: empty library")
	}
	top := lib[0]
	if e.G.Obj(top).Face() == nil || e.G.Obj(top).Face().IsLand() {
		t.Fatalf("precondition: top card %v is not a spell", e.G.Obj(top).Face())
	}
	// The static's own CheckSVar$ X ("you've cast a spell this turn") is
	// false, so the Class's level gate alone must not grant the play.
	if _, grant := e.mayPlayGrant(0, top); grant {
		t.Fatal("grant live before any spell this turn: the CheckSVar$ X gate was not read")
	}

	// Cast a spell this turn, then the top card becomes playable.
	addMana(t, e, 0, "GG")
	castOpt := castByName(t, e, 0, "Grizzly Bears")
	if castOpt == nil {
		t.Fatalf("no Grizzly Bears cast option: %+v", e.Pending().Options)
	}
	submitChoices(t, e, castOpt.Index)
	passUntilStackEmpty(t, e, 20)

	// The hand cast spent the pool; fund the top-of-library cast before
	// reading the offer, or the affordability gate (not the grant) is what
	// withholds it.
	addMana(t, e, 0, "GG")

	if free, grant := e.mayPlayGrant(0, top); !grant || free {
		t.Fatalf("top card grant after a spell this turn: free=%v grant=%v, want a non-free grant", free, grant)
	}
	// End to end: the top card is actually offered as a cast.
	d := e.Pending()
	if d == nil {
		t.Fatal("no priority decision after resolving the cast")
	}
	offered := false
	for _, o := range d.Options {
		if o.Obj == top && (o.Kind == "cast" || o.Kind == "mayplay") {
			offered = true
			break
		}
	}
	if !offered {
		t.Fatalf("top card %d not offered as a play: %+v", top, d.Options)
	}
}
