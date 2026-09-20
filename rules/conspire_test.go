package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// kw:Conspire (CR 702.78a) pinned end to end on the REAL corpus carriers.
// The deck is built from compiled corpus cards only, so no Forge script text
// is committed. None of the Conspire carriers is in any legacy golden deck,
// so no chain head depends on these cards.
//
// The helpers come from search_library_test.go (searchTestRegistry,
// searchCorpusCard, searchMoveByName), cast_test.go (addMana, submitChoices),
// token_replacement_test.go (moveSeededCard), replacement_updated_test.go
// (passUntilStackEmpty) and multikicker_test.go (castOptMode) — all the same
// package. Burn Trail ({3}{R}, "deals 3 damage to any target", K:Conspire) is
// the printed-keyword carrier; Raiding Schemes is the layer-6 grant carrier.

// conspireEngine deals seat 0 a 40-card deck led by the named Conspire
// carrier, with duplicates of every card the tests seed. Cards reach the
// battlefield through moveSeededCard (the seeded harness, a real logged
// MoveZone), so every event in the log is engine-produced and replayCheck
// stays honest.
func conspireEngine(t *testing.T, hero string) (*Engine, Config, *cards.Registry) {
	t.Helper()
	reg := searchTestRegistry(t)
	mountain := searchCorpusCard(t, reg, "Mountain")
	forest := searchCorpusCard(t, reg, "Forest")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	goblin := searchCorpusCard(t, reg, "Goblin Piker")
	deck := []*cards.Card{searchCorpusCard(t, reg, hero)}
	for i := 0; i < 5; i++ {
		deck = append(deck, goblin)
	}
	deck = append(deck, searchCorpusCard(t, reg, "Raiding Schemes"))
	for i := 0; i < 6; i++ {
		deck = append(deck, mountain)
	}
	for i := 0; i < 6; i++ {
		deck = append(deck, forest)
	}
	for len(deck) < 40 {
		deck = append(deck, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 44207, Names: []string{"consp", "opp"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg, reg
}

// seedBattlefield puts one copy of the named corpus card onto seat 0's
// battlefield through the seeded harness and returns the object id.
func seedBattlefield(t *testing.T, e *Engine, reg *cards.Registry, name string) state.ObjID {
	t.Helper()
	return moveSeededCard(t, e, 0, searchCorpusCard(t, reg, name), state.ZBattlefield)
}

// conspireAskOptions returns the pending Conspire tap election, failing when
// the decision is not the expected Min=Max=2 KChoose.
func conspireAskOptions(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 2 || d.Max != 2 || len(d.Options) == 0 || d.Options[0].Kind != "conspire" {
		t.Fatalf("expected a conspire Min=Max=2 KChoose, got %+v", d)
	}
	return d
}

// chooseConspire submits the two named objects from the conspire election.
func chooseConspire(t *testing.T, e *Engine, a, b state.ObjID) {
	t.Helper()
	d := conspireAskOptions(t, e)
	ia, ib := -1, -1
	for _, o := range d.Options {
		if o.Obj == a {
			ia = o.Index
		}
		if o.Obj == b {
			ib = o.Index
		}
	}
	if ia < 0 || ib < 0 {
		t.Fatalf("conspire options missing %d/%d: %+v", a, b, d.Options)
	}
	submitChoices(t, e, ia, ib)
}

// chooseTargetPlayer answers a pending target decision with the player option
// naming p.
func chooseTargetPlayer(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Player == p {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("player %d not offered: %+v", p, d.Options)
}

// tappedByCost reports whether id carries the pay-time Tap the cast flow
// emits ("tapped as a cost").
func tappedByCost(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Tap && ev.Obj == id && ev.Text == "tapped as a cost" {
			return true
		}
	}
	return false
}

// TestConspirePrintedOfferPayAndCopy is Burn Trail ({3}{R} sorcery, "deals 3
// damage to any target", K:Conspire): the conspired cast option is offered,
// the tap election is a Min=Max=2 KChoose over exactly the eligible
// shared-colour creatures, the two chosen creatures are tapped, exactly one
// copy resolves (3 damage twice = 6) and the pay-time CastInfo carries the
// "conspired" flag.
func TestConspirePrintedOfferPayAndCopy(t *testing.T) {
	e, cfg, reg := conspireEngine(t, "Burn Trail")
	bear := seedBattlefield(t, e, reg, "Grizzly Bears") // green: shares no colour with {R}
	g1 := seedBattlefield(t, e, reg, "Goblin Piker")
	g2 := seedBattlefield(t, e, reg, "Goblin Piker")
	g3 := seedBattlefield(t, e, reg, "Goblin Piker")

	hero := searchMoveByName(t, e, "Burn Trail", state.ZHand)
	addMana(t, e, 0, "RRRR") // {3}{R}

	opt := castOptMode(t, e.Pending().Options, hero, "conspired")
	submitChoices(t, e, opt.Index)

	// Exactly the three red Goblins are offered; the green Bear is not.
	d := conspireAskOptions(t, e)
	if len(d.Options) != 3 {
		t.Fatalf("conspire options %+v, want exactly the 3 shared-colour Goblins", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == bear {
			t.Fatal("the green Grizzly Bears must not be an eligible conspire creature")
		}
	}
	chooseConspire(t, e, g1, g2)

	// Burn Trail targets any target; the copy keeps the original's target
	// (the MayChooseTarget$ stand-in), so answer the opponent.
	chooseTargetPlayer(t, e, 1)
	passUntilStackEmpty(t, e, 40)

	if !e.G.Obj(g1).Tapped || !e.G.Obj(g2).Tapped {
		t.Fatalf("conspire taps: g1 tapped=%v g2 tapped=%v", e.G.Obj(g1).Tapped, e.G.Obj(g2).Tapped)
	}
	if e.G.Obj(g3).Tapped {
		t.Fatal("an unselected eligible creature must not be tapped")
	}
	if !tappedByCost(e, g1) || !tappedByCost(e, g2) {
		t.Fatal("the conspire creatures were not tapped as a cost")
	}
	if life := e.G.Players[1].Life; life != 14 {
		t.Fatalf("opponent life %d, want 14 (3 damage twice)", life)
	}
	if !logHasFlag(e, hero, "conspired") {
		t.Fatal("the pay-time CastInfo carries no conspired flag")
	}
	replayCheck(t, e, cfg)
}

// TestConspireDeclinedPlainCast: the plain cast of the same card emits no
// copy, no "conspired" CastInfo and no flag — the byte-identical
// modeFlags("conspired") == "" contract.
func TestConspireDeclinedPlainCast(t *testing.T) {
	e, cfg, reg := conspireEngine(t, "Burn Trail")
	seedBattlefield(t, e, reg, "Goblin Piker")
	seedBattlefield(t, e, reg, "Goblin Piker")
	hero := searchMoveByName(t, e, "Burn Trail", state.ZHand)
	addMana(t, e, 0, "RRRR")

	opt := castOptMode(t, e.Pending().Options, hero, "")
	submitChoices(t, e, opt.Index)
	chooseTargetPlayer(t, e, 1)
	passUntilStackEmpty(t, e, 40)

	if life := e.G.Players[1].Life; life != 17 {
		t.Fatalf("opponent life %d, want 17 (3 damage once, no copy)", life)
	}
	if logHasFlag(e, hero, "conspired") {
		t.Fatal("a plain cast must not carry the conspired flag")
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.CastInfo && ev.Obj == hero {
			t.Fatalf("a plain cast emitted a CastInfo: %+v", ev)
		}
	}
	replayCheck(t, e, cfg)
}

// TestConspireEligibilityBounds: fewer than two eligible shared-colour
// creatures means the conspired option is not offered at all.
func TestConspireEligibilityBounds(t *testing.T) {
	e, _, reg := conspireEngine(t, "Burn Trail")
	// One eligible (red Goblin) and one ineligible (green Bear).
	seedBattlefield(t, e, reg, "Goblin Piker")
	seedBattlefield(t, e, reg, "Grizzly Bears")
	hero := searchMoveByName(t, e, "Burn Trail", state.ZHand)
	addMana(t, e, 0, "RRRR")

	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == hero && o.Mode == "conspired" {
			t.Fatalf("conspired offered with only one eligible creature: %+v", o)
		}
	}
}

// TestConspireGrantedReachesOffer is the report's "a granted Conspire reaches
// the cast offer" goal, minus Rassilon (whose wasCastFromExile grant
// condition is a separate open issue). Raiding Schemes' static grants Conspire
// to each noncreature spell seat 0 casts (Affected$
// Card.nonCreature+YouCtrl+wasCast, AffectedZone$ Stack); Lightning Bolt has
// no printed Conspire, so the conspired option appearing for it is the grant
// at work, and with the enchantment absent the option is gone. The grant
// reaches the OFFER and the tap is paid and flagged; the COPY trigger is
// compiled only from a printed K:Conspire line (cards/keywords.go's
// expandKeywords), so a granted Conspire does not copy -- a known, ledgered
// gap named in the report's ## Issues section, and not the brief's scope
// (which asks only that the grant reach the offer).
func TestConspireGrantedReachesOffer(t *testing.T) {
	e, cfg, reg := conspireEngine(t, "Lightning Bolt")
	seedBattlefield(t, e, reg, "Goblin Piker")
	seedBattlefield(t, e, reg, "Goblin Piker")
	seedBattlefield(t, e, reg, "Goblin Piker")

	bolt := searchMoveByName(t, e, "Lightning Bolt", state.ZHand)
	addMana(t, e, 0, "R")

	// No grant yet: Lightning Bolt is an ordinary instant with no conspired
	// option.
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == bolt && o.Mode == "conspired" {
			t.Fatal("Lightning Bolt offered conspired without a grant")
		}
	}

	// The grant: Raiding Schemes on the battlefield under seat 0.
	seedBattlefield(t, e, reg, "Raiding Schemes")
	addMana(t, e, 0, "") // re-establish the priority decision after the seed
	opt := castOptMode(t, e.Pending().Options, bolt, "conspired")
	submitChoices(t, e, opt.Index)
	bf := e.G.Zone(state.ZBattlefield, 0)
	chooseConspire(t, e, bf[0], bf[1])
	chooseTargetPlayer(t, e, 1)
	passUntilStackEmpty(t, e, 40)
	if life := e.G.Players[1].Life; life != 17 {
		t.Fatalf("opponent life %d, want 17 (3 damage once: the granted cast's offer/tap is real, its copy is not)", life)
	}
	if !e.G.Obj(bf[0]).Tapped || !e.G.Obj(bf[1]).Tapped {
		t.Fatalf("granted conspire taps: bf[0]=%v bf[1]=%v", e.G.Obj(bf[0]).Tapped, e.G.Obj(bf[1]).Tapped)
	}
	if !logHasFlag(e, bolt, "conspired") {
		t.Fatal("the granted conspired cast carries no flag")
	}
	replayCheck(t, e, cfg)
}

// TestConspireExactTwoAutoTaps: with exactly two eligible creatures the tap is
// forced — no conspire KChoose is posed (the strict-supersets convention) and
// both are tapped.
func TestConspireExactTwoAutoTaps(t *testing.T) {
	e, cfg, reg := conspireEngine(t, "Burn Trail")
	g1 := seedBattlefield(t, e, reg, "Goblin Piker")
	g2 := seedBattlefield(t, e, reg, "Goblin Piker")
	hero := searchMoveByName(t, e, "Burn Trail", state.ZHand)
	addMana(t, e, 0, "RRRR")

	opt := castOptMode(t, e.Pending().Options, hero, "conspired")
	submitChoices(t, e, opt.Index)
	// No conspire KChoose: the next decision is the spell's target selection.
	d := e.Pending()
	if d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "conspire" {
		t.Fatalf("a forced two-creature tap must not ask: %+v", d)
	}
	chooseTargetPlayer(t, e, 1)
	passUntilStackEmpty(t, e, 40)

	if !e.G.Obj(g1).Tapped || !e.G.Obj(g2).Tapped {
		t.Fatalf("forced conspire taps: g1=%v g2=%v", e.G.Obj(g1).Tapped, e.G.Obj(g2).Tapped)
	}
	if !logHasFlag(e, hero, "conspired") {
		t.Fatal("the forced conspired cast carries no flag")
	}
	replayCheck(t, e, cfg)
}
