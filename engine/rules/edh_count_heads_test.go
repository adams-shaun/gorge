package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// castOption finds the cast option for id in d's option list.
func edhCastIdx(d *decision.Decision, id state.ObjID) int {
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	return idx
}

// passPriority submits the pass option of the pending priority decision.
func edhPassPriority(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "pass" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("priority decision with no pass option: %+v", d)
	}
	submitChoices(t, e, idx)
}

// discardForestIdx finds the discard option for a Forest in the pending
// hand-size ask (a discard lands in the graveyard and reads nothing).
func discardForestIdx(d *decision.Decision) int {
	for _, o := range d.Options {
		if o.Kind == "discard" && o.Label == "Discard Forest" {
			return o.Index
		}
	}
	return -1
}

// drainPool empties player 0's pool with DB$ DrainMana's shape, so the
// post-Seal affordability check starts from an exact pool.
func drainPool(t *testing.T, e *Engine) {
	t.Helper()
	e.emit(events.Event{Kind: events.ManaClear, Player: 0, Text: events.ManaDrainText})
	e.priorityRound()
}

// TestEDHCountHeadsBSealReduce drives the real Seal of the Guildpact: cast
// it, answer its two-colour as-enters ask (white, then blue), and read the
// Relative$ ReduceCost static's Count$HasNumChosenColors.OriginalHost
// through the payment machinery with an EXACT pool. The count is the priced
// spell's own: Vodalian Wave-Knight ({2}{W}{U}) shares both chosen colours,
// so its two generic are erased and it is castable from exactly {W}{U} --
// a reduction of only one would leave a generic that the pool cannot pay,
// because both its units are needed for the pips. Knight of the Keep
// ({2}{W}) shares one colour and stays castable from the same pool with one
// generic left to pay. Grizzly Bears ({1}{G}) shares neither colour and is
// not castable at all.
func TestEDHCountHeadsBSealReduce(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t,
		"s/seal_of_the_guildpact.txt", "v/vodalian_wave_knight.txt",
		"k/knight_of_the_keep.txt", "g/grizzly_bears.txt", "f/forest.txt")
	e, _ := etbreplEngine(t, reg,
		"Vodalian Wave-Knight", "Knight of the Keep", "Seal of the Guildpact", "Grizzly Bears")

	wave := searchMoveByName(t, e, "Vodalian Wave-Knight", state.ZHand)
	if wave == 0 {
		t.Fatal("Vodalian Wave-Knight is not in hand")
	}
	keep := searchMoveByName(t, e, "Knight of the Keep", state.ZHand)
	if keep == 0 {
		t.Fatal("Knight of the Keep is not in hand")
	}

	// Precondition: with an empty pool and no Seal, neither spell is offered.
	d := e.Pending()
	if d == nil {
		t.Fatal("no priority decision on Main1")
	}
	if edhCastIdx(d, wave) >= 0 || edhCastIdx(d, keep) >= 0 {
		t.Fatalf("precondition: a spell is offered from an empty pool: %+v", d.Options)
	}

	seal := searchMoveByName(t, e, "Seal of the Guildpact", state.ZHand)
	if seal == 0 {
		t.Fatal("Seal of the Guildpact is not in hand")
	}
	addMana(t, e, 0, "WWWWW")
	d = e.Pending()
	idx := edhCastIdx(d, seal)
	if idx < 0 {
		t.Fatalf("no cast option for Seal of the Guildpact: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	// The as-enters ask spans two ordinals: the second pick must not offer
	// the colour the first one recorded.
	want := []string{"White", "Blue"}
	pick := 0
	for i := 0; i < 60 && pick < 2; i++ {
		d = e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch {
		case d.Kind == decision.KChoose && d.ResumeKind == "etb":
			li := -1
			for _, o := range d.Options {
				if o.Kind == "color" && o.Label == want[pick] {
					li = o.Index
				}
				if pick == 1 && o.Kind == "color" && o.Label == "White" {
					t.Fatal("the second pick still offers the first pick's colour")
				}
			}
			if li < 0 {
				t.Fatalf("no %s option: %+v", want[pick], d.Options)
			}
			submitChoices(t, e, li)
			pick++
		case d.Kind == decision.KPriority:
			edhPassPriority(t, e)
		case d.Kind == decision.KChoose && discardForestIdx(d) >= 0:
			picks := make([]int, 0, d.Max)
			for _, o := range d.Options {
				if o.Kind == "discard" && o.Label == "Discard Forest" && len(picks) < d.Max {
					picks = append(picks, o.Index)
				}
			}
			submitChoices(t, e, picks...)
		default:
			t.Fatalf("unexpected %s decision while Seal resolves: %+v", d.Kind, d)
		}
	}
	if pick != 2 {
		t.Fatalf("the two-colour ask posed %d picks, want 2", pick)
	}
	passUntilStackEmpty(t, e, 20)
	sealOnBf := findBattlefieldCard(t, e, "Seal of the Guildpact")
	if got := e.G.Obj(sealOnBf).ChosenColors; got != "WU" {
		t.Fatalf("Seal's recorded choice = %q, want WU", got)
	}

	// From exactly {W}{U}: the two-shared spell's generic is erased, the
	// one-shared spell still has a generic the pool can pay, and the
	// unshared spell is not castable at all.
	drainPool(t, e)
	addMana(t, e, 0, "WU")
	if p := e.G.Players[0].Pool; p.Total() != 2 || p[state.MW] != 1 || p[state.MU] != 1 {
		t.Fatalf("precondition: pool = %v, want exactly one W and one U", p)
	}
	d = e.Pending()
	if d == nil {
		t.Fatal("no decision pending after Seal resolved")
	}
	// The other two spells are judged from the same untouched pool, before
	// the Wave-Knight cast spends it.
	if edhCastIdx(d, keep) < 0 {
		t.Fatalf("the one-shared spell is not castable from exactly {W}{U}: %+v", d.Options)
	}
	bears := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	if bears != 0 && edhCastIdx(d, bears) >= 0 {
		t.Fatal("the unshared {1}{G} spell is castable from exactly {W}{U}")
	}
	idx = edhCastIdx(d, wave)
	if idx < 0 {
		t.Fatalf("the two-shared spell is not castable from exactly {W}{U} (the reduction did not erase both generic): %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(wave).Zone; got != state.ZBattlefield {
		t.Fatalf("Vodalian Wave-Knight zone = %v, want battlefield (cast from exactly {W}{U})", got)
	}
}

// TestEDHCountHeadsBResetGate drives Reset's printed cast gate: NOT offered
// on your own turn, offered on the opponent's turn after their upkeep step
// has finished. The gate is the SA's own CheckSVar$ against
// Count$FinishedUpkeepsThisTurn, so the offer machinery itself is the read.
func TestEDHCountHeadsBResetGate(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t, "r/reset.txt", "g/grizzly_bears.txt", "f/forest.txt")
	e, _ := etbreplEngine(t, reg, "Reset")
	reset := searchMoveByName(t, e, "Reset", state.ZHand)
	if reset == 0 {
		t.Fatal("Reset is not in hand")
	}

	// Your own turn, Main1: the gate reads "an opponent's turn" as false.
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision on your own Main1, got %+v", d)
	}
	if idx := edhCastIdx(d, reset); idx >= 0 {
		t.Fatalf("Reset is castable on your own turn: %+v", d.Options)
	}

	// The opponent's turn, DURING their upkeep step: not offered. The count
	// is the upkeeps already finished, so it is zero while the step is still
	// in progress (the in-step adjustment).
	driveToStep(t, e, 2, 1, state.StepUpkeep)
	addManaQuiet(t, e, 0, "UU")
	d = e.Pending()
	if d != nil && d.Player != 0 {
		edhPassPriority(t, e)
		d = e.Pending()
	}
	if d == nil || d.Player != 0 || e.G.Step != state.StepUpkeep {
		t.Fatalf("precondition: want seat 0's priority in the opponent's upkeep, got step %v decision %+v", e.G.Step, d)
	}
	if idx := edhCastIdx(d, reset); idx >= 0 {
		t.Fatalf("Reset is castable during the opponent's upkeep step: %+v", d.Options)
	}

	// The opponent's turn, Main1 (their upkeep finished): offered.
	driveToStep(t, e, 2, 1, state.StepMain1)
	addMana(t, e, 0, "UU")
	edhPassPriority(t, e)
	d = e.Pending()
	if d == nil {
		t.Fatal("no decision pending on the opponent's turn")
	}
	idx := edhCastIdx(d, reset)
	if idx < 0 {
		t.Fatalf("Reset is not castable on the opponent's turn after their upkeep step: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(reset).Zone; got != state.ZGraveyard {
		t.Fatalf("Reset zone = %v, want graveyard after resolving", got)
	}
}

// TestEDHCountHeadsBSageHands drives the real Werewolf of Ancient Hunger
// back face: put Sage of Ancient Lore on the battlefield, let the upkeep
// transform trigger flip it (no spells were cast last turn), and read its
// CR-derived power and toughness, which the back face's CDA sets to
// Count$NumInAllHands -- the total size of every player's hand.
func TestEDHCountHeadsBSageHands(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t,
		"s/sage_of_ancient_lore_werewolf_of_ancient_hunger.txt", "g/grizzly_bears.txt", "f/forest.txt")
	e, _ := etbreplEngine(t, reg, "Sage of Ancient Lore")
	sage := findAndMoveToHand(t, e, 0, "Sage of Ancient Lore")
	moveToBattlefield(t, e, sage)
	e.pending = nil
	e.Advance()

	// Turn 2's upkeep: no spells were cast last turn, so the front face's
	// transform trigger fires.
	driveToStep(t, e, 2, 1, state.StepUpkeep)
	for i := 0; i < 40; i++ {
		if e.G.Obj(sage).Face().Name == "Werewolf of Ancient Hunger" {
			break
		}
		d := e.Pending()
		if d == nil {
			break
		}
		switch {
		case d.Kind == decision.KPriority:
			edhPassPriority(t, e)
		case d.Kind == decision.KChoose && discardForestIdx(d) >= 0:
			picks := make([]int, 0, d.Max)
			for _, o := range d.Options {
				if o.Kind == "discard" && o.Label == "Discard Forest" && len(picks) < d.Max {
					picks = append(picks, o.Index)
				}
			}
			submitChoices(t, e, picks...)
		default:
			t.Fatalf("unexpected %s decision during the upkeep: %+v", d.Kind, d)
		}
	}
	if got := e.G.Obj(sage).Face().Name; got != "Werewolf of Ancient Hunger" {
		t.Fatalf("precondition: face = %q, want the transformed back face (no spells cast last turn)", got)
	}

	total := int32(len(e.G.Zone(state.ZHand, 0)) + len(e.G.Zone(state.ZHand, 1)))
	if total == 0 {
		t.Fatal("precondition: every hand is empty, the read has nothing to count")
	}
	pt := e.Derived(sage)
	if pt.Power != total || pt.Toughness != total {
		t.Fatalf("Werewolf of Ancient Hunger P/T = %d/%d, want %d/%d (cards in all hands)",
			pt.Power, pt.Toughness, total, total)
	}
}

// fblthpDrawn moves the named Fblthp (genesis-dealt, so in hand or library)
// onto seat 0's battlefield from the requested origin zone and returns how
// many cards the "When Fblthp enters" trigger drew: the hand size after the
// trigger resolves, plus the one Fblthp left the hand by, minus the size
// before the move.
func fblthpDrawn(t *testing.T, e *Engine, from state.Zone) int {
	t.Helper()
	id := findAndMoveToHand(t, e, 0, "Fblthp, the Lost")
	if from == state.ZLibrary {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	if got := e.G.Obj(id).Zone; got != from {
		t.Fatalf("precondition: Fblthp zone = %v, want %v", got, from)
	}
	before := len(e.G.Zone(state.ZHand, 0))
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: state.ZBattlefield})
	e.pending = nil
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("Fblthp zone = %v, want battlefield", got)
	}
	after := len(e.G.Zone(state.ZHand, 0))
	if from == state.ZHand {
		return after - before + 1
	}
	return after - before
}

// TestEDHCountHeadsBFblthpEntersFromLibrary drives Fblthp's draw: put onto
// the battlefield from the LIBRARY it draws two cards ("If it enters from
// your library ... draw two cards instead"), put onto it from the hand it
// draws one. The ETB trigger's referent is live after the entry, so the
// library origin is the one the triggering event recorded.
func TestEDHCountHeadsBFblthpEntersFromLibrary(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t, "f/fblthp_the_lost.txt", "g/grizzly_bears.txt", "f/forest.txt")
	eLib, _ := etbreplEngine(t, reg, "Fblthp, the Lost")
	if got := fblthpDrawn(t, eLib, state.ZLibrary); got != 2 {
		t.Errorf("Fblthp entering from the library drew %d cards, want 2", got)
	}
	eHand, _ := etbreplEngine(t, reg, "Fblthp, the Lost")
	if got := fblthpDrawn(t, eHand, state.ZHand); got != 1 {
		t.Errorf("Fblthp entering from the hand drew %d cards, want 1", got)
	}
}

// TestEDHCountHeadsBFblthpHardCast hard-casts Fblthp from the hand: it was
// cast from the hand and entered from the stack, so it draws one card.
func TestEDHCountHeadsBFblthpHardCast(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t, "f/fblthp_the_lost.txt", "g/grizzly_bears.txt", "f/forest.txt")
	e, _ := etbreplEngine(t, reg, "Fblthp, the Lost")
	id := findAndMoveToHand(t, e, 0, "Fblthp, the Lost")
	e.pending = nil
	e.priorityRound()
	addMana(t, e, 0, "1U")
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := edhCastIdx(d, id)
	if idx < 0 {
		t.Fatalf("Fblthp is not castable: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	handAfterCast := len(e.G.Zone(state.ZHand, 0))
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("Fblthp zone = %v, want battlefield", got)
	}
	if drew := len(e.G.Zone(state.ZHand, 0)) - handAfterCast; drew != 1 {
		t.Errorf("a hard-cast Fblthp drew %d cards, want 1", drew)
	}
}

// TestEDHCountHeadsBEmbodimentSkipsNoCost casts the real Embodiment of
// Agonies with a {1}{G} card, a second {1}{G} card and Living End ("no
// cost") in the graveyard: two distinct costs ({1}{G} twice counts once),
// so it enters with two +1/+1 counters, not three.
func TestEDHCountHeadsBEmbodimentSkipsNoCost(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t, "e/embodiment_of_agonies.txt", "l/living_end.txt",
		"g/grizzly_bears.txt", "r/runeclaw_bear.txt", "f/forest.txt")
	e, _ := etbreplEngine(t, reg, "Embodiment of Agonies", "Living End", "Grizzly Bears", "Runeclaw Bear")
	for _, name := range []string{"Living End", "Grizzly Bears", "Runeclaw Bear"} {
		searchMoveByName(t, e, name, state.ZGraveyard)
	}
	// Precondition: the no-cost card really is in the graveyard with that cost.
	var noCost bool
	for _, id := range e.G.Zone(state.ZGraveyard, 0) {
		if o := e.G.Obj(id); o.Face().Name == "Living End" {
			noCost = o.Face().ManaCost == "no cost"
		}
	}
	if !noCost {
		t.Fatal("precondition: Living End is not in the graveyard with ManaCost 'no cost'")
	}
	emb := searchMoveByName(t, e, "Embodiment of Agonies", state.ZHand)
	addMana(t, e, 0, "1BB")
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := edhCastIdx(d, emb)
	if idx < 0 {
		t.Fatalf("Embodiment of Agonies is not castable: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(emb).Zone; got != state.ZBattlefield {
		t.Fatalf("Embodiment zone = %v, want battlefield", got)
	}
	if got := e.G.Obj(emb).Counter("P1P1"); got != 1 {
		t.Errorf("Embodiment entered with %d +1/+1 counters, want 1 (the two {1}{G} cards are one cost; Living End's no-cost face is not a cost)", got)
	}
}

// TestEDHCountHeadsBAdiposeEmergeTokens casts the real Adipose Offspring for
// its Emerge cost by sacrificing a 0/4: it creates X Alien tokens where X is
// the sacrificed creature's TOUGHNESS (4), not the single token of an
// ordinary cast.
func TestEDHCountHeadsBAdiposeEmergeTokens(t *testing.T) {
	t.Parallel()
	const wall = "Name:Emerge Wall\nManaCost:2 G\nTypes:Creature Wall\nPT:0/4\nOracle:x\n"
	e, _, _ := altCostEngine(t, 4250, []string{"Adipose Offspring"}, []string{wall}, nil)
	adipose := findCardObj(t, e, 0, "Adipose Offspring", state.ZHand)
	fuel := findCardObj(t, e, 0, "Emerge Wall", state.ZBattlefield)
	if tough := e.Derived(fuel).Toughness; tough != 4 {
		t.Fatalf("precondition: sacrificed creature toughness = %d, want 4", tough)
	}
	aliens := func() int {
		n := 0
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Alien Token" {
				n++
			}
		}
		return n
	}
	if aliens() != 0 {
		t.Fatal("precondition: an Alien token is already on the battlefield")
	}
	// {5}{W} reduced by the wall's mana value 3 is {2}{W}.
	addMana(t, e, 0, "WCC")
	castEmergeSacrificing(t, e, adipose, fuel)
	if got := aliens(); got != 4 {
		t.Errorf("emerge-cast Adipose Offspring made %d Alien tokens, want 4 (the sacrificed creature's toughness)", got)
	}
}
