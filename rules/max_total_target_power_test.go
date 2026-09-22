package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The MaxTotalTargetPower$ cap end-to-end pin on the REAL compiled corpus
// card: "Return any number of target creature cards with total power 10 or
// less from your graveyard to the battlefield. Exile Reunion of the House."
// Reunion of the House is in NO repo deck and NO legacy golden deck
// (grepping internal/testutil/decks/*.json returns nothing), so no chain
// head depends on this card. Nethroi, Apex of Death carries the same
// parameter on its mutate trigger; the shared read is pinned from the
// ability side by the authored fixture in
// TestTotalPowerCapOnAnAbilityTargetAsk.
//
// The deck is built from compiled corpus cards only (the
// search_library_test.go convention), so no Forge script text is committed
// here either; the ability fixture is an authored inline test fixture, not
// a Forge script.

// reunionEngine seats the cast subject (the named corpus card, or the
// authored fixture card when one is given) in seat 0's hand and seat 0's
// graveyard with creatures whose powers straddle the cap of 10:
//
//	Polar Kraken 11 (individually over the cap -- never offered)
//	Craw Wurm 6 · Serra Angel 4 · Hill Giant 3 · Grizzly Bears 2
//
// Precondition the assertions depend on: the target creatures are IN the
// graveyard and the protagonist IS in seat 0's hand (a library copy is
// bridged with a logged move, the newFixtureDeck convention).
func reunionEngine(t *testing.T, reg *cards.Registry, seed uint64, fixture *cards.Card, names ...string) (*Engine, Config, state.ObjID, map[string]state.ObjID) {
	t.Helper()
	forest := searchCorpusCard(t, reg, "Forest")
	deck := make([]*cards.Card, 0, 40)
	protagonist := ""
	if fixture != nil {
		deck = append(deck, fixture)
		protagonist = fixture.Faces[0].Name
	}
	for _, name := range names {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	for i := 0; i < 8; i++ {
		deck = append(deck, forest)
	}
	for len(deck) < 40 {
		deck = append(deck, searchCorpusCard(t, reg, "Grizzly Bears"))
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = forest
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"reunion", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)

	byName := func(zones ...state.Zone) map[string]state.ObjID {
		out := map[string]state.ObjID{}
		for _, z := range zones {
			for _, id := range e.G.Zone(z, 0) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil {
					out[o.Face().Name] = id
				}
			}
		}
		return out
	}
	found := byName(state.ZHand, state.ZLibrary)
	if protagonist == "" {
		protagonist = names[0]
	}
	if _, ok := found[protagonist]; !ok {
		t.Fatalf("protagonist %q absent from seat 0 hand/library", protagonist)
	}
	// Bridge a library copy into the hand with a logged move, then re-drive
	// (the newFixtureDeck convention: the stale priority snapshot does not
	// offer the bridged card).
	if o := e.G.Obj(found[protagonist]); o.Zone == state.ZLibrary {
		e.emit(events.Event{Kind: events.MoveZone, Obj: found[protagonist], From: state.ZLibrary, To: state.ZHand})
		e.pending = nil
		e.Advance()
		found = byName(state.ZHand, state.ZLibrary)
	}
	// The target creatures into the graveyard with logged moves; the
	// protagonist stays in hand.
	grave := map[string]state.ObjID{}
	for _, name := range names {
		if name == protagonist {
			continue
		}
		id, ok := found[name]
		if !ok {
			t.Fatalf("corpus card %q absent from seat 0 hand/library", name)
		}
		z := e.G.Obj(id).Zone
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZGraveyard})
		grave[name] = id
	}
	e.pending = nil
	e.priorityRound()
	return e, cfg, found[protagonist], grave
}

// reunionCastOption finds the "cast" option for the card in the pending
// priority decision.
func reunionCastOption(t *testing.T, e *Engine, id state.ObjID) decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			return o
		}
	}
	t.Fatalf("no cast option for %d: %+v", id, d.Options)
	return decision.Option{}
}

// TestReunionOfTheHouseTotalPowerCap: casting Reunion asks a KTarget whose
// options exclude the 11-power Polar Kraken, carry each candidate's own
// power as Option.Value, and whose Decision.MaxSum is the cap of 10; an
// answer whose power sum exceeds the cap is rejected on the wire; the
// boundary answer summing to exactly 10 resolves and returns exactly those
// creatures while the untouched ones stay in the graveyard.
func TestReunionOfTheHouseTotalPowerCap(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg, reunion, grave := reunionEngine(t, reg, 5501, nil,
		"Reunion of the House", "Polar Kraken", "Craw Wurm", "Serra Angel", "Hill Giant", "Grizzly Bears")
	addMana(t, e, 0, "WWWWWWW")
	submitChoices(t, e, reunionCastOption(t, e, reunion).Index)

	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the cast-path KTarget ask", d)
	}
	if d.Min != 0 {
		t.Fatalf("Min = %d, want 0 (TargetMin$ 0: any number)", d.Min)
	}
	if d.MaxSum != 10 {
		t.Fatalf("MaxSum = %d, want 10 (MaxTotalTargetPower$ 10)", d.MaxSum)
	}
	// The 11-power Polar Kraken can never be part of any legal selection
	// and must not be offered at all.
	offerSet := map[state.ObjID]decision.Option{}
	for _, o := range d.Options {
		offerSet[o.Obj] = o
	}
	if _, ok := offerSet[grave["Polar Kraken"]]; ok {
		t.Fatalf("the 11-power Polar Kraken was offered under a total-power cap of 10: options=%+v", d.Options)
	}
	// Every offered option names its own power as Value -- the field the
	// cumulative-budget contract sums.
	for name, want := range map[string]int{"Craw Wurm": 6, "Serra Angel": 4, "Hill Giant": 3, "Grizzly Bears": 2} {
		o, ok := offerSet[grave[name]]
		if !ok {
			t.Fatalf("the %d-power %s was not offered: options=%+v", want, name, d.Options)
		}
		if o.Value != want {
			t.Fatalf("%s option Value = %d, want its power %d", name, o.Value, want)
		}
	}
	if len(d.Options) != 4 {
		t.Fatalf("options = %d, want the four under-cap creatures", len(d.Options))
	}
	idxOf := func(id state.ObjID) int { return offerSet[id].Index }

	// Over-budget answers rejected on the wire: 2+3+6=11 and 2+3+4+6=15
	// both exceed the cap 10.
	for _, picks := range [][]state.ObjID{
		{grave["Grizzly Bears"], grave["Hill Giant"], grave["Craw Wurm"]},
		{grave["Grizzly Bears"], grave["Hill Giant"], grave["Serra Angel"], grave["Craw Wurm"]},
	} {
		choices := make([]int, 0, len(picks))
		for _, id := range picks {
			choices = append(choices, idxOf(id))
		}
		if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err == nil {
			t.Fatalf("an over-budget answer %v validated under the cap 10", choices)
		}
	}
	// The boundary: 6+4=10 exactly is legal and resolves.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{idxOf(grave["Craw Wurm"]), idxOf(grave["Serra Angel"])}}); err != nil {
		t.Fatalf("in-budget boundary answer rejected: %v", err)
	}
	passUntilStackEmpty(t, e, 20)

	// Exactly the answered pair is on the battlefield; the unpicked
	// creatures stay in the graveyard.
	for _, name := range []string{"Craw Wurm", "Serra Angel"} {
		id := findByName(e, name, 0)
		if id == 0 {
			t.Fatalf("%s vanished from the game", name)
		}
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Fatalf("%s zone = %s, want the battlefield", name, o.Zone)
		}
	}
	for _, name := range []string{"Grizzly Bears", "Hill Giant", "Polar Kraken"} {
		id := grave[name]
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("%s = %+v, want still in the graveyard", name, o)
		}
	}
	// The SubAbility$ DBExile exiles the spell itself (it is still on the
	// stack while its resolution runs). The spell-completion housekeeping
	// then still emits its stack-to-graveyard resting move -- the documented
	// pre-existing resolution-path quirk for any spell whose own chain moves
	// the spell card mid-resolution (TestChangeZoneWishFindsNothingOutsideTheGame)
	// -- so the assertion is the exile event, not the final zone.
	exiled := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == reunion && ev.From == state.ZStack && ev.To == state.ZExile {
			exiled = true
		}
	}
	if !exiled {
		t.Fatal("Reunion's SubAbility$ self-exile did not run")
	}
	replayCheck(t, e, cfg)
}

// TestReunionNoLegalTargetWithinCapResolvesUntargeted: when every creature
// in the graveyard is individually over the cap, the cast still resolves --
// "any number" includes zero -- with no target decision posed at all
// (Min 0's totality rule, targetAsk's len==0 exit).
func TestReunionNoLegalTargetWithinCapResolvesUntargeted(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg, reunion, grave := reunionEngine(t, reg, 5502, nil,
		"Reunion of the House", "Polar Kraken", "Craw Wurm", "Serra Angel", "Hill Giant", "Grizzly Bears")
	// Empty the graveyard of everything under the cap: only the 11-power
	// Kraken stays eligible, and it alone busts the cap of 10.
	for _, name := range []string{"Craw Wurm", "Serra Angel", "Hill Giant", "Grizzly Bears"} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: grave[name], From: state.ZGraveyard, To: state.ZExile})
	}
	e.pending = nil
	e.priorityRound()
	addMana(t, e, 0, "WWWWWWW")
	submitChoices(t, e, reunionCastOption(t, e, reunion).Index)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		t.Fatalf("a target ask was posed though no candidate fits the cap: %+v", d)
	}
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(grave["Polar Kraken"]); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Polar Kraken = %+v, want still in the graveyard", o)
	}
	// The untargeted resolution still runs the SubAbility$ self-exile (the
	// final resting zone is the graveyard through the documented bury-tail
	// quirk, so the assertion is the exile event).
	exiled := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == reunion && ev.From == state.ZStack && ev.To == state.ZExile {
			exiled = true
		}
	}
	if !exiled {
		t.Fatal("Reunion's SubAbility$ self-exile did not run after the untargeted resolution")
	}
	replayCheck(t, e, cfg)
}

// TestTotalPowerCapOnAnAbilityTargetAsk: the askTarget (ability) path --
// Nethroi, Apex of Death's trigger carries the same parameter -- is pinned
// on an authored activated-ability fixture carrying the identical parameter
// shape (TargetMin$ 0 | TargetMax$ X | ValidTgts$ Creature.YouOwn |
// MaxTotalTargetPower$ 10) so the two ask sites cannot drift.
func TestTotalPowerCapOnAnAbilityTargetAsk(t *testing.T) {
	reg := searchTestRegistry(t)
	fixture := card(t, "Name:PowerReclaimer\nManaCost:2\nTypes:Creature\n"+
		"A:AB$ ChangeZone | Cost$ 0 | Origin$ Graveyard | Destination$ Battlefield | "+
		"TargetMin$ 0 | TargetMax$ X | ValidTgts$ Creature.YouOwn | MaxTotalTargetPower$ 10\n"+
		"SVar:X:Count$ValidGraveyard Creature.YouOwn\nOracle:x\n")
	e, cfg, src, _ := reunionEngine(t, reg, 9103, fixture,
		"Polar Kraken", "Craw Wurm", "Grizzly Bears", "Hill Giant")
	// Precondition: the four creatures are in the graveyard.
	grave := map[string]state.ObjID{}
	for _, cid := range e.G.Zone(state.ZGraveyard, 0) {
		if o := e.G.Obj(cid); o != nil && o.Face() != nil {
			grave[o.Face().Name] = cid
		}
	}
	for _, name := range []string{"Polar Kraken", "Craw Wurm", "Grizzly Bears", "Hill Giant"} {
		if grave[name] == 0 {
			t.Fatalf("corpus card %q not in seat 0's graveyard (precondition)", name)
		}
	}
	// Precondition: the fixture is on the battlefield (the ability's own
	// activation requirement).
	if o := e.G.Obj(src); o == nil || o.Zone == state.ZLibrary || o.Zone == state.ZHand {
		e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: o.Zone, To: state.ZBattlefield})
	}
	if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("PowerReclaimer = %+v, want on the battlefield (precondition)", o)
	}
	e.pending = nil
	e.priorityRound()

	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	opt := abilityOption(t, e, src, 0)
	submitChoices(t, e, opt.Index)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the ability-path KTarget ask", d)
	}
	if d.MaxSum != 10 {
		t.Fatalf("MaxSum = %d, want 10 (MaxTotalTargetPower$ 10)", d.MaxSum)
	}
	offerSet := map[state.ObjID]decision.Option{}
	for _, o := range d.Options {
		offerSet[o.Obj] = o
	}
	if _, ok := offerSet[grave["Polar Kraken"]]; ok {
		t.Fatalf("the 11-power Polar Kraken was offered under the cap: options=%+v", d.Options)
	}
	// 6+3=9 is in budget; 6+3+2=11 is not.
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{offerSet[grave["Craw Wurm"]].Index, offerSet[grave["Hill Giant"]].Index}}); err != nil {
		t.Fatalf("in-budget answer rejected: %v", err)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{offerSet[grave["Craw Wurm"]].Index, offerSet[grave["Hill Giant"]].Index,
			offerSet[grave["Grizzly Bears"]].Index}}); err == nil {
		t.Fatal("an 11-power answer validated under the cap 10")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{offerSet[grave["Craw Wurm"]].Index, offerSet[grave["Hill Giant"]].Index}}); err != nil {
		t.Fatalf("in-budget answer rejected: %v", err)
	}
	passUntilStackEmpty(t, e, 20)
	for _, name := range []string{"Craw Wurm", "Hill Giant"} {
		if id := findByName(e, name, 0); id == 0 || e.G.Obj(id).Zone != state.ZBattlefield {
			t.Fatalf("%s did not reach the battlefield", name)
		}
	}
	replayCheck(t, e, cfg)
}

// TestTotalPowerCapReadsDerivedPowerInEveryZone: the cap prunes on the
// DERIVED power, not the printed face. Lord of Extinction's printed P/T is
// the characteristic-defining */* (Face().Power() reads 0 for it), but the
// CDA applies in EVERY zone (CR 208.2; rules/layers.go derivedScalarFrom's
// comment) -- with a populated graveyard its derived power is 13, over the
// cap of 10, so it can never be part of any legal selection and must not be
// offered. The first cut pruned on Face().Power(), read Lord as a free
// 0-power target and let it through (findings-r2 MAJOR 1).
func TestTotalPowerCapReadsDerivedPowerInEveryZone(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg, reunion, grave := reunionEngine(t, reg, 5503, nil,
		"Reunion of the House", "Lord of Extinction", "Craw Wurm", "Serra Angel")
	lord := grave["Lord of Extinction"]
	if lord == 0 {
		t.Fatal("Lord of Extinction absent from seat 0's graveyard (precondition)")
	}
	// Precondition the assertions depend on: the printed face is the CDA
	// */* (the zero the first cut pruned on) while the DERIVED power in the
	// graveyard is over the cap. Bridge ten more cards out of the library
	// so the graveyard Lord counts (all cards, both graveyards) pushes it
	// past 10.
	if f := e.G.Obj(lord).Face(); f.Power() != 0 {
		t.Fatalf("Lord of Extinction printed power = %d, want the CDA 0 (precondition)", f.Power())
	}
	moved := 0
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...) {
		if moved >= 10 {
			break
		}
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != "Grizzly Bears" {
			continue
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
		moved++
	}
	if moved < 10 {
		t.Fatalf("only %d Grizzly Bears reachable in seat 0's library (precondition)", moved)
	}
	e.pending = nil
	e.priorityRound()
	if p := e.Power(lord); p <= 10 {
		t.Fatalf("Lord of Extinction derived power in the graveyard = %d, want > 10 (precondition)", p)
	}

	addMana(t, e, 0, "WWWWWWW")
	submitChoices(t, e, reunionCastOption(t, e, reunion).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the cast-path KTarget ask", d)
	}
	if d.MaxSum != 10 {
		t.Fatalf("MaxSum = %d, want 10 (MaxTotalTargetPower$ 10)", d.MaxSum)
	}
	offerSet := map[state.ObjID]decision.Option{}
	for _, o := range d.Options {
		offerSet[o.Obj] = o
	}
	if _, ok := offerSet[lord]; ok {
		t.Fatalf("the derived-13-power Lord of Extinction was offered under the cap of 10: options=%+v", d.Options)
	}
	// The ordinary printed-power candidates are still offered with their
	// own powers as Values, and the boundary answer 6+4=10 resolves.
	for name, want := range map[string]int{"Craw Wurm": 6, "Serra Angel": 4} {
		o, ok := offerSet[grave[name]]
		if !ok {
			t.Fatalf("the %d-power %s was not offered: options=%+v", want, name, d.Options)
		}
		if o.Value != want {
			t.Fatalf("%s option Value = %d, want its power %d", name, o.Value, want)
		}
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{offerSet[grave["Craw Wurm"]].Index, offerSet[grave["Serra Angel"]].Index}}); err != nil {
		t.Fatalf("in-budget boundary answer rejected: %v", err)
	}
	passUntilStackEmpty(t, e, 20)
	for _, name := range []string{"Craw Wurm", "Serra Angel"} {
		if id := findByName(e, name, 0); id == 0 || e.G.Obj(id).Zone != state.ZBattlefield {
			t.Fatalf("%s did not reach the battlefield", name)
		}
	}
	if o := e.G.Obj(lord); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Lord of Extinction = %+v, want still in the graveyard", o)
	}
	replayCheck(t, e, cfg)
}

// TestTotalPowerCapOfZeroIsEnforced: a MaxTotalTargetPower$ resolving to
// zero (or negative) is still a real cap -- every surviving candidate's own
// power is <= the cap, so any subset sums under it and the per-candidate
// pruning alone enforces the bound; the ask attaches NO Decision.MaxSum
// (a MaxSum of 0 reads as NO budget on the wire, not as a zero budget --
// findings-r2 MAJOR 2). The pin: under a literal cap of 0 the 0/5 Wall of
// Roots is offered and pickable, the 2-power Grizzly Bears is pruned, and
// the CDA */* Lord of Extinction (printed power 0, derived power 13) is
// pruned on its DERIVED power -- the first cut offered it as a free target
// because its printed read was 0.
func TestTotalPowerCapOfZeroIsEnforced(t *testing.T) {
	reg := searchTestRegistry(t)
	fixture := card(t, "Name:PowerReclaimerZero\nManaCost:2\nTypes:Creature\n"+
		"A:AB$ ChangeZone | Cost$ 0 | Origin$ Graveyard | Destination$ Battlefield | "+
		"TargetMin$ 0 | TargetMax$ X | ValidTgts$ Creature.YouOwn | MaxTotalTargetPower$ 0\n"+
		"SVar:X:Count$ValidGraveyard Creature.YouOwn\nOracle:x\n")
	e, cfg, src, _ := reunionEngine(t, reg, 5504, fixture,
		"Lord of Extinction", "Wall of Roots", "Grizzly Bears")
	// Precondition the assertions depend on: the three creatures are in the
	// graveyard and the fixture is on the battlefield.
	grave := map[string]state.ObjID{}
	for _, cid := range e.G.Zone(state.ZGraveyard, 0) {
		if o := e.G.Obj(cid); o != nil && o.Face() != nil {
			grave[o.Face().Name] = cid
		}
	}
	for _, name := range []string{"Lord of Extinction", "Wall of Roots", "Grizzly Bears"} {
		if grave[name] == 0 {
			t.Fatalf("corpus card %q not in seat 0's graveyard (precondition)", name)
		}
	}
	if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield {
		e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: o.Zone, To: state.ZBattlefield})
	}
	// Bridge ten more cards out of the library so Lord of Extinction's CDA
	// (all cards, both graveyards) pushes its derived power past 10 -- the
	// printed-power read that the first cut used sees 0 and would offer it.
	moved := 0
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...) {
		if moved >= 10 {
			break
		}
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != "Grizzly Bears" {
			continue
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
		moved++
	}
	if moved < 10 {
		t.Fatalf("only %d Grizzly Bears reachable in seat 0's library (precondition)", moved)
	}
	e.pending = nil
	e.priorityRound()
	if p := e.Power(grave["Lord of Extinction"]); p <= 10 {
		t.Fatalf("Lord of Extinction derived power = %d, want > 10 (precondition)", p)
	}

	opt := abilityOption(t, e, src, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the ability-path KTarget ask", d)
	}
	// The budget is ABSENT (a zero cap reads as no budget on the wire); the
	// cap is carried by the pruning instead.
	if d.MaxSum != 0 {
		t.Fatalf("MaxSum = %d, want 0 (a zero cap attaches no budget)", d.MaxSum)
	}
	offerSet := map[state.ObjID]decision.Option{}
	for _, o := range d.Options {
		offerSet[o.Obj] = o
	}
	for name, power := range map[string]int{"Grizzly Bears": 2, "Lord of Extinction": 0} {
		if _, ok := offerSet[grave[name]]; ok {
			t.Fatalf("the %s (power %d under the read that matters) was offered under a cap of 0: options=%+v",
				name, power, d.Options)
		}
	}
	wall, ok := offerSet[grave["Wall of Roots"]]
	if !ok {
		t.Fatalf("the 0-power Wall of Roots was not offered under a cap of 0: options=%+v", d.Options)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{wall.Index}}); err != nil {
		t.Fatalf("the in-cap Wall of Roots answer was rejected: %v", err)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{wall.Index}}); err != nil {
		t.Fatalf("Wall of Roots answer rejected: %v", err)
	}
	passUntilStackEmpty(t, e, 20)
	if id := findByName(e, "Wall of Roots", 0); id == 0 || e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatal("Wall of Roots did not reach the battlefield")
	}
	for _, name := range []string{"Lord of Extinction", "Grizzly Bears"} {
		if o := e.G.Obj(grave[name]); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("%s = %+v, want still in the graveyard", name, o)
		}
	}
	replayCheck(t, e, cfg)
}
