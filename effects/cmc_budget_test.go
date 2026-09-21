package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// The WithTotalCMC$ budget tests on the three non-Dig pickers the parameter
// rides (the Dig window was fixed by budget1; these are its three siblings):
//
//   - effHiddenPick  -- Lively Dirge's DBReturn graveyard shape;
//   - effSearchLibrary -- Protean Hulk's death-trigger library search;
//   - effPlay        -- Invoke Calamity's may-play grant.
//
// The askHost double captures the posed decision and suspends; re-entry is a
// second Resolve with the Ctx answer fields set, exactly the engine's
// contract. Every fixture builds its own cards with explicit ManaCost values
// so Face().Cmc() is deterministic. Budget values differ across the cases
// (4, 5, 6) so a silent-zero or uncapped bug cannot pass by coincidence.

// budgetCard builds one card with the given mana value as that many {W} pips.
func budgetCard(t *testing.T, name, types string, mv int) *cards.Card {
	t.Helper()
	pips := make([]string, 0, mv)
	for j := 0; j < mv; j++ {
		pips = append(pips, "{W}")
	}
	src := "Name:" + name + "\nTypes:" + types + "\nManaCost:" +
		strings.Join(pips, "") + "\nOracle:x\n"
	if strings.Contains(types, "Creature") {
		src += "PT:1/1\n"
	}
	return mkCard(t, src)
}

// graveyardBudgetFixture puts seat 0's graveyard in the given MV order and
// returns (host, ids).
func graveyardBudgetFixture(t *testing.T, mvs ...int) (*askHost, []state.ObjID) {
	t.Helper()
	h := &askHost{}
	h.g = state.NewGame(names(2))
	ids := make([]state.ObjID, 0, len(mvs))
	for i, mv := range mvs {
		ids = append(ids, h.g.AddObject(budgetCard(t,
			"Creature"+string(rune('A'+i)), "Creature", mv), 0).ID)
	}
	h.g.SetZone(state.ZGraveyard, 0, ids)
	for _, id := range ids {
		h.g.Obj(id).Zone = state.ZGraveyard // fixture setup; events never ran
	}
	return h, ids
}

// livelyDirgeReturn is Lively Dirge's real DBReturn SVar line verbatim.
const livelyDirgeReturn = "DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | " +
	"WithTotalCMC$ 4 | ChangeNum$ 2 | Hidden$ True | ChangeType$ Creature.YouOwn"

// TestLivelyDirgeReturnBudgetNarrowsEligible: a graveyard card whose own mana
// value exceeds the budget can never be returned. Graveyard [5,2,2,2], budget
// 4 -- the 5-MV card is absent from the options while the three 2-MV cards
// are offered, each naming its mana value in Option.Value.
func TestLivelyDirgeReturnBudgetNarrowsEligible(t *testing.T) {
	h, ids := graveyardBudgetFixture(t, 5, 2, 2, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t, livelyDirgeReturn))
	if h.asked == nil {
		t.Fatal("no decision: the hidden pick must ask whenever a card is offered")
	}
	d := h.asked
	if d.ResumeKind != "hidden_pick" {
		t.Fatalf("ResumeKind = %q, want hidden_pick", d.ResumeKind)
	}
	if d.MaxSum != 4 {
		t.Fatalf("MaxSum = %d, want 4", d.MaxSum)
	}
	if len(d.Options) != 3 {
		t.Fatalf("options = %+v, want exactly the three 2-MV cards (the 5-MV excluded)", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == ids[0] {
			t.Fatalf("the 5-MV card %d was offered under a budget of 4", ids[0])
		}
		if o.Value != 2 {
			t.Fatalf("option %+v Value = %d, want each offered card's mana value 2", o, o.Value)
		}
	}
	// An over-budget answer is rejected on the wire -- with Max 2 the only
	// way to exceed budget 4 here is a two-pick, and every pair sums to 4, so
	// the cumulative rejection is pinned by the [3,3] fixture below instead.
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 1}}); err != nil {
		t.Fatalf("an at-budget pair was rejected: %v", err)
	}
}

// TestLivelyDirgeReturnBudgetCapsCumulative: the budget is cumulative, not
// per card. Graveyard [3,3,2], budget 5, ChangeNum 2 -- all three are
// individually affordable but the two 3s together exceed the cap, so
// Validate rejects that pair, accepts one summing to 5, and the re-entered
// answer moves exactly the answered cards.
func TestLivelyDirgeReturnBudgetCapsCumulative(t *testing.T) {
	h, ids := graveyardBudgetFixture(t, 3, 3, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | "+
			"WithTotalCMC$ 5 | ChangeNum$ 2 | Hidden$ True | ChangeType$ Creature.YouOwn"))
	if h.asked == nil {
		t.Fatal("no decision posed")
	}
	d := h.asked
	if d.MaxSum != 5 {
		t.Fatalf("MaxSum = %d, want 5", d.MaxSum)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 1}}); err == nil {
		t.Fatal("a 3+3 pair exceeding the budget 5 passed Validate; the cumulative cap must be enforced")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 2}}); err != nil {
		t.Fatalf("an at-budget pair (3+2=5) was rejected: %v", err)
	}
	// Re-entry with the in-budget answer moves exactly those two cards to
	// the battlefield; the unpicked third stays in the graveyard.
	ctx := &Ctx{Controller: 0, HiddenPick: []state.ObjID{ids[0], ids[2]}, HiddenPickDone: true}
	Resolve(h, ctx, sa(t,
		"DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | "+
			"WithTotalCMC$ 5 | ChangeNum$ 2 | Hidden$ True | ChangeType$ Creature.YouOwn"))
	if bf := h.g.Zone(state.ZBattlefield, 0); len(bf) != 2 || bf[0] != ids[0] || bf[1] != ids[2] {
		t.Fatalf("battlefield = %v, want exactly the answered [%d %d]", bf, ids[0], ids[2])
	}
	if gy := h.g.Zone(state.ZGraveyard, 0); len(gy) != 1 || gy[0] != ids[1] {
		t.Fatalf("graveyard = %v, want the unpicked [%d]", gy, ids[1])
	}
}

// TestMandatoryBudgetHiddenPickLowersMin: a Mandatory$ pick whose ChangeNum
// exceeds what the budget affords must not demand more picks than it can pay
// for -- the Min lowers to the forced greedy count (effDig's rule), so a
// satisfying answer exists. Graveyard [3,3], budget 4, ChangeNum 2: the
// greedy take is one card (3+3 > 4), so Min is 1, not 2.
func TestMandatoryBudgetHiddenPickLowersMin(t *testing.T) {
	h, _ := graveyardBudgetFixture(t, 3, 3)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | "+
			"WithTotalCMC$ 4 | ChangeNum$ 2 | Hidden$ True | Mandatory$ True | ChangeType$ Creature.YouOwn"))
	if h.asked == nil {
		t.Fatal("no decision posed")
	}
	d := h.asked
	if d.Min != 1 {
		t.Fatalf("Min = %d, want 1 (the greedy count under the budget 4)", d.Min)
	}
	if d.Max != 2 {
		t.Fatalf("Max = %d, want the ChangeNum 2", d.Max)
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}
	if err := d.Validate(in); err != nil {
		t.Fatalf("the greedy single pick was rejected: %v", err)
	}
}

// proteanHulkSearch is Protean Hulk's real TrigChangeZone SVar line verbatim;
// its ChangeNum$ X resolves through the card's own
// SVar:X:Count$ValidLibrary Creature.YouCtrl, supplied in the Ctx here.
const proteanHulkSearch = "DB$ ChangeZone | Origin$ Library | Destination$ Battlefield | " +
	"ChangeNum$ X | WithTotalCMC$ 6 | ChangeType$ Creature.YouCtrl"

// TestProteanHulkSearchBudgetNarrowsEligible: the library search narrows to
// the affordable pool and carries the budget on the wire. Library
// [8,4,4,2] creatures (so X = Count$ValidLibrary Creature.YouCtrl = 4),
// budget 6: the 8-MV card is not offered, MaxSum is 6, and the two 4s
// together exceed the cap so Validate rejects that pair while 4+2 fits.
func TestProteanHulkSearchBudgetNarrowsEligible(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	ids := make([]state.ObjID, 0, 4)
	for i, mv := range []int{8, 4, 4, 2} {
		ids = append(ids, h.g.AddObject(budgetCard(t,
			"Creature"+string(rune('A'+i)), "Creature", mv), 0).ID)
	}
	h.g.SetZone(state.ZLibrary, 0, ids)
	Resolve(h, &Ctx{Controller: 0, SVars: map[string]string{
		"X": "Count$ValidLibrary Creature.YouCtrl"}}, sa(t, proteanHulkSearch))
	if h.asked == nil {
		t.Fatal("no decision: the budget forces a choice the forced take cannot make")
	}
	d := h.asked
	if d.ResumeKind != "search" {
		t.Fatalf("ResumeKind = %q, want search", d.ResumeKind)
	}
	if d.MaxSum != 6 {
		t.Fatalf("MaxSum = %d, want 6", d.MaxSum)
	}
	if len(d.Options) != 3 {
		t.Fatalf("options = %+v, want exactly the three affordable creatures (the 8-MV excluded)", d.Options)
	}
	if d.Options[0].Obj == ids[0] {
		t.Fatalf("the 8-MV card %d was offered under a budget of 6", ids[0])
	}
	if d.Options[0].Value != 4 {
		t.Fatalf("option Value = %d, want the offered card's mana value 4", d.Options[0].Value)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 1}}); err == nil {
		t.Fatal("a 4+4 pair exceeding the budget 6 passed Validate")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 2}}); err != nil {
		t.Fatalf("an at-budget pair (4+2=6) was rejected: %v", err)
	}
}

// invokeCalamityPlay is Invoke Calamity's real Play SVar line (minus the
// SubAbility/ReplaceGraveyard riders and the Hand half of ValidZone, which
// this fixture does not exercise).
const invokeCalamityPlay = "SP$ Play | Valid$ Card.YouOwn | ValidSA$ Instant,Sorcery | " +
	"WithTotalCMC$ 6 | ValidZone$ Graveyard | Amount$ 2 | WithoutManaCost$ True | Optional$ True"

// TestInvokeCalamityPlayBudgetNarrowsAndCaps: the may-play grant offers only
// the individually affordable cards, names each card's mana value in
// Option.Value, and enforces the cumulative cap on the wire. Graveyard
// [7MV instant, 2MV instant, 2MV sorcery, 5MV instant], budget 6: the 7-MV
// card is not offered; all three spells together (9) and the 5+2 pairs (7)
// exceed the cap, a lone 5 or a 2+2 pair fits.
func TestInvokeCalamityPlayBudgetNarrowsAndCaps(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	ids := []state.ObjID{
		h.g.AddObject(budgetCard(t, "BigBurn", "Instant", 7), 0).ID,
		h.g.AddObject(budgetCard(t, "SmallBurn", "Instant", 2), 0).ID,
		h.g.AddObject(budgetCard(t, "SmallWisdom", "Sorcery", 2), 0).ID,
		h.g.AddObject(budgetCard(t, "MidBurn", "Instant", 5), 0).ID,
	}
	h.g.SetZone(state.ZGraveyard, 0, ids)
	Resolve(h, &Ctx{Controller: 0}, sa(t, invokeCalamityPlay))
	if h.asked == nil {
		t.Fatal("no KModes decision posed")
	}
	d := h.asked
	if d.Kind != decision.KModes || d.ResumeKind != "play" {
		t.Fatalf("decision = %+v, want a play KModes", d)
	}
	if d.MaxSum != 6 {
		t.Fatalf("MaxSum = %d, want 6", d.MaxSum)
	}
	if len(d.Options) != 3 {
		t.Fatalf("options = %+v, want the three affordable spells (the 7-MV excluded)", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == ids[0] {
			t.Fatalf("the 7-MV spell %d was offered under a budget of 6", ids[0])
		}
		if o.Value == 0 {
			t.Fatalf("option %+v carries no Value; a budget Play names each card's mana value", o)
		}
	}
	// Over-budget answers rejected: three spells (9 > 6) and a 5+2 pair (7 > 6).
	idxOf := func(id state.ObjID) int {
		for _, o := range d.Options {
			if o.Obj == id {
				return o.Index
			}
		}
		t.Fatalf("spell %d not offered", id)
		return -1
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{idxOf(ids[1]), idxOf(ids[2]), idxOf(ids[3])}}); err == nil {
		t.Fatal("a three-spell answer exceeding the budget 6 passed Validate")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{idxOf(ids[1]), idxOf(ids[3])}}); err == nil {
		t.Fatal("a 2+5 pair exceeding the budget 6 passed Validate")
	}
	// In-budget answers accepted.
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idxOf(ids[3])}}); err != nil {
		t.Fatalf("a lone 5-MV spell was rejected: %v", err)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{idxOf(ids[1]), idxOf(ids[2])}}); err != nil {
		t.Fatalf("a 2+2 pair within the budget was rejected: %v", err)
	}
}

// TestBudgetlessBodiesCarryNoBudgetWire: without WithTotalCMC$ the three
// pickers keep their exact pre-budget wire shape -- Decision.MaxSum 0 and
// Option.Value 0 (omitempty drops both fields, so existing decision
// serialisations are byte-identical).
func TestBudgetlessBodiesCarryNoBudgetWire(t *testing.T) {
	h, _ := graveyardBudgetFixture(t, 3, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | "+
			"ChangeNum$ 2 | Hidden$ True | ChangeType$ Creature.YouOwn"))
	if h.asked == nil {
		t.Fatal("no decision posed")
	}
	if h.asked.MaxSum != 0 {
		t.Fatalf("hidden pick MaxSum = %d, want 0 without WithTotalCMC$", h.asked.MaxSum)
	}
	for _, o := range h.asked.Options {
		if o.Value != 0 {
			t.Fatalf("hidden pick option %+v carries Value %d without a budget", o, o.Value)
		}
	}
	h2 := &askHost{}
	h2.g = state.NewGame(names(2))
	gy := []state.ObjID{h2.g.AddObject(budgetCard(t, "Burn", "Instant", 3), 0).ID}
	h2.g.SetZone(state.ZGraveyard, 0, gy)
	h2.g.Obj(gy[0]).Zone = state.ZGraveyard // fixture setup; events never ran
	Resolve(h2, &Ctx{Controller: 0}, sa(t,
		"SP$ Play | Valid$ Card.YouOwn | ValidSA$ Instant | ValidZone$ Graveyard | Optional$ True"))
	if h2.asked == nil {
		t.Fatal("no play decision posed")
	}
	if h2.asked.MaxSum != 0 {
		t.Fatalf("play MaxSum = %d, want 0 without WithTotalCMC$", h2.asked.MaxSum)
	}
	for _, o := range h2.asked.Options {
		if o.Value != 0 {
			t.Fatalf("play option %+v carries Value %d without a budget", o, o.Value)
		}
	}
}
