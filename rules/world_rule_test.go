// Ticket agent-20260923T065617Z-9b7a6efa: the CR 704.5k world rule. If two or
// more permanents carrying the World supertype are controlled by one player,
// that player chooses one and the rest go to their owners' graveyards -- the
// legend rule's (CR 704.5j) twin. The build already implements the legend
// rule with a controller choice through one parked-batch channel
// (legendBatch); the world rule reuses that channel, the batch carrying which
// rule parked it, so the ask prompt, the Choose counter ("legend_keep" /
// "world_keep") and the departure Text ("legend rule" / "world rule") all
// derive from the batch's rule. Legend batches emit byte-identical events to
// the pre-generalization build, which the existing TestLegend* suite pins.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// worldEnchantment is a plain World enchantment source. World is a supertype
// the layer system can change, read from the derived type list exactly as the
// legend rule reads Legendary.
func worldEnchantment(name string) string {
	return "Name:" + name + "\nManaCost:2 B\nTypes:World Enchantment\nOracle:x\n"
}

// worldPairGame fields TWO identically-named World enchantments for seat 0
// through newFixtureDeck + moveSeeded (a World card is no one's commander) and
// returns the engine and both battlefield ids in battlefield order. The
// precondition the world rule reads -- both on the battlefield, both carrying
// the World supertype, both the same name -- is asserted here, so a vacuous
// setup fails loudly instead of passing silently.
func worldPairGame(t *testing.T, name string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	src := worldEnchantment(name)
	// Two copies: the fixture (bridged to hand by newFixtureDeck) and one
	// extra, both in seat 0's deck.
	e, _, _ := newFixtureDeck(t, 93, src, src)
	id1 := moveSeeded(t, e, 0, src, state.ZBattlefield)
	id2 := moveSeeded(t, e, 0, src, state.ZBattlefield)
	for _, id := range []state.ObjID{id1, id2} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("fixture: world permanent %d not on the battlefield (zone %v)", id, o)
		}
		if !o.Face().IsWorld() {
			t.Fatalf("fixture: %s is not a World permanent; CR 704.5k cannot fire", o.Face().Name)
		}
		if o.Face().Name != name {
			t.Fatalf("fixture: name %q, want %q", o.Face().Name, name)
		}
	}
	if id1 == id2 {
		t.Fatalf("fixture: the two permanents share one id; nothing is duplicated")
	}
	return e, id1, id2
}

// worldPending asserts the engine is holding exactly the CR 704.5k choice for
// the given controller over the given duplicate set, offered in battlefield
// order, and returns the pending decision. It fails when the engine asks
// nothing (the pre-fix behaviour: the duplicate world permanents sat on the
// battlefield forever).
func worldPending(t *testing.T, e *Engine, p state.PlayerID, ids ...state.ObjID) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatalf("no decision pending: the world rule did not ask its controller")
	}
	if d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("pending decision %s Min %d Max %d, want a KChoose Min 1 Max 1", d.Kind, d.Min, d.Max)
	}
	if d.Player != p {
		t.Fatalf("world choice asked seat %d, want the duplicates' controller seat %d", d.Player, p)
	}
	if len(d.Options) != len(ids) {
		t.Fatalf("world choice offers %d options, want one per duplicate (%d)", len(d.Options), len(ids))
	}
	for i, o := range d.Options {
		if o.Kind != "keep" {
			t.Fatalf("world option %d Kind %q, want \"keep\"", i, o.Kind)
		}
		if o.Obj != ids[i] {
			t.Fatalf("world option %d names obj %d, want %d (battlefield order)", i, o.Obj, ids[i])
		}
	}
	return d
}

// worldChooseMarkers returns the counters of every Choose event naming obj.
func worldChooseMarkers(e *Engine, obj state.ObjID) []string {
	var out []string
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Obj == obj {
			out = append(out, ev.Counter)
		}
	}
	return out
}

// TestWorldRuleAsksControllerWhichDuplicateToKeep is the headline: two
// same-named World permanents under one controller pose a choice, and keeping
// the SECOND leaves it on the battlefield while the FIRST departs as a
// world-rule placement with a "world_keep" Choose marker. It fails with the
// fix reverted: no decision is pending and both stay on the battlefield.
func TestWorldRuleAsksControllerWhichDuplicateToKeep(t *testing.T) {
	e, id1, id2 := worldPairGame(t, "Nether Void")
	e.checkStateBased()
	d := worldPending(t, e, 0, id1, id2)
	if e.legendBatch == nil || e.legendBatch.rule != sbaWorld {
		t.Fatalf("the batch is not parked for the world rule")
	}
	if e.legendBatch.group.ids[0] != id1 {
		t.Fatalf("the parked world set is not in battlefield order")
	}
	submitKeep(t, e, d, 1)
	if o := e.G.Obj(id2); o.Zone != state.ZBattlefield {
		t.Fatalf("kept world permanent left the battlefield (zone %v); the controller's choice did not hold", o.Zone)
	}
	if o := e.G.Obj(id1); o.Zone != state.ZGraveyard {
		t.Fatalf("unchosen world permanent stayed in %v; want its owner's graveyard", o.Zone)
	}
	if got := legendDepartureText(t, e, id1); got != "world rule" {
		t.Fatalf("unchosen world permanent departed with Text %q, want \"world rule\" (placement, not destruction)", got)
	}
	if markers := worldChooseMarkers(e, id2); len(markers) != 1 || markers[0] != "world_keep" {
		t.Fatalf("Choose markers for kept permanent %d = %v, want exactly [\"world_keep\"]", id2, markers)
	}
	if e.legendBatch != nil {
		t.Fatalf("world batch still parked after the answer")
	}
}

// TestWorldRuleBotAnswerKeepsBattlefieldOrderFirst runs the deterministic
// bot's own answer through the validator on a board where the choice binds:
// botpolicy has no scored arm for a "keep" ballot, so its clamp fallback
// takes option 0 -- the battlefield-order first member. The intent must
// validate (no livelock: an illegal answer would re-submit forever) and the
// settle must hold.
func TestWorldRuleBotAnswerKeepsBattlefieldOrderFirst(t *testing.T) {
	e, id1, id2 := worldPairGame(t, "Nether Void")
	e.checkStateBased()
	d := worldPending(t, e, 0, id1, id2)
	bot := newTestBot(93)
	in := bot.answer(e, d)
	if err := d.Validate(in); err != nil {
		t.Fatalf("the bot's own answer does not validate: %v", err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("bot answer %v, want option 0 (the battlefield-order first member)", in.Choices)
	}
	if err := e.Submit(in); err != nil {
		t.Fatalf("Submit(bot answer): %v", err)
	}
	if o := e.G.Obj(id1); o.Zone != state.ZBattlefield {
		t.Fatalf("bot-kept first member in %v, want battlefield", o.Zone)
	}
	if o := e.G.Obj(id2); o.Zone != state.ZGraveyard {
		t.Fatalf("bot-kept board: second member in %v, want graveyard", o.Zone)
	}
	if got := legendDepartureText(t, e, id2); got != "world rule" {
		t.Fatalf("bot-binned member departed with Text %q, want \"world rule\"", got)
	}
}

// TestWorldRuleIsPerController pins CR 704.5k's per-controller scoping: the
// rule applies only when ONE player controls two or more same-named World
// permanents. Seat 0 controls two (which it is asked about, over exactly its
// own two permanents) while seat 1 controls one same-named World permanent
// that is never offered in seat 0's ask and never binned. The positive half
// (seat 0 IS asked) is asserted first, so the test cannot pass with the whole
// world rule unregistered.
func TestWorldRuleIsPerController(t *testing.T) {
	src := worldEnchantment("Nether Void")
	// Seat 0's deck carries two copies, seat 1's one; moveSeeded fields a real
	// deck card through a logged MoveZone, so replay reconstructs the board.
	e, _, _ := newFixtureDeckWithOpponentCard(t, 97, src, src, src)
	s0a := moveSeeded(t, e, 0, src, state.ZBattlefield)
	s0b := moveSeeded(t, e, 0, src, state.ZBattlefield)
	s1 := moveSeeded(t, e, 1, src, state.ZBattlefield)
	// Precondition: all three are battlefield World permanents, seat 0's two
	// share a controller, and seat 1's is a different controller -- otherwise
	// the scoping assertion is vacuous.
	for _, id := range []state.ObjID{s0a, s0b, s1} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield || !o.Face().IsWorld() {
			t.Fatalf("fixture: permanent %d not a battlefield World permanent (zone %v)", id, o.Zone)
		}
	}
	if c := e.G.Obj(s0a).Controller; c != e.G.Obj(s0b).Controller {
		t.Fatalf("fixture: seat 0's two permanents have different controllers %d/%d", c, e.G.Obj(s0b).Controller)
	}
	if e.G.Obj(s0a).Controller == e.G.Obj(s1).Controller {
		t.Fatalf("fixture: seat 1's permanent shares seat 0's controller; the per-controller assertion is vacuous")
	}
	e.checkStateBased()
	// The positive half: seat 0 IS asked, over exactly its own two permanents.
	d := worldPending(t, e, 0, s0a, s0b)
	for _, o := range d.Options {
		if o.Obj == s1 {
			t.Fatalf("seat 0's world ask offered seat 1's permanent %d; the rule is not per-controller", s1)
		}
	}
	submitKeep(t, e, d, 0)
	// The negative half: seat 1's lone same-named World permanent is untouched.
	if o := e.G.Obj(s1); o.Zone != state.ZBattlefield {
		t.Fatalf("per-controller scoping binned seat 1's permanent (zone %v)", o.Zone)
	}
	if o := e.G.Obj(s0b); o.Zone != state.ZGraveyard {
		t.Fatalf("seat 0's unchosen duplicate in %v, want graveyard", o.Zone)
	}
}

// TestWorldRuleSinglePermanentPosesNothing asserts a lone World permanent is
// not a duplicate set: no ask, no batch, and it stays. The-positive control
// is in the same test: fielding a second same-named World permanent makes the
// ask arrive, so the test cannot pass with the world rule unregistered. The
// precondition (each permanent really is a battlefield World permanent) is
// asserted so a broken setup cannot pass silently.
func TestWorldRuleSinglePermanentPosesNothing(t *testing.T) {
	src := worldEnchantment("Concordant Crossroads")
	e, _, _ := newFixtureDeck(t, 94, src, src)
	id1 := moveSeeded(t, e, 0, src, state.ZBattlefield)
	o := e.G.Obj(id1)
	if o == nil || o.Zone != state.ZBattlefield || !o.Face().IsWorld() {
		t.Fatalf("fixture: the lone World permanent is not on the battlefield (zone %v)", o.Zone)
	}
	e.checkStateBased()
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("a lone World permanent posed a choice: %+v", d)
	}
	if e.legendBatch != nil {
		t.Fatalf("a lone World permanent parked a duplicate set")
	}
	if o := e.G.Obj(id1); o.Zone != state.ZBattlefield {
		t.Fatalf("a lone World permanent left the battlefield (zone %v)", o.Zone)
	}
	// Positive control: the second copy makes it a duplicate set, and the
	// world rule's handler runs (it asks). Without the handler this never
	// fires, so the absence assertions above are not vacuous.
	id2 := moveSeeded(t, e, 0, src, state.ZBattlefield)
	if o := e.G.Obj(id2); o.Zone != state.ZBattlefield || !o.Face().IsWorld() {
		t.Fatalf("fixture: the second World permanent is not on the battlefield (zone %v)", o.Zone)
	}
	e.checkStateBased()
	worldPending(t, e, 0, id1, id2)
}

// TestWorldRuleReadsDerivedSupertype pins that the world scan reads the
// DERIVED type list, not the printed face: two same-named non-World
// enchantments under one controller become a World duplicate set when a
// layer-4 static (CR 613.1c AddTypes$ World) grants the supertype. The
// precondition asserts the printed faces are NOT World while the derived
// lists ARE, so a scan that read the printed face alone cannot pass. It fails
// with the fix reverted: nothing is asked.
func TestWorldRuleReadsDerivedSupertype(t *testing.T) {
	const grant = "Name:Worldmaker\nManaCost:2 U\nTypes:Enchantment\n" +
		"S:Mode$ Continuous | Affected$ Enchantment.YouCtrl | AddTypes$ World | Description$ x\nOracle:x\n"
	plain := "Name:Gravity Sphere\nManaCost:2 R\nTypes:Enchantment\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 95, grant, plain, plain)
	moveSeeded(t, e, 0, grant, state.ZBattlefield)
	id1 := moveSeeded(t, e, 0, plain, state.ZBattlefield)
	id2 := moveSeeded(t, e, 0, plain, state.ZBattlefield)
	for _, id := range []state.ObjID{id1, id2} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("fixture: permanent %d not on the battlefield (zone %v)", id, o)
		}
		if o.Face().IsWorld() {
			t.Fatalf("fixture: printed face of %q already is World; the derived-read assertion would be vacuous", o.Face().Name)
		}
		derivedWorld := false
		for _, typ := range e.Derived(id).Types {
			if typ == "World" {
				derivedWorld = true
			}
		}
		if !derivedWorld {
			t.Fatalf("fixture: the layer-4 AddTypes$ World grant is not live in %d's derived types %v", id, e.Derived(id).Types)
		}
	}
	e.checkStateBased()
	d := worldPending(t, e, 0, id1, id2)
	submitKeep(t, e, d, 0)
	if o := e.G.Obj(id1); o.Zone != state.ZBattlefield {
		t.Fatalf("kept derived-World member in %v, want battlefield", o.Zone)
	}
	if o := e.G.Obj(id2); o.Zone != state.ZGraveyard {
		t.Fatalf("binned derived-World member in %v, want graveyard", o.Zone)
	}
	if got := legendDepartureText(t, e, id2); got != "world rule" {
		t.Fatalf("derived-World member departed with Text %q, want \"world rule\"", got)
	}
}

// TestWorldAndLegendRulesAreOrthogonal: one legendary pair and one world pair
// (different names, one controller) in one pass. The legend set is parked
// first; answering it lets the Submit tail's next SBA pass pose the world
// set. Each batch carries its own rule, so each emits its own counter and
// departure Text and neither is mistaken for the other. It fails with the fix
// reverted: the world ask never arrives.
func TestWorldAndLegendRulesAreOrthogonal(t *testing.T) {
	legend := "Name:Legend Twin\nManaCost:2 G\nTypes:Legendary Creature Bear\nPT:5/5\nOracle:x\n"
	world := worldEnchantment("The Abyss")
	e, _, _ := newFixtureDeck(t, 96, legend, legend, world, world)
	l1 := moveSeeded(t, e, 0, legend, state.ZBattlefield)
	l2 := moveSeeded(t, e, 0, legend, state.ZBattlefield)
	w1 := moveSeeded(t, e, 0, world, state.ZBattlefield)
	w2 := moveSeeded(t, e, 0, world, state.ZBattlefield)
	// Precondition: both pairs are actually duplicate sets on the battlefield.
	if o := e.G.Obj(l1); o.Zone != state.ZBattlefield || !o.Face().IsLegendary() {
		t.Fatalf("fixture: legendary pair member %d not a battlefield legend (zone %v)", l1, o.Zone)
	}
	if o := e.G.Obj(w1); o.Zone != state.ZBattlefield || !o.Face().IsWorld() {
		t.Fatalf("fixture: world pair member %d not a battlefield World permanent (zone %v)", w1, o.Zone)
	}
	e.checkStateBased()
	// The legend rule is asked first (same pass, before the world scan).
	dl := legendPending(t, e, 0, l1, l2)
	if e.legendBatch == nil || e.legendBatch.rule != sbaLegend {
		t.Fatalf("the first parked batch is not the legend batch")
	}
	submitKeep(t, e, dl, 0)
	// The legend answer's Submit tail re-scanned and posed the world set.
	dw := worldPending(t, e, 0, w1, w2)
	if e.legendBatch == nil || e.legendBatch.rule != sbaWorld {
		t.Fatalf("the second parked batch is not the world batch")
	}
	submitKeep(t, e, dw, 0)
	if o := e.G.Obj(l1); o.Zone != state.ZBattlefield {
		t.Fatalf("kept legend in %v, want battlefield", o.Zone)
	}
	if o := e.G.Obj(l2); o.Zone != state.ZGraveyard {
		t.Fatalf("binned legend in %v, want graveyard", o.Zone)
	}
	if got := legendDepartureText(t, e, l2); got != "legend rule" {
		t.Fatalf("binned legend departed with Text %q, want \"legend rule\"", got)
	}
	if o := e.G.Obj(w1); o.Zone != state.ZBattlefield {
		t.Fatalf("kept world permanent in %v, want battlefield", o.Zone)
	}
	if o := e.G.Obj(w2); o.Zone != state.ZGraveyard {
		t.Fatalf("binned world permanent in %v, want graveyard", o.Zone)
	}
	if got := legendDepartureText(t, e, w2); got != "world rule" {
		t.Fatalf("binned world permanent departed with Text %q, want \"world rule\"", got)
	}
	if markers := worldChooseMarkers(e, l1); len(markers) != 1 || markers[0] != "legend_keep" {
		t.Fatalf("kept legend Choose markers = %v, want [\"legend_keep\"]", markers)
	}
	if markers := worldChooseMarkers(e, w1); len(markers) != 1 || markers[0] != "world_keep" {
		t.Fatalf("kept world Choose markers = %v, want [\"world_keep\"]", markers)
	}
	if e.legendBatch != nil {
		t.Fatalf("a batch is still parked after both rules settled")
	}
}
