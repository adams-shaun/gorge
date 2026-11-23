package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// A sacrifice-only mana source (a Gold token's "Sacrifice this artifact: Add
// one mana of any color", an Eldrazi Spawn token's {C}) is a mana ability the
// payment window can use (CR 605.1a, 605.3a), so an {X} spell's witness is
// priced with it: gorge's potential planner announced Lunar Frenzy's largest X
// counting the Gold token's unit (8 mana, X = 7) and its payment sacrificed
// the token at activation, while dirac's {X} witness census (availUngranted)
// admitted only tap-cost sources and priced X = 6 over seven sources, so the
// token stayed on the battlefield and the floating pool diverged. The census
// now admits a sacrifice-only source beside the tap ones, tapped or untapped
// alike (CR 602.2b), as gorge's census does and availForMode's already does.
//
// TestSacManaSourcePaymentReprosLockstep replays the two recorded games,
// TestSacManaSourcePaymentFourSeat plays four-seat pods over the same
// mechanism and requires a payment to sacrifice a mana token, and
// TestSacManaSourcePaymentMutation restores the pre-fix census and requires
// the recording to diverge again.
func TestSacManaSourcePaymentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_mana_source_payment")
	stdReproNamed(t, "sac_mana_source_payment_b")
}

// sacManaPodDeck holds the payment's pieces: Curse of Opulence makes Gold (a
// sacrifice-only source), Brood Monitor makes Eldrazi Spawn, Treasure Dredger
// makes a Treasure, Lotus Petal sacrifices itself, Lunar Frenzy is the {X}
// spell whose witness the census prices, and Hero's Blade is the equipment
// whose activation a tapped Spawn can fund.
func sacManaPodDeck() []string {
	return deckOf(4, "Lunar Frenzy",
		deckOf(4, "Curse of Opulence",
			deckOf(4, "Brood Monitor",
				deckOf(4, "Treasure Dredger",
					deckOf(4, "Lotus Petal",
						deckOf(4, "Hero's Blade",
							deckOf(6, "Mountain",
								deckOf(6, "Swamp",
									deckOf(6, "Forest",
										deckOf(6, "Island")...)...)...)...)...)...)...)...)...)
}

// TestSacManaSourcePaymentFourSeat plays 20 four-seat pods and requires no
// divergence. The stats hook fails the test when no payment's plan sacrificed
// a mana source, so the no-divergence check cannot pass empty.
func TestSacManaSourcePaymentFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.SacManaPaymentStats
	old := dirac.DebugSacManaPaymentStats
	dirac.DebugSacManaPaymentStats = &st
	defer func() { dirac.DebugSacManaPaymentStats = old }()
	deck := sacManaPodDeck()
	set := printedDelveSet(t, [2][]string{deck, deck})
	play := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck},
		nil, func(*GorgeSide, Side) *Divergence { return nil }, nil,
		func(seed uint64) Policy { return combatPolicy(seed, nil) })
	fourSeatAtkClean(t, play, 20)
	if st.Paid == 0 {
		t.Fatalf("no payment's plan sacrificed a mana source in 20 pods: the mechanism was never reached")
	}
	t.Logf("%d payments sacrificed a mana source", st.Paid)
}

// TestSacManaSourcePaymentMutation replays the recording with the witness
// census's sacrifice-only admission off: it prices the {X} one short again and
// the game diverges, so the recording's clean replay depends on the admission.
func TestSacManaSourcePaymentMutation(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "sac_mana_source_payment.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugSacManaAfterPay
	dirac.DebugSacManaAfterPay = true
	defer func() { dirac.DebugSacManaAfterPay = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatalf("mutation: the replay with the witness census's sacrifice-only admission off did not diverge")
	}
	t.Logf("mutation: diverged at %d (%s %s)", res.Div.At, res.Div.What, res.Div.Field)
}
