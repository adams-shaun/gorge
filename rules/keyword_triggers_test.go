package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Task 16 keyword triggers: Undying, Evolve, Exalted, Prowess. Each keyword
// is expanded by cards/keywords.go into an ordinary ChangesZone / Attacks /
// SpellCast trigger routed through trigger_match.go's own modes (see
// keyword_registration_test.go's pin and the acceptance ratchet), so these
// tests drive the same engine paths a real card of each keyword exercises.
//
// The tests emit setup events directly (e.emit) and then call e.priorityRound
// -- the engine's "CR 117.5: handle state-based actions and triggered
// abilities before granting priority" entry point -- to place any queued
// trigger on the stack ahead of passUntilStackEmpty draining it. This deviates
// from the brief's bare e.Advance() calls, which are a no-op here: emitting an
// event directly leaves whatever decision was pending (genesis's first
// priority in newFixtureDeck) untouched, so e.Advance() returns immediately
// without ever running putTriggersOnStack. e.priorityRound() is the same
// refresh existing helpers (addMana, putCreature) use for this exact reason.
// See the task report.

func TestUndyingReturnsOnceWithACounter(t *testing.T) {
	geist := "Name:Geist\nManaCost:G G\nTypes:Creature Spirit\nPT:2/1\nK:Haste\nK:Undying\nOracle:x\n"
	e, cfg, g := newFixtureDeck(t, 81, geist)
	e.emit(events.Event{Kind: events.MoveZone, Obj: g, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Damage, Obj: g, Amount: 3})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(g); o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 || e.Power(g) != 3 {
		t.Fatalf("after first death: %s, counters %d", o.Zone, o.Counter("P1P1"))
	}
	e.emit(events.Event{Kind: events.Damage, Obj: g, Amount: 5})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(g).Zone != state.ZGraveyard {
		t.Fatal("undying returned a creature that had a +1/+1 counter")
	}
	replayCheck(t, e, cfg)
}

func TestEvolveGrowsOnlyForBiggerCreatures(t *testing.T) {
	small := "Name:Small\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n"
	big := "Name:Big\nManaCost:2\nTypes:Creature\nPT:2/2\nOracle:x\n"
	theirs := "Name:Theirs\nManaCost:5\nTypes:Creature\nPT:5/5\nOracle:x\n"
	e, cfg, one := newFixtureDeck(t, 82,
		"Name:One\nManaCost:G\nTypes:Creature Human Ooze\nPT:1/1\nK:Evolve\nOracle:x\n",
		small, big)
	e.emit(events.Event{Kind: events.MoveZone, Obj: one, From: state.ZHand, To: state.ZBattlefield})
	e.priorityRound()
	putCreature(t, e, 0, small) // equal size: must not evolve
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 0 {
		t.Fatal("evolved for an equal-size creature")
	}
	putCreature(t, e, 0, big) // strictly bigger: must evolve
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 1 {
		t.Fatal("did not evolve for a bigger creature")
	}
	// An opponent's creature (seat 1) entering must not evolve seat 0's One:
	// Evolve's expansion is ValidCard$ Creature.YouCtrl+Other, so a seat on
	// the other side of the table is simply not YouCtrl. The card is minted
	// onto seat 1's battlefield via TokenCreate (newFixtureDeck's extras only
	// ever seed seat 0, so there is no real seat-1 card to move).
	e.priorityRound()
	putToken(t, e, 1, theirs, state.ZBattlefield)
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 1 {
		t.Fatal("evolved for an opponent's creature")
	}
	replayCheck(t, e, cfg)
}

// TestDethroneCountsOnlyTheAttackedPlayersLife drives Treasonous Ogre's
// actual compiled script. Dethrone uses the defender carried by the attack
// event: a different player having more life must not make an attack at a
// lower-life opponent eligible.
func TestDethroneCountsOnlyTheAttackedPlayersLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	ogre, ok := reg.Lookup("Treasonous Ogre")
	if !ok {
		t.Fatal("Treasonous Ogre missing from corpus")
	}
	if d := ogre.Link(); len(d) != 0 {
		t.Fatalf("link Treasonous Ogre: %v", d)
	}
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = ogre
	}
	cfg := seatZeroStart(Config{Seed: 85, Names: []string{"ogre", "other", "third"}, Decks: [][]*cards.Card{deck, deck, deck}})
	e := New(cfg)
	var id state.ObjID
	for _, candidate := range e.G.Objs {
		if candidate.Owner == 0 && candidate.Face() != nil && candidate.Face().Name == "Treasonous Ogre" {
			id = candidate.ID
			break
		}
	}
	if id == 0 {
		t.Fatal("Treasonous Ogre was not created")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, To: state.ZBattlefield})
	// Seat 1 is tied for the most life, so attacking it triggers Dethrone.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{id}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
		t.Fatalf("Dethrone at tied-most-life defender gave %d counters, want 1", got)
	}
	// A third player above the defender prevents Dethrone. The condition is
	// greatest life among every player, not merely >= the attacker's life.
	e.emit(events.Event{Kind: events.LifeChange, Player: 2, Amount: 1})
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{id}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
		t.Fatalf("Dethrone fired while an uninvolved player had more life: counters %d", got)
	}
	replayCheck(t, e, cfg)
}

// TestGrantedDethroneTriggers uses Marchesa's real static script. A granted
// keyword must create its rules trigger too; checking HasKeyword alone would
// make the creature visibly have Dethrone while its attacks did nothing.
func TestGrantedDethroneTriggers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	marchesa, ok := reg.Lookup("Marchesa, the Black Rose")
	if !ok {
		t.Fatal("Marchesa missing from corpus")
	}
	if d := marchesa.Link(); len(d) != 0 {
		t.Fatalf("link Marchesa: %v", d)
	}
	bear := card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	deck := make([]*cards.Card, 40)
	deck[0] = marchesa
	for i := 1; i < len(deck); i++ {
		deck[i] = bear
	}
	cfg := seatZeroStart(Config{Seed: 186, Names: []string{"marchesa", "other"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfg)
	var m, b state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner != 0 || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Marchesa, the Black Rose":
			m = o.ID
		case "Bear":
			b = o.ID
		}
	}
	if m == 0 || b == 0 {
		t.Fatalf("fixture ids Marchesa=%d Bear=%d", m, b)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: m, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: b, To: state.ZBattlefield})
	if !e.HasKeyword(b, "Dethrone") {
		t.Fatal("Marchesa did not grant Dethrone")
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{b}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(b).Counter("P1P1"); got != 1 {
		t.Fatalf("granted Dethrone counters=%d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

func TestRiotAndHideawayUseRealCorpusCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	spider, ok := reg.Lookup("Spider-Punk")
	if !ok {
		t.Fatal("Spider-Punk missing from corpus")
	}
	if d := spider.Link(); len(d) != 0 {
		t.Fatalf("link Spider-Punk: %v", d)
	}
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = spider
	}
	cfgSpider := seatZeroStart(Config{Seed: 187, Names: []string{"spider", "other"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfgSpider)
	id := e.G.Objs[0].ID
	// Drive the real card through the shared as-enters selection machinery.
	e.cast = &pendingCast{player: 0, card: id, from: state.ZLibrary, ability: -1}
	e.collectETBChoices(0)
	if len(e.cast.etbs) != 1 || e.cast.etbs[0].kind != "riot" {
		t.Fatalf("riot choices: %#v", e.cast.etbs)
	}
	e.etbAnswer(&decision.Decision{}, []decision.Option{e.cast.etbs[0].options[1]})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	if !e.HasKeyword(id, "Haste") || e.G.Obj(id).Counter("P1P1") != 0 {
		t.Fatal("Riot haste choice was not applied")
	}

	knoll, ok := reg.Lookup("Spinerock Knoll")
	if !ok {
		t.Fatal("Spinerock Knoll missing from corpus")
	}
	if d := knoll.Link(); len(d) != 0 {
		t.Fatalf("link Spinerock Knoll: %v", d)
	}
	for i := range deck {
		deck[i] = knoll
	}
	cfg := seatZeroStart(Config{Seed: 188, Names: []string{"knoll", "other"}, Decks: [][]*cards.Card{deck, deck}})
	e2 := New(cfg)
	kid := e2.G.Objs[0].ID
	e2.emit(events.Event{Kind: events.MoveZone, Obj: kid, From: state.ZLibrary, To: state.ZBattlefield})
	exiled := e2.G.Zone(state.ZExile, 0)
	if len(exiled) != 4 {
		t.Fatalf("Hideaway exiled %d cards, want 4", len(exiled))
	}
	for _, xid := range exiled {
		if e2.G.Obj(xid).ExiledWith != kid {
			t.Fatalf("Hideaway provenance for %d = %d, want %d", xid, e2.G.Obj(xid).ExiledWith, kid)
		}
	}
}

// TestExtortUsesRealCorpusCard drives Crypt Ghast's real compiled script:
// casting a spell with an Extort permanent on the battlefield fires a
// SpellCast trigger whose body poses the optional {W/B} payment, and on pay
// each opponent loses 1 life and the controller gains that much.
func TestExtortUsesRealCorpusCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	ghast, ok := reg.Lookup("Crypt Ghast")
	if !ok {
		t.Fatal("Crypt Ghast missing from corpus")
	}
	if d := ghast.Link(); len(d) != 0 {
		t.Fatalf("link Crypt Ghast: %v", d)
	}
	bolt := card(t, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	deck := []*cards.Card{ghast}
	game := func() (*Engine, Config, state.ObjID) {
		cfg := seatZeroStart(Config{Seed: 190, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{
				append(append([]*cards.Card{}, deck...), mountainDeck(t, 39)...),
				mountainDeck(t, 40)},
			Tokens: map[string]*cards.Card{}})
		e := New(cfg)
		e.Advance()
		// Put Crypt Ghast on seat 0's battlefield, and Bolt in hand.
		var g state.ObjID
		for _, id := range append(e.G.Zone(state.ZLibrary, 0), e.G.Zone(state.ZHand, 0)...) {
			if e.G.Obj(id).Face() != nil && e.G.Obj(id).Face().Name == "Crypt Ghast" {
				g = id
			}
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: g, From: e.G.Obj(g).Zone, To: state.ZBattlefield})
		bo := e.G.AddObject(bolt, 0)
		bo.Zone = state.ZHand
		e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), bo.ID))
		e.G.Players[0].Pool[state.MR] = 2
		e.G.Players[0].Pool[state.MW] = 1
		e.pending = nil
		e.Advance()
		return e, cfg, g
	}

	// Cast Bolt, answer the target (seat 1), then the cast resolves. The
	// Extort trigger fires on the SpellCast; the caster must be able to say
	// "pay" and see the drain. We fund a W so the {W/B} pip is payable.
	e, _, g := game()
	if !e.HasKeyword(g, "Extort") {
		t.Fatal("Crypt Ghast does not grant Extort on the battlefield")
	}
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}
	// Choose the cast option for Bolt.
	var ci int = -1
	for _, o := range d.Options {
		if o.Kind == "cast" {
			ci = o.Index
		}
	}
	if ci < 0 {
		t.Fatalf("no cast option: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{ci}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision for Bolt, got %+v", d)
	}
	var ti int = -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			ti = o.Index
		}
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{ti}}); err != nil {
		t.Fatalf("submit target: %v", err)
	}
	// The cast is paid and pushed. The Extort trigger should now be posed as
	// the next decision (an optional KModes ask), when the trigger drain runs.
	// First allow the trigger drain to reach the ask by passing any priority
	// that arrives before it. The trigger is queued by the PutOnStack that
	// payCast emits; the drain poses it as a KModes.
	d = e.Pending()
	for d != nil && d.Kind == decision.KPriority {
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected Extort KModes ask, got %+v", d)
	}
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	// Choose "pay" (option index 0), then let the trigger resolution (and the
	// Bolt spell) finish draining the stack so the drain's LifeChange lands.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit extort pay: %v", err)
	}
	passUntilStackEmpty(t, e, 40)
	// Bolt deals 1 to the opponent, and Extort drains 1 (opponent) and gains
	// it (caster), so after both resolutions the opponent is life-2 and the
	// caster is life+1.
	if e.G.Players[1].Life != life1-2 {
		t.Fatalf("opponent life %d after extort, want %d", e.G.Players[1].Life, life1-2)
	}
	if e.G.Players[0].Life != life0+1 {
		t.Fatalf("caster life %d after extort, want %d", e.G.Players[0].Life, life0+1)
	}
}

// TestPlayUsesRealCorpusCard drives Spinerock Knoll's real script: its
// activated Play ability plays the card exiled by its own Hideaway (Defined$
// ExiledWith) without paying its mana cost, once an opponent has been dealt 7+
// this turn. We let Hideaway exile the top cards with provenance, swap one for
// a Bear, deal 7 to the opponent, activate the Play ability, and verify the
// Bear is played onto the battlefield from exile without a mana cost.
func TestPlayUsesRealCorpusCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	knoll, ok := reg.Lookup("Spinerock Knoll")
	if !ok {
		t.Fatal("Spinerock Knoll missing from corpus")
	}
	if d := knoll.Link(); len(d) != 0 {
		t.Fatalf("link Spinerock Knoll: %v", d)
	}
	bear := card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	deck := []*cards.Card{knoll}
	cfg := seatZeroStart(Config{Seed: 192, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append(append([]*cards.Card{}, deck...), mountainDeck(t, 39)...),
			mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	// Put Spinerock Knoll on seat 0's battlefield; its Hideaway exiles the top
	// 4 library cards with provenance ExiledWith == the Knoll.
	var kid state.ObjID
	for _, id := range append(e.G.Zone(state.ZLibrary, 0), e.G.Zone(state.ZHand, 0)...) {
		if e.G.Obj(id).Face() != nil && e.G.Obj(id).Face().Name == "Spinerock Knoll" {
			kid = id
		}
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: kid, From: e.G.Obj(kid).Zone, To: state.ZBattlefield})
	if len(e.G.Zone(state.ZExile, 0)) != 4 {
		t.Fatalf("Hideaway should exile 4, got %d", len(e.G.Zone(state.ZExile, 0)))
	}
	// Swap the first exiled card for a Bear with the Knoll's provenance so the
	// Play ability has a card to play.
	var er state.ObjID
	for _, xid := range e.G.Zone(state.ZExile, 0) {
		if e.G.Obj(xid).ExiledWith == kid {
			er = xid
			break
		}
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: er, From: state.ZExile, To: state.ZGraveyard})
	bo := e.G.AddObject(bear, 0)
	bo.Zone = state.ZExile
	bo.ExiledWith = kid
	e.G.SetZone(state.ZExile, 0, append(e.G.Zone(state.ZExile, 0), bo.ID))
	// Deal 7 to the opponent so X (MaxOppDamageThisTurn) >= 7, then fund {R}
	// and untap the Knoll (a hideaway land enters tapped), in a sorcery window.
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 7})
	e.G.Obj(kid).Tapped = false
	e.G.Players[0].Pool[state.MR] = 1
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	e.pending = nil
	e.Advance()

	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}
	var ai int = -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == kid {
			ai = o.Index
		}
	}
	if ai < 0 {
		t.Fatalf("no Spinerock ability option: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{ai}}); err != nil {
		t.Fatalf("submit ability: %v", err)
	}
	// The Play effect poses a KModes choose; answer it (option 0 plays the
	// Bear from exile), then let the cast resolve.
	d = e.Pending()
	for d != nil && d.Kind != decision.KModes {
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected a KModes for the Play choice, got %+v", d)
	}
	var pi int = -1
	for _, o := range d.Options {
		if o.Obj == bo.ID {
			pi = o.Index
		}
	}
	if pi < 0 {
		t.Fatalf("exiled Bear not offered by the Play choice: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pi}}); err != nil {
		t.Fatalf("submit play: %v", err)
	}
	passUntilStackEmpty(t, e, 60)
	// The Bear should now be on seat 0's battlefield, played from exile.
	var bearOnBF bool
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).ID == bo.ID {
			bearOnBF = true
		}
	}
	if !bearOnBF {
		t.Fatalf("Bear not played onto the battlefield from exile: zone=%v", e.G.Obj(bo.ID).Zone)
	}
}

// TestDredgeUsesRealCorpusCard drives Golgari Thug's real script: a card
// with Dredge 4 in the graveyard replaces a draw -- the controller may instead
// mill 4 and return it to hand. We put a Golgari Thug in seat 0's graveyard,
// trigger a draw, answer the dredge ask "yes", and verify the 4 cards were
// milled and the Thug returned to hand (and no card was drawn).
func TestDredgeUsesRealCorpusCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	thug, ok := reg.Lookup("Golgari Thug")
	if !ok {
		t.Fatal("Golgari Thug missing from corpus")
	}
	if d := thug.Link(); len(d) != 0 {
		t.Fatalf("link Golgari Thug: %v", d)
	}
	deck := []*cards.Card{thug}
	cfg := seatZeroStart(Config{Seed: 193, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append(append([]*cards.Card{}, deck...), mountainDeck(t, 39)...),
			mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	// Move the Thug to seat 0's graveyard (it starts in hand).
	var tid state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(id).Face() != nil && e.G.Obj(id).Face().Name == "Golgari Thug" {
			tid = id
		}
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: tid, From: state.ZHand, To: state.ZGraveyard})
	libBefore := len(e.G.Zone(state.ZLibrary, 0))
	handBefore := len(e.G.Zone(state.ZHand, 0))
	// Draw for seat 0 directly through the shared path.
	e.drawCard(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected dredge KModes ask, got %+v", d)
	}
	// Choose "dredge" (option 0).
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit dredge: %v", err)
	}
	if len(e.G.Zone(state.ZLibrary, 0)) != libBefore-4 {
		t.Fatalf("library after dredge = %d, want %d (milled 4)", len(e.G.Zone(state.ZLibrary, 0)), libBefore-4)
	}
	// The Thug leaves the graveyard for hand, so after milling the graveyard
	// holds exactly the 4 milled cards.
	if len(e.G.Zone(state.ZGraveyard, 0)) != 4 {
		t.Fatalf("graveyard after dredge = %d, want 4 (the 4 milled)", len(e.G.Zone(state.ZGraveyard, 0)))
	}
	if len(e.G.Zone(state.ZHand, 0)) != handBefore+1 {
		t.Fatalf("hand after dredge = %d, want %d (Thug returned)", len(e.G.Zone(state.ZHand, 0)), handBefore+1)
	}
	// The Thug is back in hand.
	inHand := false
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if id == tid {
			inHand = true
		}
	}
	if !inHand {
		t.Fatal("Golgari Thug not returned to hand after dredge")
	}
}

// TestSoulbondUsesRealCorpusCard drives Wingcrafter's real script: it has
// K:Soulbond and a continuous static granting Flying to `Creature.PairedWith,
// Creature.Self+Paired`. When it enters paired with another creature, both
// gain Flying (so the paired partner reads HasKeyword Flying).
func TestSoulbondUsesRealCorpusCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	wing, ok := reg.Lookup("Wingcrafter")
	if !ok {
		t.Fatal("Wingcrafter missing from corpus")
	}
	if d := wing.Link(); len(d) != 0 {
		t.Fatalf("link Wingcrafter: %v", d)
	}
	bear := card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	deck := []*cards.Card{wing}
	cfg := seatZeroStart(Config{Seed: 195, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append(append([]*cards.Card{}, deck...), mountainDeck(t, 39)...),
			mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	var wid state.ObjID
	for _, id := range append(e.G.Zone(state.ZLibrary, 0), e.G.Zone(state.ZHand, 0)...) {
		if e.G.Obj(id).Face() != nil && e.G.Obj(id).Face().Name == "Wingcrafter" {
			wid = id
		}
	}
	// A Bear on the battlefield for it to pair with (the Soulbond entry
	// trigger fires while the Wingcrafter resolves its MoveZone, so the Bear
	// must already be present as the Pair candidate).
	bo := e.G.AddObject(bear, 0)
	bo.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), bo.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: wid, From: e.G.Obj(wid).Zone, To: state.ZBattlefield})
	// The Soulbond entry trigger (ChangesZone to battlefield) fires Pair; drain
	// the stack so the pairing (applied by the resolving trigger) is in place
	// before we check it.
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(wid).Paired == 0 {
		t.Fatalf("Wingcrafter not paired on entry: Paired=%d", e.G.Obj(wid).Paired)
	}
	if e.G.Obj(wid).Paired != bo.ID || e.G.Obj(bo.ID).Paired != wid {
		t.Fatalf("pairing not reciprocal: w.Paired=%d b.Paired=%d", e.G.Obj(wid).Paired, e.G.Obj(bo.ID).Paired)
	}
	// Both read Flying via the continuous static's Affected$Paired/PairedWith.
	if !e.HasKeyword(wid, "Flying") || !e.HasKeyword(bo.ID, "Flying") {
		t.Fatalf("paired creatures should both have Flying: w=%v b=%v", e.HasKeyword(wid, "Flying"), e.HasKeyword(bo.ID, "Flying"))
	}
}

// TestMyriadUsesRealCorpusCard drives Chittering Dispatcher's real script:
// a K:Myriad creature, when it attacks, creates a tapped attacking token
// copy for each opponent other than the defending player.
func TestMyriadUsesRealCorpusCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	disperser, ok := reg.Lookup("Chittering Dispatcher")
	if !ok {
		t.Fatal("Chittering Dispatcher missing from corpus")
	}
	if d := disperser.Link(); len(d) != 0 {
		t.Fatalf("link Chittering Dispatcher: %v", d)
	}
	deck := []*cards.Card{disperser}
	cfg := seatZeroStart(Config{Seed: 196, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{
			append(append([]*cards.Card{}, deck...), mountainDeck(t, 39)...),
			mountainDeck(t, 40), mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	var did state.ObjID
	for _, id := range append(e.G.Zone(state.ZLibrary, 0), e.G.Zone(state.ZHand, 0)...) {
		if e.G.Obj(id).Face() != nil && e.G.Obj(id).Face().Name == "Chittering Dispatcher" {
			did = id
		}
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: did, From: e.G.Obj(did).Zone, To: state.ZBattlefield})
	// Dispatcher attacks seat 1; in a 3-seat game there are TWO other
	// opponents (seat 1 defender, seat 2 the extra Myriad target).
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{did}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	// Expect one MyriadCopy token attacking seat 2 created (the defender is
	// seat 1, excluded).
	tokens := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).IsToken && e.G.Obj(id).IsCopy {
			tokens++
		}
	}
	if tokens != 1 {
		t.Fatalf("Myriad created %d attacker tokens, want 1", tokens)
	}
}

func TestExaltedPumpsALoneAttackerAndProwessPumpsOnNoncreatureSpells(t *testing.T) {
	knight := "Name:Knight\nManaCost:1 B\nTypes:Creature Human Knight\nPT:2/1\nK:Exalted\nOracle:x\n"
	other := "Name:Other\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n"
	e, cfg, k := newFixtureDeck(t, 83, knight, other)
	e.emit(events.Event{Kind: events.MoveZone, Obj: k, From: state.ZHand, To: state.ZBattlefield})
	e.priorityRound()
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{k}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if e.Power(k) != 3 {
		t.Fatalf("lone attacker power %d", e.Power(k))
	}
	// End combat so the first pump expires (until-end-of-turn) and k returns
	// to 2/1, so the two-attacker assertion below measures a fresh attack.
	e.cleanupStep()
	if e.Power(k) != 2 {
		t.Fatalf("power after cleanup %d, want 2", e.Power(k))
	}
	o := putCreature(t, e, 0, other)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{k, o}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if e.Power(k) != 2 {
		t.Fatal("exalted fired for two attackers")
	}
	replayCheck(t, e, cfg)

	e2, cfg2, sw := newFixtureDeck(t, 84,
		"Name:Swift\nManaCost:R\nTypes:Creature Human Monk\nPT:1/2\nK:Haste\nK:Prowess\nOracle:x\n",
		"Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	e2.emit(events.Event{Kind: events.MoveZone, Obj: sw, From: state.ZHand, To: state.ZBattlefield})
	e2.priorityRound()
	bolt := addToHand(t, e2, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	addMana(t, e2, 0, "R")
	e2.Advance()
	castObj(t, e2, bolt)
	passUntilStackEmpty(t, e2, 20)
	if e2.Power(sw) != 2 || e2.Toughness(sw) != 3 {
		t.Fatalf("prowess: %d/%d", e2.Power(sw), e2.Toughness(sw))
	}
	replayCheck(t, e2, cfg2)
}
