package rules

// Mirror of the dr-mbc-abbb lockstep divergence (focus-abs2, pair 1042,
// decision 171): Absorb Vis in hand, Three Tree City ({T}: Add {C}, and a
// {2},{T}: "Add X mana of any one color, X is the number of creatures you
// control of the chosen type") untapped, Mana Flare on a battlefield, empty
// pool. TTC's arm is a PAID, indeterminate-amount, any-colour production:
// Mana Flare's TapsForMana extra reflects one mana of the type that tap
// produced, so tapping the arm for black makes two black. The {1}{B}
// basic landcycling is then payable (CR 601.2g: mana abilities may be
// activated while paying a cost) and the offer must exist.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mirrorBoard builds a fresh engine whose battlefield is the named corpus
// cards in order (seat 0 first, then seat 1's), with hand in seat 0's hand,
// at seat 0's main-phase priority. Every permanent is untapped and unsick.
func mirrorBoard(t *testing.T, hand string, bf0, bf1 []string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range append(append(append([]string{}, bf0...), bf1...), hand) {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.SetZone(state.ZHand, 1, nil)
	var hand0 []state.ObjID
	o := e.G.AddObject(lookup(t, reg, hand), 0)
	o.Zone = state.ZHand
	hand0 = append(hand0, o.ID)
	e.G.SetZone(state.ZHand, 0, hand0)
	place := func(n string, p state.PlayerID) state.ObjID {
		o := e.G.AddObject(lookup(t, reg, n), p)
		o.Zone = state.ZBattlefield
		o.SummonSick = false
		e.G.SetZone(state.ZBattlefield, p, append(e.G.Zone(state.ZBattlefield, p), o.ID))
		return o.ID
	}
	for _, n := range bf0 {
		place(n, 0)
	}
	for _, n := range bf1 {
		place(n, 1)
	}
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

// mirrorBoard2 is mirrorBoard with both seats' hands set: hand0 and hand1 are
// the named corpus cards in that order.
func mirrorBoard2(t *testing.T, bf0, bf1, hand0, hand1 []string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range append(append(append(append([]string{}, bf0...), bf1...), hand0...), hand1...) {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.SetZone(state.ZHand, 1, nil)
	setHand := func(p state.PlayerID, names []string) {
		var hand []state.ObjID
		for _, n := range names {
			o := e.G.AddObject(lookup(t, reg, n), p)
			o.Zone = state.ZHand
			hand = append(hand, o.ID)
		}
		e.G.SetZone(state.ZHand, p, hand)
	}
	setHand(0, hand0)
	setHand(1, hand1)
	place := func(n string, p state.PlayerID) {
		o := e.G.AddObject(lookup(t, reg, n), p)
		o.Zone = state.ZBattlefield
		o.SummonSick = false
		e.G.SetZone(state.ZBattlefield, p, append(e.G.Zone(state.ZBattlefield, p), o.ID))
	}
	for _, n := range bf0 {
		place(n, 0)
	}
	for _, n := range bf1 {
		place(n, 1)
	}
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

// absorbVisCycleOffered reports whether Absorb Vis's own hand activation is
// among seat 0's priority options (the source object's ability option, not
// another card's).
func absorbVisCycleOffered(e *Engine) bool {
	absVis := absorbVisHandObj(e)
	if absVis == 0 {
		return false
	}
	return absorbVisAbilityOffered(e, absVis)
}

func absorbVisHandObj(e *Engine) state.ObjID {
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(id).Face().Name == "Absorb Vis" {
			return id
		}
	}
	return 0
}

func absorbVisAbilityOffered(e *Engine, absVis state.ObjID) bool {
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == absVis {
			return true
		}
	}
	return false
}

// cheapCastOffered reports whether any cast option exists (a cheap
// single-pip spell is the arm's census witness).
func cheapCastOffered(e *Engine) bool {
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" {
			return true
		}
	}
	return false
}

// The full decision-171 state: both seats' boards and hands, three Islands
// with two tapped (the two Mistwalker activations tapped them), Wizard Class,
// pool empty.
func TestAbsorbVisCycleGame171StateOffered(t *testing.T) {
	t.Parallel()
	e := mirrorBoard2(t,
		[]string{"Island", "Island", "Island", "Three Tree City", "Mistwalker", "Wizard Class"},
		[]string{"Swamp", "Mountain", "Mana Flare", "Swamp", "Three Tree City", "Fathom Fleet Swordjack"},
		[]string{"Talrand, Sky Summoner", "Parasitic Impetus", "Archaeomancer", "Savage Gorger", "Citadel Gate", "Absorb Vis", "Swamp"},
		[]string{"Swamp", "Mountain", "Mana Flare", "Swamp"})
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	tapped := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Face().Name == "Island" && tapped < 2 {
			o.Tapped = true
			tapped++
		}
	}
	if tapped != 2 {
		t.Fatalf("precondition: %d Islands tapped, want 2", tapped)
	}
	// p1's Three Tree City: its choice resolved ("Deserter"), it is tapped
	// for the turn? it is summoning-sick only. Tapped for realism.
	p1TTC := -1
	for i, id := range e.G.Zone(state.ZBattlefield, 1) {
		if e.G.Obj(id).Face().Name == "Three Tree City" {
			p1TTC = i
		}
	}
	if p1TTC >= 0 {
		e.G.Obj(e.G.Zone(state.ZBattlefield, 1)[p1TTC]).ChosenType = "Deserter"
	}
	e.G.Step = state.StepUpkeep
	if !absorbVisCycleOffered(e) {
		t.Fatalf("the {1}{B} cycle is payable (fee {2} from the untapped Island's U plus Mana Flare's U, the arm's X=1 Changeling production plus Flare's reflected one pays {1}{B}) but was not offered; actions %v", e.legalActions(0))
	}
}

// The driven twin of decision 171: TTC chosen Aenar, one Changeling
// Mistwalker counts, three Islands of which the two Mistwalker-pump payments
// tapped, Wizard Class, opponent's Mana Flare, pool empty. The {1}{B} cycle
// is payable (the untapped Island's U plus Mana Flare's U funds the arm's
// {2}; the arm's X=1 pick plus Flare's reflected one pays {1}{B}) and the
// offer must exist.
func TestAbsorbVisCycleDrivenPumpsOffered(t *testing.T) {
	t.Parallel()
	e := mirrorBoard2(t,
		[]string{"Three Tree City", "Mistwalker", "Island", "Island", "Island", "Wizard Class"},
		[]string{"Mana Flare"},
		[]string{"Absorb Vis"},
		nil)
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	e.Advance()
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	e.G.Players[0].Pool = state.Mana{}
	pump := func() {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			d = e.Pending()
			if d == nil {
				t.Fatal("no pending decision")
			}
		}
		var opt *decision.Option
		for i := range d.Options {
			if o := &d.Options[i]; o.Kind == "ability" {
				if opt == nil {
					opt = o
				}
			}
		}
		if opt == nil {
			t.Fatalf("no ability option to pump Mistwalker: %+v", d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
			t.Fatal(err)
		}
		for i := 0; ; i++ {
			d := e.Pending()
			if d == nil || i == 20 {
				t.Fatal("the activation did not settle")
			}
			if d.Kind == decision.KPriority {
				break
			}
			t.Fatalf("unexpected %v ask at %q: %+v", d.Kind, d.Prompt, d.Options)
		}
	}
	pump()
	pump()
	ttcTapped := e.G.Obj(ttcO).Tapped
	poolC := e.G.Players[0].Pool[5]
	if ttcTapped || poolC != 0 {
		t.Fatalf("precondition: after the two pumps TTC tapped=%v pool C=%d, want untapped, 0", ttcTapped, poolC)
	}
	offs := e.legalActions(0)
	if !absorbVisCycleOffered(e) {
		t.Fatalf("the {1}{B} cycle is payable but was not offered; actions %v", offs)
	}
	for _, o := range offs {
		if o.Kind == "ability" && strings.Contains(o.Label, "Absorb Vis") {
			t.Fatalf("offer option: %+v", o)
		}
	}
}

const debugBolt = "Name:Test Bolt\nManaCost:1 B\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | TgtPrompt$ Select target creature | NumDmg$ 1 | SpellDescription$ Test Bolt deals 1 damage.\nOracle:x\n"

func TestAbsorbVisCycleWithFlareOffered(t *testing.T) {
	t.Parallel()
	// p0: TTC (chosen "Aenar", one Changeling Mistwalker counts), Island;
	// p1: Mana Flare. The arm's tap makes B + Flare's reflected B: {1}{B}.
	e := mirrorBoard(t, "Absorb Vis",
		[]string{"Three Tree City", "Mistwalker", "Island"},
		[]string{"Mana Flare"})
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	if !absorbVisCycleOffered(e) {
		t.Fatal("the {1}{B} cycle is payable via Mana Flare's reflected any-colour unit but was not offered")
	}
}

func TestAbsorbVisCycleWithOwnFlareOffered(t *testing.T) {
	t.Parallel()
	e := mirrorBoard(t, "Absorb Vis",
		[]string{"Three Tree City", "Mistwalker", "Island", "Mana Flare"},
		nil)
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	if !absorbVisCycleOffered(e) {
		t.Fatal("the {1}{B} cycle is payable via own Mana Flare but was not offered")
	}
}

func TestAbsorbVisCycleWithoutFlareRefused(t *testing.T) {
	t.Parallel()
	// No black mana exists: TTC's {T} makes {C}, the Island {U}, and with no
	// Aenar in play the arm is never activated (its yield is 0). The cycle
	// must not be offered.
	e := mirrorBoard(t, "Absorb Vis",
		[]string{"Three Tree City", "Island"},
		nil)
	if absorbVisCycleOffered(e) {
		t.Fatal("no black mana exists (TTC makes {C}, the Island {U}, the arm yields nothing) yet the {1}{B} cycle was offered")
	}
}

// The arm's {2} fee paid from three Islands, its chosen colour plus Mana
// Flare's extra on its tap paying the {1}{B}.
func TestAbsorbVisCycleThreeIslandsFlareOffered(t *testing.T) {
	t.Parallel()
	e := mirrorBoard(t, "Absorb Vis",
		[]string{"Three Tree City", "Mistwalker", "Island", "Island", "Island", "Mana Flare"},
		nil)
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	if !absorbVisCycleOffered(e) {
		t.Fatal("the {1}{B} cycle is payable (fee {2} from three Islands, the arm's chosen colour plus Mana Flare's extra on its tap) but was not offered")
	}
}

// Does the offer walk price TTC's {2},{T} arm for a CAST of a cheap {1}{B}
// spell in the same board?
func TestCheapBlackCastWithTTCArmOffered(t *testing.T) {
	t.Parallel()
	e := mirrorBoard(t, debugBolt,
		[]string{"Three Tree City", "Mistwalker", "Island", "Island", "Island", "Mana Flare"},
		nil)
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	if !cheapCastOffered(e) {
		t.Fatal("the {1}{B} cast is payable in the same board but was not offered")
	}
}

// Two Mistwalkers: TTC's arm yields 2, no longer under its {2} fee.
func TestCheapBlackCastWithTTCArmTwoCreaturesOffered(t *testing.T) {
	t.Parallel()
	e := mirrorBoard(t, debugBolt,
		[]string{"Three Tree City", "Mistwalker", "Mistwalker", "Island", "Island", "Mana Flare"},
		nil)
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	if !cheapCastOffered(e) {
		t.Fatal("the {1}{B} cast is payable (arm 2 units >= its {2} fee) but was not offered")
	}
}

// The exact divergence state: TTC and every Island TAPPED, {B}{B} floating
// (the arm's activation plus Mana Flare's extra), Wizard Class and p1's Mana
// Flare on the board.
func TestAbsorbVisCycleFloatingBlackOffered(t *testing.T) {
	t.Parallel()
	e := mirrorBoard(t, "Absorb Vis",
		[]string{"Three Tree City", "Mistwalker", "Island", "Island", "Island", "Wizard Class"},
		[]string{"Mana Flare"})
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Face().Name != "Mistwalker" && o.Face().Name != "Wizard Class" {
			o.Tapped = true
		}
	}
	e.G.Players[0].Pool[state.MB] = 2
	if !absorbVisCycleOffered(e) {
		t.Fatalf("the floating {B}{B} pays the {1}{B} cycle outright but it was not offered; actions %v", e.legalActions(0))
	}
}

// The same state through the driven game flow: real Advance, real priority
// decision, permanents moved by logged MoveZone events.
func TestAbsorbVisCycleFloatingBlackDriven(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	names := []string{"Three Tree City", "Mistwalker", "Island", "Island", "Island", "Wizard Class", "Mana Flare", "Absorb Vis"}
	var deck []*cards.Card
	for _, n := range names {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), append([]*cards.Card{lookup(t, reg, "Mana Flare")}, mountainDeck(t, 39)...)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	// seat 0's board
	for _, n := range []string{"Three Tree City", "Mistwalker", "Island", "Island", "Island", "Wizard Class"} {
		id := findAndMoveToHand(t, e, 0, n)
		moveToBattlefield(t, e, id)
	}
	// seat 1's Mana Flare
	moveToBattlefield(t, e, findAndMoveToHand(t, e, 1, "Mana Flare"))
	abs := findAndMoveToHand(t, e, 0, "Absorb Vis")
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		o.SummonSick = false
		switch o.Face().Name {
		case "Three Tree City":
			o.ChosenType = "Aenar"
		case "Island":
			o.Tapped = true
		}
	}
	e.G.Players[0].Pool[state.MB] = 2
	if !absorbVisCycleOffered(e) {
		t.Fatalf("driven board with floating {B}{B}: the {1}{B} cycle was not offered; actions %v", e.legalActions(0))
	}
	_ = abs
}
