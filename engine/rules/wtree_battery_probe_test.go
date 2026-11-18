package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

const wtreeSrc = `Name:The World Tree
ManaCost:no cost
Types:Land
S:Mode$ Continuous | Affected$ Land.YouCtrl | AddAbility$ AnyMana | IsPresent$ Land.YouCtrl | PresentCompare$ GE6 | Description$ As long as you control six or more lands, lands you control have "{T}: Add one mana of any color."
SVar:AnyMana:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any color.
A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add {G}.
A:AB$ ChangeZone | Cost$ W W U U B B R R G G T Sac<1/CARDNAME> | Origin$ Library | Destination$ Battlefield | ChangeType$ God | ChangeNum$ XFetch | SpellDescription$ Search your library for any number of God cards, put them onto the battlefield, then shuffle.
SVar:XFetch:Count$ValidLibrary God.YouCtrl
Oracle:x`

const holdSrc = `Name:Dwarven Hold
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ R | Amount$ X | SVar:X:Count$xPaid | SpellDescription$ Add {R} for each storage counter removed.
Oracle:x`

const sunlitSrc = `Name:Sunlit Marsh
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T | Produced$ Combo W B | SpellDescription$ Add {W} or {B}.
Oracle:x`

const librarySrc = `Name:Library Fixture
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T | Produced$ Combo U R | SpellDescription$ Add {U} or {R}.
Oracle:x`

func wtreeBoard(t *testing.T, counters int32) (*Engine, state.ObjID) {
	t.Helper()
	wt := card(t, wtreeSrc)
	hold := card(t, holdSrc)
	sunlit := card(t, sunlitSrc)
	lib := card(t, librarySrc)
	swamp := card(t, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n")
	forest := card(t, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n")
	var p0deck []*cards.Card
	var p0board []*cards.Card
	for i := 0; i < 3; i++ {
		p0deck = append(p0deck, swamp, forest)
		p0board = append(p0board, swamp, forest)
	}
	p0board = append(p0board, sunlit, lib, wt, hold)
	p0deck = append(p0deck, sunlit, lib, wt, hold)
	for i := 0; i < 5; i++ {
		p0deck = append(p0deck, swamp)
	}
	var p1deck []*cards.Card
	for i := 0; i < 10; i++ {
		p1deck = append(p1deck, swamp)
	}
	cfg := Config{Seed: 99, Names: []string{"a", "b"}, Decks: [][]*cards.Card{p0deck, p1deck}, Tokens: map[string]*cards.Card{}}
	e := New(cfg)
	e.Advance()
	// put everything on the battlefield for seat 0
	place := func(c *cards.Card) {
		found := false
		for _, cand := range e.G.Zone(state.ZHand, 0) {
			if e.G.Obj(cand).Face().Name == c.Faces[0].Name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: cand, From: state.ZHand, To: state.ZBattlefield})
				found = true
				break
			}
		}
		if !found {
			for _, cand := range e.G.Zone(state.ZLibrary, 0) {
				if e.G.Obj(cand).Face().Name == c.Faces[0].Name {
					e.emit(events.Event{Kind: events.MoveZone, Obj: cand, From: state.ZLibrary, To: state.ZBattlefield})
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("fixture %q not in seat 0's hand or library", c.Faces[0].Name)
		}
	}
	for _, c := range p0board {
		place(c)
	}
	e.pending = nil
	e.Advance()
	putCountersOn(t, e, 0, "Dwarven Hold", "STORAGE", counters)
	var wtID state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Face().Name == "The World Tree" {
			wtID = id
		}
	}
	if wtID == 0 {
		t.Fatal("The World Tree is not on the battlefield")
	}
	e.G.Obj(wtID).Tapped = false
	driveToStep(t, e, 2, 0, state.StepMain1)
	return e, wtID
}

func searchAbilityIndex(e *Engine, wtID state.ObjID) int {
	f := e.G.Obj(wtID).Face()
	for i := range f.Abilities {
		if f.Abilities[i].API == "ChangeZone" {
			return i
		}
	}
	return -1
}

func TestWTreeBatteryProbe(t *testing.T) {
	e, wtID := wtreeBoard(t, 2)
	d := e.Pending()
	if d == nil {
		e.Advance()
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision: %+v", d)
	}
	offered := 0
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind == "activate" && o.Obj == wtID {
			offered++
			t.Logf("offered: %+v", *o)
		}
	}
	t.Logf("options: %d, WT activations offered: %d", len(d.Options), offered)
	// diagnostics
	desc := paymentFor(wtID, true, Cost{})
	hyp := e.potentialManaFor(0, &desc)
	t.Logf("potential: %v", hyp)
	census := e.paymentPlanCensusOf(0, &hyp)
	t.Logf("census: complete=%v relaxable=%v gaps=%v bounded=%v fees=%d", census.complete, census.relaxable, census.gaps, census.bounded, census.fees())
	pp := e.PotentialPaymentPlans(0)
	for _, plan := range pp {
		if plan.Action.Obj == wtID && plan.Action.Kind == "ability" {
			t.Logf("WT potential plan: reason=%q detail=%q script=%d steps", plan.Reason, plan.Detail, len(plan.Script))
			for i, st := range plan.Script {
				l := fmt.Sprint(st.Kind) + ":"
				for _, pk := range st.Picks {
					l += pk.Label + "|" + pk.ManaSymbol + ";" + fmt.Sprint(pk.Ability) + " "
				}
				t.Logf("script[%d]: %s", i, l)
			}
		}
	}
	t.Logf("last prefix search: nodes=%d limited=%v ok=%v ran3=%v lim1=%v depth3=%d drops=%d covered=%d pool=%v found3=%+v", dbgLastSearch.nodes, dbgLastSearch.limited, dbgLastSearch.ok, dbgLastSearch.ran3, dbgLastSearch.lim1, dbgLastSearch.depth3, dbgLastSearch.drops, dbgLastSearch.covered, dbgLastSearch.coverPool, dbgFound3)
	t.Logf("WT obj id: %d", wtID)
	t.Logf("last ask: %s opts=%v", dbgAsk.kind, dbgAsk.opts)
	for i, l := range dbgLastSearch.path3 {
		t.Logf("path3[%d]: %s", i, l)
	}
	for i, l := range dbgFoundTrace {
		if i < 80 {
			t.Logf("ftrace[%d]: %s", i, l)
		}
	}
	// the full search at a large budget: where does it walk deepest?
	if script, reason := e.PotentialPlayScript(0, decision.PotentialAction{Kind: "ability", Obj: wtID, Ability: searchAbilityIndex(e, wtID)}, 4000); true {
		t.Logf("full search: reason=%q script=%d steps", reason, len(script))
	}
	sr := e.newScriptSearch(0, decision.PotentialAction{Kind: "ability", Obj: wtID, Ability: searchAbilityIndex(e, wtID)}, -1, 4000)
	if sr != nil {
		var census paymentPlanCensus
		func() {
			e.beginDerivedMemo()
			defer e.endDerivedMemo()
			hyp := e.PotentialMana(0)
			census = e.paymentPlanCensusOf(0, &hyp)
		}()
		sr.gaps, sr.passes = census.gaps, true
		sr.run(e)
		t.Logf("run: nodes=%d limited=%v approx=%v maxDepth=%d", sr.nodes, sr.limited, sr.approx, sr.maxDepth)
		for i, st := range sr.maxPath {
			t.Logf("maxPath[%d]: kind=%v picks=%+v", i, st.Kind, st.Picks)
		}
	}
	// the offer gate, exactly as the walk prices it
	w := &legalWalk{e: e}
	// the search ability is The World Tree's printed activated ability
	var searchSA *cards.SA
	for _, sa := range e.G.Obj(wtID).Face().Abilities {
		if sa.API == "ChangeZone" {
			searchSA = sa
		}
	}
	if searchSA == nil {
		t.Fatal("fixture: no ChangeZone ability on The World Tree")
	}
	cost := e.abilityCompiledCost(searchSA).Cost
	scope := abilityScope(searchSA)
	t.Logf("cost: %+v tap=%v", cost.Colored, cost.Tap)
	_ = searchSA
	t.Logf("offerCastable=%v", w.offerCastable(0, wtID, cost, scope, true))
	t.Logf("autoPayOffer=%v", w.autoPayOffer(0, wtID, 0, cost, searchSA))
	t.Logf("abilityTargetsAvailable=%v sVarGate=%v", e.abilityTargetsAvailable(0, wtID, searchSA), e.sVarGateOK(0, wtID, searchSA, 0))
	// window units: is the battery and each granted ability there?
	for _, u := range e.windowManaUnits(0) {
		o := e.G.Obj(u.ID)
		als := ""
		for _, a := range u.Alts {
			als += effects.ManaOf(a.Ma).Produced + "/" + fmt.Sprint(e.manaStaticFor(0, u.ID, a.Ma).Amount) + " "
		}
		t.Logf("unit %s: free=%d alts=%s", o.Face().Name, u.FreeCount, als)
	}
	if offered == 0 {
		t.Fatalf("the World Tree search activation is not offered (%d options)", len(d.Options))
	}
	if offered == 0 {
		t.Fatalf("the World Tree search activation is not offered (%d options)", len(d.Options))
	}
	t.Fatalf("search offered; see logs")
}
