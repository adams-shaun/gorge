package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A continuous static affects permanents unless it names another zone (CR 109.2,
// 611.3a): "creatures you control have trample" reaches the battlefield, not a creature
// card in hand or a creature spell on the stack. Forge's StaticAbilityContinuous reads
// the same default for AffectedZone$ (read, not run).

// zoneHandCard adds a card to p's hand, the eventless placement onBoard makes.
func zoneHandCard(t *testing.T, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), p)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, p, append(e.G.Zone(state.ZHand, p), o.ID))
	e.staticEpoch, e.activeEpoch = -1, -1
	e.emit(events.Event{Kind: events.Note, Text: "settle"})
	return o.ID
}

func keywordsHave(keywords []string, head string) bool {
	for _, k := range keywords {
		if strings.EqualFold(cardsKeywordHead(k), head) {
			return true
		}
	}
	return false
}

const zoneBear = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

func TestBattlefieldStaticGrantsPermanentsOnly(t *testing.T) {
	e := layerEngine(t)
	onBoardCard(t, e, 0, corpusCardNamed(t, "Garruk's Uprising"))
	hand := zoneHandCard(t, e, 0, zoneBear)
	if keywordsHave(e.Derived(hand).Keywords, "Trample") {
		t.Fatal("a creature card in hand has the trample Garruk's Uprising gives creatures you control")
	}
	if keywordsHave(e.derivedWith(hand, state.ZStack).Keywords, "Trample") {
		t.Fatal("a creature spell on the stack has the trample Garruk's Uprising gives creatures you control")
	}
	if !keywordsHave(e.derivedWith(hand, state.ZBattlefield).Keywords, "Trample") {
		t.Fatal("read as the permanent it will be, the card has trample")
	}
	if !keywordsHave(e.Derived(onBoard(t, e, 0, zoneBear)).Keywords, "Trample") {
		t.Fatal("a creature on the battlefield has no trample")
	}
}

func TestStaticNamingAZoneReachesIt(t *testing.T) {
	e := layerEngine(t)
	onBoard(t, e, 0, "Name:Herald\nManaCost:2\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature.YouCtrl | AffectedZone$ Hand | AddKeyword$ Flash | Description$ x\nOracle:x\n")
	if !keywordsHave(e.Derived(zoneHandCard(t, e, 0, zoneBear)).Keywords, "Flash") {
		t.Fatal("a static naming the hand grants nothing to a creature card there")
	}
	onBoard(t, e, 0, "Name:Banner\nManaCost:2\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature.YouCtrl | AffectedZone$ Battlefield | AddPower$ 2 | Description$ x\nOracle:x\n")
	if p := e.Derived(zoneHandCard(t, e, 0, zoneBear)).Power; p != 2 {
		t.Fatalf("a creature card in hand has power %d under a static naming the battlefield, want its printed 2", p)
	}
	if p := e.Derived(onBoard(t, e, 0, zoneBear)).Power; p != 4 {
		t.Fatalf("a creature on the battlefield has power %d under a static naming the battlefield, want 4", p)
	}
}
