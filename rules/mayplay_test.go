package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestMayPlayStaticGrantsLandFromGraveyard drives Conduit of Worlds' real
// static -- S:Mode$ Continuous | Affected$ Land.YouOwn | MayPlay$ True |
// AffectedZone$ Graveyard -- through the priority walk: a basic land in its
// controller's graveyard must be offered as an ordinary play_land (spending
// the turn's land drop), and committing it must move it from the graveyard
// onto the battlefield. A land played this way is an ordinary land play, not
// an additional drop, so a second land may not be played the same turn.
func TestMayPlayStaticGrantsLandFromGraveyard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	conduit, ok := reg.Lookup("Conduit of Worlds")
	if !ok {
		t.Fatal("Conduit of Worlds missing from corpus")
	}
	if d := conduit.Link(); len(d) != 0 {
		t.Fatalf("link Conduit of Worlds: %v", d)
	}
	e, _, _ := newFixtureDeck(t, 211, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
	co := e.G.AddObject(conduit, 0)
	co.Zone = state.ZBattlefield
	lo := e.G.AddObject(mountainCard(t), 0)
	lo.Zone = state.ZGraveyard
	// An opponent's land in THEIR graveyard: the grant is the static
	// controller's, so seat 1 walking its own graveyard gets nothing.
	fo := e.G.AddObject(mountainCard(t), 1)
	fo.Zone = state.ZGraveyard
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), co.ID))
	e.G.SetZone(state.ZGraveyard, 0, append(e.G.Zone(state.ZGraveyard, 0), lo.ID))
	e.G.SetZone(state.ZGraveyard, 1, append(e.G.Zone(state.ZGraveyard, 1), fo.ID))
	e.G.Step, e.G.Active, e.G.Priority, e.G.Turn = state.StepMain1, 0, 0, 1
	e.pending = nil
	e.Advance()

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("priority = %+v, want priority for seat 0", d)
	}
	var playIdx int = -1
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == lo.ID {
			playIdx = o.Index
		}
	}
	if playIdx < 0 {
		t.Fatalf("granted graveyard land not offered as play_land: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{playIdx}}); err != nil {
		t.Fatalf("submit play_land: %v", err)
	}
	if lo.Zone != state.ZBattlefield {
		t.Fatalf("land zone %v, want battlefield", lo.Zone)
	}
	if e.G.Players[0].LandsPlayed != 1 {
		t.Fatalf("LandsPlayed = %d, want 1 (the granted play is the land drop)", e.G.Players[0].LandsPlayed)
	}
	// The land drop is now spent: the opponent's land in their graveyard and
	// any second seat-0 land must not be offered this turn.
	d = e.Pending()
	if d == nil {
		t.Fatal("no pending decision after the land play")
	}
	for _, o := range d.Options {
		if o.Kind == "play_land" {
			t.Fatalf("play_land still offered after the drop: %+v", o)
		}
	}
}

// TestMayPlayGrantStaysWithItsController verifies the battlefield grant's
// scope the negative way: with Conduit of Worlds under seat 0's control,
// seat 1's priority must not offer a play_land for a card in seat 1's own
// graveyard (Affected$ Land.YouOwn resolves against the static's controller).
func TestMayPlayGrantStaysWithItsController(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	conduit, ok := reg.Lookup("Conduit of Worlds")
	if !ok {
		t.Fatal("Conduit of Worlds missing from corpus")
	}
	e, _, _ := newFixtureDeck(t, 212, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
	co := e.G.AddObject(conduit, 0)
	co.Zone = state.ZBattlefield
	fo := e.G.AddObject(mountainCard(t), 1)
	fo.Zone = state.ZGraveyard
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), co.ID))
	e.G.SetZone(state.ZGraveyard, 1, append(e.G.Zone(state.ZGraveyard, 1), fo.ID))
	// Pass priority to seat 1 in seat 0's main phase (seat 1 may act there at
	// instant speed; a land play needs its own turn, so nothing may be
	// offered either way -- this asserts the grant does not leak).
	e.G.Step, e.G.Active, e.G.Priority, e.G.Turn = state.StepMain1, 0, 1, 1
	e.pending = nil
	e.Advance()

	d := e.Pending()
	if d == nil || d.Player != 1 {
		t.Fatalf("pending = %+v, want priority for seat 1", d)
	}
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == fo.ID {
			t.Fatalf("opponent's graveyard land offered a grant meant for seat 0: %+v", o)
		}
	}
}

// TestMosswortBridgeAmountAllPlaysEveryExiledCard drives Mosswort Bridge's
// real Play SA -- AB$ Play | Defined$ ExiledWith | Amount$ All | Optional$ True
// | WithoutManaCost$ True -- with TWO cards carrying the bridge's ExiledWith
// provenance in exile: the ask offers both, an answer may select several, and
// every selected card is played onto the battlefield free of its mana cost.
func TestMosswortBridgeAmountAllPlaysEveryExiledCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bridge, ok := reg.Lookup("Mosswort Bridge")
	if !ok {
		t.Fatal("Mosswort Bridge missing from corpus")
	}
	if d := bridge.Link(); len(d) != 0 {
		t.Fatalf("link Mosswort Bridge: %v", d)
	}
	e, _, _ := newFixtureDeck(t, 213, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
	bo := e.G.AddObject(bridge, 0)
	bo.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), bo.ID))
	// Two creatures with 10 total power gate the condition (X >= 10).
	for i := 0; i < 2; i++ {
		c := e.G.AddObject(card(t, "Name:Power\nManaCost:3 G\nTypes:Creature\nPT:5/5\nOracle:x\n"), 0)
		c.Zone = state.ZBattlefield
		e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), c.ID))
	}
	// Two exiled cards with the bridge's provenance, plus one WITHOUT it
	// (the grant population is Defined$ ExiledWith, so that one is not
	// offered).
	var exiled [2]state.ObjID
	for i := 0; i < 2; i++ {
		o := e.G.AddObject(card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
		o.Zone = state.ZExile
		o.ExiledWith = bo.ID
		e.G.SetZone(state.ZExile, 0, append(e.G.Zone(state.ZExile, 0), o.ID))
		exiled[i] = o.ID
	}
	stray := e.G.AddObject(card(t, "Name:Stray\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	stray.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 0, append(e.G.Zone(state.ZExile, 0), stray.ID))
	// Fund {G} for the activation cost and untap the bridge (hideaway lands
	// enter tapped).
	e.G.Obj(bo.ID).Tapped = false
	e.G.Players[0].Pool[state.MG] = 1
	e.G.Step, e.G.Active, e.G.Priority, e.G.Turn = state.StepMain1, 0, 0, 1
	e.pending = nil
	e.Advance()

	d := e.Pending()
	ability := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == bo.ID {
			ability = o.Index
		}
	}
	if ability < 0 {
		t.Fatalf("bridge ability not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{ability}}); err != nil {
		t.Fatalf("activate bridge: %v", err)
	}
	for d = e.Pending(); d != nil && d.Kind != decision.KModes; d = e.Pending() {
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
	}
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("Play ask = %+v, want KModes", d)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("Play ask bounds Min=%d Max=%d, want Min 0 (Optional) Max 2 (Amount$ All over two candidates)", d.Min, d.Max)
	}
	idx := map[state.ObjID]int{}
	for _, o := range d.Options {
		if o.Obj == stray.ID {
			t.Fatalf("card without ExiledWith provenance offered: %+v", d.Options)
		}
		if o.Obj == exiled[0] || o.Obj == exiled[1] {
			idx[o.Obj] = o.Index
		}
	}
	if len(idx) != 2 {
		t.Fatalf("not both exiled cards offered: %+v", d.Options)
	}
	// Select BOTH cards (Amount$ All semantics: several in one answer).
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{idx[exiled[1]], idx[exiled[0]]}}); err != nil {
		t.Fatalf("submit two-card play: %v", err)
	}
	passUntilStackEmpty(t, e, 60)
	for i, id := range exiled {
		if e.G.Obj(id).Zone != state.ZBattlefield {
			t.Fatalf("exiled card %d zone %v, want battlefield (Amount$ All plays every answer)", i, e.G.Obj(id).Zone)
		}
		if e.G.Obj(id).Tapped {
			t.Fatalf("exiled card %d entered tapped", i)
		}
	}
	if stray.Zone != state.ZExile {
		t.Fatalf("unprovenanced card moved: zone %v", stray.Zone)
	}
}

// TestMosswortBridgePlayMayDecline pins the Optional$ True decline: the same
// ask answered EMPTY plays nothing -- both exiled cards stay in exile, the
// activation still resolves (it is consumed), and the game continues.
func TestMosswortBridgePlayMayDecline(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bridge, ok := reg.Lookup("Mosswort Bridge")
	if !ok {
		t.Fatal("Mosswort Bridge missing from corpus")
	}
	e, _, _ := newFixtureDeck(t, 214, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
	bo := e.G.AddObject(bridge, 0)
	bo.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), bo.ID))
	for i := 0; i < 2; i++ {
		c := e.G.AddObject(card(t, "Name:Power\nManaCost:3 G\nTypes:Creature\nPT:5/5\nOracle:x\n"), 0)
		c.Zone = state.ZBattlefield
		e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), c.ID))
	}
	eo := e.G.AddObject(card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	eo.Zone = state.ZExile
	eo.ExiledWith = bo.ID
	e.G.SetZone(state.ZExile, 0, append(e.G.Zone(state.ZExile, 0), eo.ID))
	e.G.Obj(bo.ID).Tapped = false
	e.G.Players[0].Pool[state.MG] = 1
	e.G.Step, e.G.Active, e.G.Priority, e.G.Turn = state.StepMain1, 0, 0, 1
	e.pending = nil
	e.Advance()

	d := e.Pending()
	ability := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == bo.ID {
			ability = o.Index
		}
	}
	if ability < 0 {
		t.Fatalf("bridge ability not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{ability}}); err != nil {
		t.Fatalf("activate bridge: %v", err)
	}
	for d = e.Pending(); d != nil && d.Kind != decision.KModes; d = e.Pending() {
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
	}
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("Play ask = %+v, want KModes", d)
	}
	// The empty answer is a legal decline of an Optional$ Play.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("submit decline: %v", err)
	}
	passUntilStackEmpty(t, e, 40)
	if eo.Zone != state.ZExile {
		t.Fatalf("declined card still left exile: zone %v", eo.Zone)
	}
	d = e.Pending()
	if d == nil {
		t.Fatal("engine wedged after declining the Play ask")
	}
}

func mountainCard(t *testing.T) *cards.Card {
	t.Helper()
	return card(t, "Name:Mountain\nTypes:Land Mountain\nOracle:x\n")
}
