package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSourceHasQualityPlayerUIDForms pins Forge's two player-valued
// protection qualities: Player.PlayerUID_<n> is a source controlled by seat
// n, and Player.OpponentOf PlayerUID_<n> is a source controlled by anyone
// else. An out-of-range seat names no source, so both forms are false rather
// than an accidental match on a zero-value seat.
func TestSourceHasQualityPlayerUIDForms(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	o := e.G.AddObject(card(t, "Name:Source\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	o.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{o.ID})
	if got := e.controllerOf(o.ID); got != 1 {
		t.Fatalf("precondition: source controller = %d, want seat 1", got)
	}
	cases := []struct {
		quality string
		want    bool
	}{
		{"Player.PlayerUID_1", true},
		{"Player.PlayerUID_0", false},
		{"Player.OpponentOf PlayerUID_0", true},
		{"Player.OpponentOf PlayerUID_1", false},
		{"Player.PlayerUID_2", false},
		{"Player.OpponentOf PlayerUID_2", false},
	}
	for _, tc := range cases {
		if got := e.sourceHasQuality(o.ID, tc.quality); got != tc.want {
			t.Errorf("sourceHasQuality(%q) = %v, want %v", tc.quality, got, tc.want)
		}
	}
}

// TestCourageousResolveProtectionFromEachOpponent is the corpus carrier end to
// end: the spell's DefinedKW$ Opponent expands the KW$ protection token once
// per opponent, and the resulting quality actually stops an opponent's
// damage, blocking, and targeted spell while leaving the caster's own spell
// free to target the creature.
func TestCourageousResolveProtectionFromEachOpponent(t *testing.T) {
	t.Parallel()
	const (
		bearSrc  = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		boltSrc  = "Name:Prot Bolt\nManaCost:0\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2\nOracle:x\n"
		sweepSrc = "Name:Sweeper\nManaCost:0\nTypes:Creature Goblin\nPT:5/5\nA:AB$ DamageAll | Cost$ 0 | ValidCards$ Creature | NumDmg$ 2\nOracle:x\n"
	)
	reg := testutil.CorpusRegistry(t)
	courageous := lookup(t, reg, "Courageous Resolve")
	bearCard, boltCard, sweepCard := card(t, bearSrc), card(t, boltSrc), card(t, sweepSrc)
	build := func(s uint64) Config {
		return Config{Seed: s, Names: []string{"a", "b", "c"},
			Decks: [][]*cards.Card{
				append([]*cards.Card{courageous, bearCard, boltCard}, mountainDeck(t, 37)...),
				append([]*cards.Card{sweepCard, boltCard}, mountainDeck(t, 38)...),
				mountainDeck(t, 40),
			},
			Tokens: map[string]*cards.Card{},
		}
	}
	cfg := seatZeroStart(build(7))
	e := New(cfg)
	e.Advance()

	bear := putCreature(t, e, 0, bearSrc)
	sweeper := putCreature(t, e, 1, sweepSrc)
	courageousID := findAndMoveToHand(t, e, 0, "Courageous Resolve")

	// A real attack declaration makes the bear a legal attacker for the
	// blocking half; it is also replayable, unlike a direct IsAttacking write.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.pending = nil
	if !e.canBlock(sweeper, bear) {
		t.Fatal("precondition: the sweeper can block the bear before protection is granted")
	}

	addMana(t, e, 0, "WWW")
	castFirstOf(t, e, courageousID)
	answerKTarget(t, e, bear)
	passUntilStackEmpty(t, e, 50)

	for _, want := range []string{"Protection:Player.PlayerUID_1:b", "Protection:Player.PlayerUID_2:c"} {
		if !slices.Contains(e.Keywords(bear), want) {
			t.Fatalf("bear keywords = %v, want %q", e.Keywords(bear), want)
		}
	}
	if e.canBlock(sweeper, bear) {
		t.Error("the sweeper can still block the protected bear")
	}

	// The opponent's untargeted sweep still resolves, but the protected bear
	// takes nothing while the sweeper itself proves the sweep ran.
	e.askPriority(1)
	submitChoices(t, e, abilityOption(t, e, sweeper, 0).Index)
	passUntilStackEmpty(t, e, 50)
	if got := e.G.Obj(sweeper).Damage; got != 2 {
		t.Fatalf("precondition: sweeper damage = %d, want 2 (the sweep must have resolved)", got)
	}
	if got := e.G.Obj(bear).Damage; got != 0 {
		t.Fatalf("protected bear took %d damage, want 0", got)
	}
	prevented := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Obj == bear && ev.Text == "prevented: protection" {
			prevented = true
		}
	}
	if !prevented {
		t.Fatal("no prevented: protection Note was recorded for the protected bear")
	}

	// An opponent's targeted spell is not offered the protected creature; the
	// caster's own copy of the same spell still is.
	oppBolt := findAndMoveToHand(t, e, 1, "Prot Bolt")
	e.askPriority(1)
	castFirstOf(t, e, oppBolt)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("opponent's bolt did not ask for a target: %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == bear {
			t.Fatalf("opponent's bolt was offered the protected bear: %+v", d.Options)
		}
	}
	answerKTarget(t, e, sweeper)
	passUntilStackEmpty(t, e, 50)

	ownBolt := findAndMoveToHand(t, e, 0, "Prot Bolt")
	e.askPriority(0)
	castFirstOf(t, e, ownBolt)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("own bolt did not ask for a target: %+v", d)
	}
	found := false
	for _, o := range d.Options {
		if o.Obj == bear {
			found = true
		}
	}
	if !found {
		t.Fatalf("own bolt was not offered the protected bear: %+v", d.Options)
	}
	answerKTarget(t, e, bear)
	passUntilStackEmpty(t, e, 50)

	replayCheck(t, e, cfg)
}
