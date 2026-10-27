package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An untyped MayPlayLimit$ static is ONE permission: its limit counts the
// plays made through the static, not the plays of each affected card
// (cr-g215; Forge's StaticAbility.getMayPlayTurn and XMage's
// OnceEachTurnCastWatcher both count per static).

const (
	perStaticGraveyardGrant = "S:Mode$ Continuous | Affected$ Instant.YouOwn | AffectedZone$ Graveyard | MayPlay$ True | MayPlayLimit$ 1\n"
	// johannStatic is Johann, Apprentice Sorcerer's real may-play static.
	johannStatic = "S:Mode$ Continuous | MayPlay$ True | MayPlayLimit$ 1 | Affected$ Card.Instant+TopLibrary+YouCtrl+nonLand,Card.Sorcery+TopLibrary+YouCtrl+nonLand | AffectedZone$ Library | Description$ Once each turn, you may cast an instant or sorcery spell from the top of your library.\n"
	// serraStatic is Serra Paragon's real may-play static.
	serraStatic = "S:Mode$ Continuous | Affected$ Permanent.YouOwn+cmcLE3,Land.YouOwn | ValidAfterStack$ Spell.cmcLE3 | Condition$ PlayerTurn | MayPlay$ True | MayPlayLimit$ 1 | EffectZone$ Battlefield | AffectedZone$ Graveyard | Description$ Once during each of your turns, you may play a land from your graveyard or cast a permanent spell with mana value 3 or less from your graveyard.\n"
)

func perStaticEnchant(name string) string {
	return "Name:" + name + "\nManaCost:1\nTypes:Enchantment\n" + perStaticGraveyardGrant + "Oracle:x\n"
}

func perStaticZap(name string) string {
	return "Name:" + name + "\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"
}

// perStaticOffer reports whether seat 0's priority options hold an option of
// kind for id.
func perStaticOffer(e *Engine, kind string, id state.ObjID) bool {
	for _, o := range e.legalActions(0) {
		if o.Kind == kind && o.Obj == id {
			return true
		}
	}
	return false
}

// perStaticCast casts id through its (single) may-play option, aims it at
// seat 1 and resolves it.
func perStaticCast(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision to cast %d: %+v", id, d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "mayplay" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("may-play cast of %d not offered: %+v", id, d.Options)
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		tIdx := -1
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player == 1 {
				tIdx = o.Index
			}
		}
		if tIdx < 0 {
			t.Fatalf("seat 1 not offered as a target: %+v", d.Options)
		}
		submitChoices(t, e, tIdx)
	}
	passUntilStackEmpty(t, e, 40)
	if !slices.ContainsFunc(e.L.Events, func(ev events.Event) bool { return ev.Kind == events.PutOnStack && ev.Obj == id }) {
		t.Fatalf("card %d never went on the stack", id)
	}
	e.priorityRound()
}

// perStaticNextTurn drives to seat 0's next turn and refunds red mana.
func perStaticNextTurn(t *testing.T, e *Engine, mana string) {
	t.Helper()
	driveToStep(t, e, e.G.Turn+2, 0, state.StepMain1)
	addMana(t, e, 0, mana)
}

// perStaticTokenCount counts the pay-time CastInfo events and LandPlayed
// events that carry a limited untyped permission token.
func perStaticTokenCount(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if (ev.Kind == events.CastInfo && strings.Contains(ev.Counter, "perm=")) ||
			(ev.Kind == events.LandPlayed && strings.Contains(ev.Text, "perm=")) {
			n++
		}
	}
	return n
}

func TestMayPlayLimitIsPerStaticNotPerCard(t *testing.T) {
	t.Parallel()
	e, cfg, _ := newFixtureDeck(t, 205, perStaticEnchant("Gravebinder"),
		perStaticZap("ZapA"), perStaticZap("ZapB"))
	moveSeeded(t, e, 0, perStaticEnchant("Gravebinder"), state.ZBattlefield)
	a := moveSeeded(t, e, 0, perStaticZap("ZapA"), state.ZGraveyard)
	b := moveSeeded(t, e, 0, perStaticZap("ZapB"), state.ZGraveyard)
	addMana(t, e, 0, "RR")

	// Precondition: the static covers both cards before any cast, so the
	// only reason ZapB can vanish is the limit.
	if !perStaticOffer(e, "cast", a) || !perStaticOffer(e, "cast", b) {
		t.Fatalf("precondition: both instants must be offered: %+v", e.legalActions(0))
	}
	perStaticCast(t, e, a)
	if perStaticTokenCount(e) != 1 {
		t.Fatalf("the cast must stamp the static's permission token once, got %d", perStaticTokenCount(e))
	}
	if perStaticOffer(e, "cast", a) || perStaticOffer(e, "cast", b) {
		t.Fatalf("the static's one use is spent; neither instant may be offered: %+v", e.legalActions(0))
	}

	perStaticNextTurn(t, e, "RR")
	if !perStaticOffer(e, "cast", b) {
		t.Fatalf("next turn the static is available again: %+v", e.legalActions(0))
	}
	replayCheck(t, e, cfg)
}

func TestJohannOnceEachTurnFromLibraryTop(t *testing.T) {
	t.Parallel()
	johann := "Name:Johann Fixture\nManaCost:1 U R\nTypes:Legendary Creature Human Wizard\nPT:2/2\n" + johannStatic + "Oracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 206, johann,
		perStaticZap("ZapA"), perStaticZap("ZapB"), perStaticZap("ZapC"))
	moveSeeded(t, e, 0, johann, state.ZBattlefield)
	var ids []state.ObjID
	for _, n := range []string{"ZapA", "ZapB", "ZapC"} {
		for _, id := range e.G.Zone(state.ZLibrary, 0) {
			if e.G.Obj(id).Face().Name == n {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) != 3 {
		// A fixture instant that started in hand is not on the library.
		for _, id := range e.G.Zone(state.ZHand, 0) {
			if n := e.G.Obj(id).Face().Name; strings.HasPrefix(n, "Zap") {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
			}
		}
		ids = ids[:0]
		for _, n := range []string{"ZapA", "ZapB", "ZapC"} {
			for _, id := range e.G.Zone(state.ZLibrary, 0) {
				if e.G.Obj(id).Face().Name == n {
					ids = append(ids, id)
				}
			}
		}
	}
	if len(ids) != 3 {
		t.Fatalf("precondition: the three instants must be in the library, got %v", ids)
	}
	order := append([]state.ObjID(nil), ids...)
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if id != ids[0] && id != ids[1] && id != ids[2] {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order})
	addMana(t, e, 0, "RR")

	if top := e.G.Zone(state.ZLibrary, 0)[0]; top != ids[0] || !perStaticOffer(e, "cast", ids[0]) {
		t.Fatalf("precondition: ZapA must be the offered library top: top=%d %+v", top, e.legalActions(0))
	}
	perStaticCast(t, e, ids[0])
	if top := e.G.Zone(state.ZLibrary, 0)[0]; top != ids[1] {
		t.Fatalf("precondition: ZapB must be the new library top, got %d", top)
	}
	if perStaticOffer(e, "cast", ids[1]) {
		t.Fatalf("Johann's once-each-turn cast is spent; the next top card must not be offered: %+v", e.legalActions(0))
	}

	// Next turn the draw takes ZapB and ZapC is the top card.
	perStaticNextTurn(t, e, "RR")
	if top := e.G.Zone(state.ZLibrary, 0)[0]; top != ids[2] || !perStaticOffer(e, "cast", ids[2]) {
		t.Fatalf("next turn the new top card must be offered again: top=%d %+v", top, e.legalActions(0))
	}
	replayCheck(t, e, cfg)
}

func TestSerraParagonLandThenSpellSharesTheLimit(t *testing.T) {
	t.Parallel()
	serra := "Name:Paragon Fixture\nManaCost:2 W W\nTypes:Creature Angel\nPT:3/4\nK:Flying\n" + serraStatic + "Oracle:x\n"
	land := "Name:Gravewalk\nTypes:Land\nOracle:x\n"
	bear := "Name:Cheap Bear\nManaCost:1 R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 207, serra, land, bear)
	moveSeeded(t, e, 0, serra, state.ZBattlefield)
	l := moveSeeded(t, e, 0, land, state.ZGraveyard)
	b := moveSeeded(t, e, 0, bear, state.ZGraveyard)
	addMana(t, e, 0, "RR")

	if !perStaticOffer(e, "play_land", l) || !perStaticOffer(e, "cast", b) {
		t.Fatalf("precondition: the land play and the cheap permanent must both be offered: %+v", e.legalActions(0))
	}
	e.priorityRound()
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == l {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("land play not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	if o := e.G.Obj(l); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the land must have been played, zone %s", o.Zone)
	}
	if perStaticTokenCount(e) != 1 {
		t.Fatalf("the land play must stamp the static's permission token, got %d", perStaticTokenCount(e))
	}
	e.priorityRound()
	if perStaticOffer(e, "cast", b) {
		t.Fatalf("the land spent Serra Paragon's one use; the permanent must not be offered: %+v", e.legalActions(0))
	}

	perStaticNextTurn(t, e, "RR")
	if !perStaticOffer(e, "cast", b) {
		t.Fatalf("next turn the permission is available again: %+v", e.legalActions(0))
	}
	replayCheck(t, e, cfg)
}

func TestTwoLimitedStaticsCountSeparately(t *testing.T) {
	t.Parallel()
	e, cfg, _ := newFixtureDeck(t, 208, perStaticEnchant("Gravebinder One"),
		perStaticEnchant("Gravebinder Two"), perStaticZap("ZapA"), perStaticZap("ZapB"), perStaticZap("ZapC"))
	moveSeeded(t, e, 0, perStaticEnchant("Gravebinder One"), state.ZBattlefield)
	moveSeeded(t, e, 0, perStaticEnchant("Gravebinder Two"), state.ZBattlefield)
	a := moveSeeded(t, e, 0, perStaticZap("ZapA"), state.ZGraveyard)
	b := moveSeeded(t, e, 0, perStaticZap("ZapB"), state.ZGraveyard)
	c := moveSeeded(t, e, 0, perStaticZap("ZapC"), state.ZGraveyard)
	addMana(t, e, 0, "RRR")

	// Each instant is offered once per still-unused static.
	n := 0
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == a {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("precondition: ZapA must be offered through both statics, got %d: %+v", n, e.legalActions(0))
	}
	perStaticCast(t, e, a)
	// One static is spent, the other is not: the second cast is offered but
	// through a single remaining permission.
	n = 0
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == b {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("one static must remain for ZapB, got %d offers: %+v", n, e.legalActions(0))
	}
	perStaticCast(t, e, b)
	if perStaticOffer(e, "cast", c) {
		t.Fatalf("both statics are spent; a third cast must not be offered: %+v", e.legalActions(0))
	}
	if perStaticTokenCount(e) != 2 {
		t.Fatalf("two casts must stamp two tokens, got %d", perStaticTokenCount(e))
	}
	replayCheck(t, e, cfg)
}
