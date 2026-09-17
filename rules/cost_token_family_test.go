package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The cost-token family (task inbox-paramcensus-cost-token-family): the five
// heads ParseCost still substituted generic mana for on the repo decks --
// announced PayLife<X>, bare Exile<N/Spec> (battlefield), Draw<N/Spec>
// (previously MIS-BUCKETED into SubCounter by the nonManaCost switch), the
// announced SubCounter<X/Kind> removal, and DamageYou<N> (the unless-pay
// shape the Sacrifice arm pays; the head itself was still unmodelled). One
// engine test per cost kind, each proving the cost is actually paid and that
// an unpayable shape is never offered (or never announced beyond its bound).

// corpusDeckEngine builds a replayable two-seat engine whose seat-0 deck
// begins with hand (left in the opening hand) followed by board (moved onto
// the battlefield with logged MoveZones), mountains filling the rest; seat 1
// is all mountains. Cards must be in the returned cfg's Decks so
// replayCheck can rebuild the game from the log alone. Returns the engine,
// the config, and hand[0]'s hand id (0 when hand is empty).
func corpusDeckEngine(t *testing.T, hand, board []*cards.Card) (*Engine, Config, state.ObjID) {
	t.Helper()
	deck := append(append([]*cards.Card{}, hand...), board...)
	cfg := Config{Seed: 31, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append(deck, mountainDeck(t, 40-len(deck))...),
			mountainDeck(t, 40),
		}}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	var handIDs []state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner != 0 {
			continue
		}
		for _, h := range hand {
			if o.Card == h && o.Zone == state.ZHand {
				handIDs = append(handIDs, o.ID)
			}
		}
		for _, b := range board {
			if o.Card == b {
				e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
			}
		}
	}
	if len(hand) > 0 && len(handIDs) != 1 {
		// The genesis deal may not have dealt the protagonist: bridge it from
		// the library with a logged MoveZone (the same shape
		// newFixtureDeck's bridge uses), then re-drive.
		if len(handIDs) == 0 {
			for i := range e.G.Objs {
				o := &e.G.Objs[i]
				if o.Owner == 0 && o.Card == hand[0] && o.Zone == state.ZLibrary {
					e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZHand})
					handIDs = append(handIDs, o.ID)
					break
				}
			}
			e.pending = nil
			e.Advance()
		}
		if len(handIDs) != 1 {
			t.Fatalf("expected exactly one opening-hand copy of the hand card, got %v", handIDs)
		}
	}
	e.Advance()
	if len(hand) == 0 {
		return e, cfg, 0
	}
	return e, cfg, handIDs[0]
}

// drawsForPlayer counts the Draw events naming p in the log.
func drawsForPlayer(e *Engine, p state.PlayerID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Draw && ev.Player == p {
			n++
		}
	}
	return n
}

// xAskOptions returns the pending X-announcement decision's options, failing
// when the pending decision is not one.
func xAskOptions(t *testing.T, e *Engine) []decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("no X announcement ask pending: %+v", d)
	}
	return d.Options
}

func maxXOf(options []decision.Option) int {
	m := -1
	for _, o := range options {
		if o.Kind == "x" && o.Amount > m {
			m = o.Amount
		}
	}
	return m
}

func chooseX(t *testing.T, e *Engine, x int) {
	t.Helper()
	for _, o := range xAskOptions(t, e) {
		if o.Kind == "x" && o.Amount == x {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("X = %d not offered", x)
}

// TestToxicDelugePaysAnnouncedLife: the announced PayLife<X> cost part is
// announced as the cast's X, bounded by the payer's life total, and the
// settle pays exactly X life beside the mana; the effect sees the same X.
// The unpayable shapes are bounded: no mana means no cast option, and the X
// ask never offers a value beyond the payer's life.
func TestToxicDelugePaysAnnouncedLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	deluge := mustCorpusCard(t, reg, "Toxic Deluge")
	bear := card(t, "Name:Grizzly\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg, delugeID := corpusDeckEngine(t, []*cards.Card{deluge}, []*cards.Card{bear})
	// No mana: the cast is not offered at all (the mana part is unpayable).
	e.askPriority(0)
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == delugeID {
			t.Fatal("Toxic Deluge offered from an empty pool")
		}
	}
	addMana(t, e, 0, "BRR")
	// Seat the payer at 7 life through a logged LifeChange (not a bare field
	// write), so the replay rebuilds the same total.
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -13})
	e.priorityRound()
	submitChoices(t, e, castOption(t, e, delugeID))
	// The X ask is bounded by the payer's life (7), not by mana (the 2 B mana
	// part is X-independent).
	if m := maxXOf(xAskOptions(t, e)); m != 7 {
		t.Fatalf("PayLife<X> options max = %d, want the payer's life 7", m)
	}
	chooseX(t, e, 1)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life; got != 6 {
		t.Fatalf("life after Toxic Deluge X=1 = %d, want 6 (7-1)", got)
	}
	var bearID state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Face().Name == "Grizzly" {
			bearID = id
		}
	}
	if bearID == 0 || e.G.Obj(bearID).Zone != state.ZBattlefield || e.Power(bearID) != 1 || e.Toughness(bearID) != 1 {
		t.Fatalf("bear after -X/-X not a battlefield 1/1 (id %d, p/t %d/%d)",
			bearID, e.Power(bearID), e.Toughness(bearID))
	}
	// A bigger X than the board can survive is still a legal announcement --
	// the cost is life, not the effect's reach. X=3 sweeps the 2/2 bear to
	// the graveyard while the payer keeps 4 life.
	e2, cfg2, delugeID2 := corpusDeckEngine(t, []*cards.Card{deluge}, []*cards.Card{bear})
	addMana(t, e2, 0, "BRR")
	e2.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -13})
	e2.priorityRound()
	e2.askPriority(0)
	submitChoices(t, e2, castOption(t, e2, delugeID2))
	chooseX(t, e2, 3)
	passUntilStackEmpty(t, e2, 20)
	if got := e2.G.Players[0].Life; got != 4 {
		t.Fatalf("life after Toxic Deluge X=3 = %d, want 4", got)
	}
	for _, id := range e2.G.Zone(state.ZGraveyard, 0) {
		if e2.G.Obj(id).Face().Name == "Grizzly" {
			bearID = 0 // found: the sweep took it
		}
	}
	if bearID != 0 {
		t.Fatalf("the 2/2 bear survived a -3/-3 sweep: %+v", e2.G.Obj(bearID))
	}
	replayCheck(t, e2, cfg2)
	replayCheck(t, e, cfg)
}

// TestBareExileCostExilesTheSource: Relic of Progenitus's "{1}, Exile Relic
// of Progenitus: Exile all graveyards" -- the bare Exile<1/CARDNAME> token
// exiles the source from the battlefield as the payment (the singleton
// self-reference asks nothing), the ability resolves, and the mana plus the
// exile requirement gate the offer.
func TestBareExileCostExilesTheSource(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	relicCard := mustCorpusCard(t, reg, "Relic of Progenitus")
	e, cfg, _ := corpusDeckEngine(t, nil, []*cards.Card{relicCard})
	relic := state.ObjID(0)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Face().Name == "Relic of Progenitus" {
			relic = id
		}
	}
	if relic == 0 {
		t.Fatal("Relic not on the battlefield")
	}
	idx := -1
	for i, sa := range relicCard.Faces[0].Abilities {
		if sa.API == "ChangeZoneAll" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("Relic of Progenitus has no ChangeZoneAll ability")
	}
	// A card from each library into that seat's graveyard for the sweep to
	// take (deck cards, moved with logged events, so the replay rebuilds).
	for p := state.PlayerID(0); p < 2; p++ {
		lib := e.G.Zone(state.ZLibrary, p)
		e.emit(events.Event{Kind: events.MoveZone, Obj: lib[len(lib)-1], From: state.ZLibrary, To: state.ZGraveyard})
	}
	// No mana: the ability is not offered (the exile part alone does not pay
	// the {1}).
	e.askPriority(0)
	if _, ok := findAbilityOption(e, relic, idx); ok {
		t.Fatal("Relic's exile-sweep ability offered from an empty pool")
	}
	addMana(t, e, 0, "R")
	e.priorityRound()
	opt, ok := findAbilityOption(e, relic, idx)
	if !ok {
		t.Fatal("Relic's exile-sweep ability not offered with the {1} in the pool")
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(relic); o.Zone != state.ZExile {
		t.Fatalf("Relic after its own exile cost: zone %s, want exile", o.Zone)
	}
	for p := state.PlayerID(0); p < 2; p++ {
		if n := len(e.G.Zone(state.ZGraveyard, p)); n != 0 {
			t.Fatalf("seat %d graveyard has %d cards after the sweep, want 0", p, n)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("seat 0's hand = %d, want %d after the sweep's draw (genesis deal aside)", got, handBefore+1)
	}
	replayCheck(t, e, cfg)
}

// TestDrawCostDrawsThePayer: a Cost$ Draw<1/You> component is a Draw part,
// not the SubCounter removal the old nonManaCost default bucketed it into;
// the activation pays it by drawing the payer one card, then the body
// resolves. A spec naming a trigger-only role has no binding in the cast
// flow and the ability is never offered. Riddlesmith's own Draw<1/You> lives
// on a trigger Execute$ body (whose costs this build's executor does not
// charge -- see the task report), so the payment path is pinned on a fixture
// with the same printed token.
func TestDrawCostDrawsThePayer(t *testing.T) {
	// The parse half: the Draw bucket, on the real Riddlesmith script shape.
	parsed := ParseCost("Draw<1/You>")
	if len(parsed.Unknown) != 0 || len(parsed.Draw) != 1 || parsed.Draw[0].N != 1 || parsed.Draw[0].Spec != "You" {
		t.Fatalf("ParseCost(\"Draw<1/You>\") = %+v, want one Draw part and no Unknown", parsed)
	}
	if len(parsed.SubCounter) != 0 {
		t.Fatalf("Draw<1/You> still mis-bucketed into SubCounter: %+v", parsed.SubCounter)
	}
	src := "Name:LootEngine\nTypes:Artifact Creature\nPT:1/1\n" +
		"A:AB$ GainLife | Cost$ Draw<1/You> | Defined$ You | LifeAmount$ 2\nOracle:x\n"
	engine := card(t, src)
	e, cfg, _ := corpusDeckEngine(t, nil, []*cards.Card{engine})
	id := e.G.Zone(state.ZBattlefield, 0)[0]
	e.askPriority(0)
	opt, ok := findAbilityOption(e, id, 0)
	if !ok {
		t.Fatal("the Draw-cost ability not offered")
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	lifeBefore := e.G.Players[0].Life
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("hand after the Draw<1/You> payment = %d, want %d", got, handBefore+1)
	}
	if got := e.G.Players[0].Life; got != lifeBefore+2 {
		t.Fatalf("life after the activation = %d, want %d (the body resolved)", got, lifeBefore+2)
	}
	replayCheck(t, e, cfg)

	// A Draw spec with no cast-flow binding is unpayable: never offered.
	unbindable := card(t, "Name:LootEngine2\nTypes:Artifact Creature\nPT:1/1\n"+
		"A:AB$ GainLife | Cost$ Draw<1/Player.TriggeredTarget> | Defined$ You | LifeAmount$ 2\nOracle:x\n")
	e2, _, _ := corpusDeckEngine(t, nil, []*cards.Card{unbindable})
	e2.askPriority(0)
	if _, ok := findAbilityOption(e2, e2.G.Zone(state.ZBattlefield, 0)[0], 0); ok {
		t.Fatal("a trigger-only Draw spec was offered as a cast-flow payment")
	}
}

// TestAnnouncedSubCounterCostRemovesCounters: Chandra, Awakened Inferno's
// ultimate (Cost$ SubCounter<X/LOYALTY>, NumDmg$ X) announces X bounded by
// the walker's loyalty, removes exactly X loyalty counters as the payment,
// and deals X to the target. Before the announced form was modelled the cost
// degraded to one generic, no X was announced, and the ultimate dealt zero.
func TestAnnouncedSubCounterCostRemovesCounters(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	chandraCard := mustCorpusCard(t, reg, "Chandra, Awakened Inferno")
	ult := -1
	for i, sa := range chandraCard.Faces[0].Abilities {
		if sa.Params["Ultimate"] == "True" {
			ult = i
		}
	}
	if ult < 0 {
		t.Fatal("Chandra has no ultimate ability")
	}
	bearCard := mustCorpusCard(t, reg, "Grizzly Bears")
	e, cfg, chandra := walkerBoard(t, reg, "Chandra, Awakened Inferno", bearCard)
	e.Advance()
	loyalty := e.G.Obj(chandra).Counter("LOYALTY")
	if loyalty != 6 {
		t.Fatalf("Chandra entered with %d loyalty, want 6", loyalty)
	}
	bear := state.ObjID(0)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Face().Name == "Grizzly Bears" {
			bear = id
		}
	}
	if bear == 0 {
		t.Fatal("the bear is not on the battlefield")
	}
	opt := abilityOption(t, e, chandra, ult)
	submitChoices(t, e, opt.Index)
	if m := maxXOf(xAskOptions(t, e)); m != int(loyalty) {
		t.Fatalf("SubCounter<X/LOYALTY> options max = %d, want the walker's loyalty %d", m, loyalty)
	}
	chooseX(t, e, 2)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("want the ultimate's target ask, got %+v", d)
	}
	targetIdx := -1
	for _, o := range d.Options {
		if o.Obj == bear && o.Kind != "player" {
			targetIdx = o.Index
		}
	}
	if targetIdx < 0 {
		t.Fatalf("the bear not offered as the ultimate's target: %+v", d.Options)
	}
	submitChoices(t, e, targetIdx)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(chandra).Counter("LOYALTY"); got != loyalty-2 {
		t.Fatalf("loyalty after the ultimate = %d, want %d", got, loyalty-2)
	}
	dealt := int32(0)
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Obj == bear {
			dealt += ev.Amount
		}
	}
	if dealt != 2 {
		t.Fatalf("the ultimate dealt %d damage to the bear, want 2", dealt)
	}
	replayCheck(t, e, cfg)
}

// TestDamageYouCostModelledAndPaid: DamageYou<4> parses as a modelled head
// (no Unknown entry, no generic substitution), the strict unless parser still
// declines it for every API the Sacrifice arm does not intercept, and the
// real card's payment is the damage the accepting opponent takes -- pinned
// end to end on the corpus card (the decline arm is the existing
// sacrifice_unless_pay_test.go's).
func TestDamageYouCostModelledAndPaid(t *testing.T) {
	parsed := ParseCost("DamageYou<4>")
	if len(parsed.Unknown) != 0 || len(parsed.DamageYou) != 1 || parsed.DamageYou[0].N != 4 {
		t.Fatalf("ParseCost(\"DamageYou<4>\") = %+v, want one DamageYou part and no Unknown", parsed)
	}
	if _, ok := ParseUnlessCost("DamageYou<4>"); ok {
		t.Fatal("ParseUnlessCost accepted DamageYou<N> -- the strict gate must keep declining it outside the Sacrifice arm")
	}
	reg := testutil.CorpusRegistry(t)
	e, devil := devilToHand(t, reg, "Vexing Devil")
	e.G.Players[0].Pool = state.Mana{state.MR: 1}
	e.askPriority(0)
	submitChoices(t, e, passToCast(t, e, devil))
	ask := drainUntilKModes(t, e, 60)
	if ask == nil {
		t.Fatal("no DamageYou offer posed for the resolving Vexing Devil")
	}
	lifeBefore := e.G.Players[1].Life
	submitChoices(t, e, ask.Options[0].Index) // accept: take the 4 damage
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[1].Life; got != lifeBefore-4 {
		t.Fatalf("the accepting opponent's life = %d, want %d", got, lifeBefore-4)
	}
	if o := e.G.Obj(devil); o.Zone != state.ZGraveyard {
		t.Fatalf("the devil after the accepted DamageYou payment: zone %s, want graveyard (sacrificed)", o.Zone)
	}
}
