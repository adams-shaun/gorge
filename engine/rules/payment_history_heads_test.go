package rules

// The payment-history count heads (Count$TotalManaSpent, ManaColorsPaid,
// xColorPaid, Chroma, ChromaInGrave, TriggeredPayingMana,
// TriggeredManaCostDevotion): each reads what a payment actually spent or the
// mana-cost symbols of a named set of cards. Every test drives a real corpus
// card through the real activation / cast / upkeep path.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// activateWithMana funds seat 0 with the given ManaAdd counters (a plain
// colour letter, or a producer-tagged spelling such as "TreasureR"), activates
// the permanent's first ability and resolves the stack.
func activateWithMana(t *testing.T, e *Engine, id state.ObjID, counters ...string) {
	t.Helper()
	toMain1(t, e)
	for _, c := range counters {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: c, Amount: 1})
	}
	e.priorityRound()
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
}

// TestJetmirsFixerReadsTotalManaSpent: "If mana from a Treasure was spent to
// activate this ability, put a +1/+1 counter on it instead" reads
// Count$TotalManaSpent Treasure off the activation's own payment.
func TestJetmirsFixerReadsTotalManaSpent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name         string
		mana         []string
		wantCounters int32
		wantPower    int32
	}{
		{"treasureMana", []string{effects.ManaProducerCounter("Treasure", false, "R"), "G"}, 1, 3},
		{"plainMana", []string{"R", "G"}, 0, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _ := etbreplEngine(t, reg, "Jetmir's Fixer")
			id := searchMoveByName(t, e, "Jetmir's Fixer", state.ZBattlefield)
			activateWithMana(t, e, id, tc.mana...)
			o := e.G.Obj(id)
			if got := o.Counter("P1P1"); got != tc.wantCounters {
				t.Fatalf("P1P1 counters = %d, want %d (mana %v)", got, tc.wantCounters, tc.mana)
			}
			// The pump (+1/+1 until end of turn) and the counter (+1/+1
			// permanently) both leave a 3/3 this turn: the distinction is the
			// counter, asserted above.
			if pt := e.Derived(id); pt.Power != tc.wantPower {
				t.Fatalf("power = %d, want %d", pt.Power, tc.wantPower)
			}
		})
	}
}

// TestParagonOfModernityReadsColorsPaid: exactly three colours of mana spent
// to activate -> a +1/+1 counter; colours count once each.
func TestParagonOfModernityReadsColorsPaid(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name         string
		mana         []string
		wantCounters int32
	}{
		{"threeColours", []string{"R", "G", "U"}, 1},
		{"twoColoursThreeUnits", []string{"R", "R", "G"}, 0},
		{"colourless", []string{"C", "C", "C"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _ := etbreplEngine(t, reg, "Paragon of Modernity")
			id := searchMoveByName(t, e, "Paragon of Modernity", state.ZBattlefield)
			activateWithMana(t, e, id, tc.mana...)
			if got := e.G.Obj(id).Counter("P1P1"); got != tc.wantCounters {
				t.Fatalf("P1P1 counters = %d, want %d (mana %v)", got, tc.wantCounters, tc.mana)
			}
		})
	}
}

// TestPrimalcruxChromaCountsGreenSymbols: Primalcrux's P/T equals the green
// mana symbols among the permanents its controller controls (CR 202.1).
func TestPrimalcruxChromaCountsGreenSymbols(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := etbreplEngine(t, reg, "Primalcrux", "Grizzly Bears")
	id := searchMoveByName(t, e, "Primalcrux", state.ZBattlefield)
	if pt := e.Derived(id); pt.Power != 6 || pt.Toughness != 6 {
		t.Fatalf("alone, P/T = %d/%d, want 6/6 (its own six {G})", pt.Power, pt.Toughness)
	}
	searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	if pt := e.Derived(id); pt.Power != 7 || pt.Toughness != 7 {
		t.Fatalf("with Grizzly Bears, P/T = %d/%d, want 7/7", pt.Power, pt.Toughness)
	}
}

// TestUmbraStalkerChromaInGraveCountsBlackSymbols: the graveyard form.
func TestUmbraStalkerChromaInGraveCountsBlackSymbols(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := etbreplEngine(t, reg, "Umbra Stalker", "Forsworn Paladin")
	id := searchMoveByName(t, e, "Umbra Stalker", state.ZBattlefield)
	if pt := e.Derived(id); pt.Power != 0 {
		t.Fatalf("empty graveyard power = %d, want 0", pt.Power)
	}
	searchMoveByName(t, e, "Forsworn Paladin", state.ZGraveyard)
	if pt := e.Derived(id); pt.Power != 1 || pt.Toughness != 1 {
		t.Fatalf("one {B} in the graveyard: P/T = %d/%d, want 1/1", pt.Power, pt.Toughness)
	}
}

// TestSoulBurnXColorPaid: the life gained is limited to the {B} spent on X.
func TestSoulBurnXColorPaid(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name     string
		mana     string
		x        int32
		wantGain int32
	}{
		// {X}{2}{B} with X=3: all six units black, three of them on X.
		{"allBlack", "BBBBBB", 3, 3},
		// Only the printed {B} is black; none of it was spent on X.
		{"blackOnlyOnPip", "BRRRRR", 3, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _ := etbreplEngine(t, reg, "Soul Burn")
			id := searchMoveByName(t, e, "Soul Burn", state.ZHand)
			e.G.Players[0].Life = 10
			e.G.Players[1].Life = 20
			addMana(t, e, 0, tc.mana)
			opt := castOptionFor(t, e, id)
			submitChoices(t, e, opt.Index)
			answerX(t, e, tc.x)
			// Target the opponent.
			if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
				idx := -1
				for _, o := range d.Options {
					if o.Kind == "player" && o.Player == 1 {
						idx = o.Index
					}
				}
				if idx < 0 {
					t.Fatalf("opponent not offered: %+v", d.Options)
				}
				submitChoices(t, e, idx)
			}
			passUntilStackEmpty(t, e, 30)
			if got := e.G.Players[1].Life; got != 20-tc.x {
				t.Fatalf("opponent life = %d, want %d (X damage)", got, 20-tc.x)
			}
			if got := e.G.Players[0].Life - 10; got != tc.wantGain {
				t.Fatalf("life gained = %d, want %d", got, tc.wantGain)
			}
		})
	}
}

// TestEveryPaymentHistoryHeadCardIsPlayable is the class census: the cards the
// seven heads gated out of the pool are supported now.
func TestEveryPaymentHistoryHeadCardIsPlayable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{
		"Jetmir's Fixer", "Paragon of Modernity", "Forsworn Paladin", "Soul Burn",
		"Balduvian Fallen", "Managorger Phoenix", "Heartlash Cinder", "Outrage Shaman",
		"Primalcrux", "Springjack Shepherd", "Umbra Stalker",
	} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus card %q missing", name)
		}
		if miss := reg.Unsupported(c, sup); len(miss) != 0 {
			t.Errorf("%s is still unsupported: %v", name, miss)
		}
	}
}

// TestBalduvianFallenCountsBlackOrRedPaidForUpkeep: "it gets +1/+0 until end
// of turn for each {B} or {R} spent this way" reads the mana the cumulative
// upkeep payment actually spent.
func TestBalduvianFallenCountsBlackOrRedPaidForUpkeep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name      string
		mana      string
		wantBonus int32
	}{
		{"black", "B", 1},
		{"red", "R", 1},
		{"green", "G", 0},
		{"colourless", "C", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fallen, ok := reg.Lookup("Balduvian Fallen")
			if !ok {
				t.Fatal("corpus fixture: Balduvian Fallen missing")
			}
			// The edh_trigf kernel-probe shape: no turn loop, the upkeep's own
			// resolution is driven directly.
			e := New(Config{Seed: 7, Names: []string{"a", "b"},
				Decks: [][]*cards.Card{append([]*cards.Card{fallen}, mountainDeck(t, 39)...), mountainDeck(t, 40)}})
			e.G.Step = state.StepMain1
			e.G.Active, e.G.Priority = 0, 0
			e.G.Turn = 1
			id := moveByName(t, e, 0, "Balduvian Fallen", state.ZBattlefield)
			base := e.Derived(id).Power
			e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: tc.mana, Amount: 1})
			d := trigFUpkeepAsk(t, e)
			pay := -1
			for _, o := range d.Options {
				if o.Kind == "cumulative_pay" {
					pay = o.Index
				}
			}
			if pay < 0 {
				t.Fatalf("the pay option is not offered: %+v", d.Options)
			}
			submitChoices(t, e, pay)
			if len(e.pendingTriggers) == 0 {
				t.Fatal("the paid upkeep queued no trigger")
			}
			e.putTriggersOnStack()
			for i := 0; i < 10 && len(e.G.Stack) > 0; i++ {
				e.resolveTop()
			}
			if got := e.Derived(id).Power - base; got != tc.wantBonus {
				t.Fatalf("power bonus = %d, want %d (paid %s)", got, tc.wantBonus, tc.mana)
			}
		})
	}
}

// TestChromaEntersHeadsCountMatchingSymbols: Heartlash Cinder's "+X/+0 where X
// is the number of red mana symbols in the mana costs of permanents you
// control" counts its own {R} and Managorger Phoenix's {R}{R}.
func TestChromaEntersHeadsCountMatchingSymbols(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := etbreplEngine(t, reg, "Heartlash Cinder", "Managorger Phoenix")
	searchMoveByName(t, e, "Managorger Phoenix", state.ZBattlefield)
	passUntilStackEmpty(t, e, 5)
	id := searchMoveByName(t, e, "Heartlash Cinder", state.ZBattlefield)
	// A battlefield move is not a cast, but the ETB trigger still fires.
	for i := 0; i < 20 && len(e.G.Stack) == 0; i++ {
		e.putTriggersOnStack()
	}
	passUntilStackEmpty(t, e, 30)
	// 1/1 base; red symbols: Cinder {R} + Phoenix {R}{R} = 3.
	if got := e.Derived(id).Power; got != 4 {
		t.Fatalf("Heartlash Cinder power = %d, want 4 (1 + 3 red symbols)", got)
	}
}

// TestManagorgerPhoenixCountsRedSymbolsOfTheCastSpell: with Phoenix in the
// graveyard, casting a spell puts a flame counter on it for each {R} in that
// spell's mana cost (Count$TriggeredManaCostDevotion.Red).
func TestManagorgerPhoenixCountsRedSymbolsOfTheCastSpell(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := etbreplEngine(t, reg, "Managorger Phoenix", "Heartlash Cinder")
	phoenix := searchMoveByName(t, e, "Managorger Phoenix", state.ZGraveyard)
	castFromHand(t, e, "Heartlash Cinder", "RR")
	if got := e.G.Obj(phoenix).Counter("FLAME"); got != 1 {
		t.Fatalf("flame counters = %d, want 1 (Heartlash Cinder costs {1}{R})", got)
	}
}
