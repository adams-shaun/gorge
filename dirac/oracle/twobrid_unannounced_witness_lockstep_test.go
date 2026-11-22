package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// A twobrid cost's pips are announced before payment (CR 107.4e, 601.2b), but
// two pre-announcement callers price the card while it still carries the
// HybTwobrid marker: the granted-offer search (grantSearchRun -> leafPays ->
// witnessEats -> xWitnessEats -> tapForPK, Flame Javelin beside a Gold token)
// and the lowered plain hand cast (beginCast -> lowerWitnessEx -> tapForPK,
// Magmablood Archaic). tapForPK used to hand the marker to the hybrid witness,
// which panics by design. tapForPK now resolves a face the way the cast
// planner's planTwobridCost does (colour first, then two generic, chosen by
// an actual plan) and returns a clean miss when no face plans; the panic still
// guards an announced cost that reaches the witness (a real invariant breach).
//
// TestTwobridUnannouncedWitnessReprosLockstep replays the two recorded
// games, TestTwobridUnannouncedWitnessFourSeat plays four-seat pods and
// requires the guard to be reached, and TestTwobridUnannouncedWitnessMutation
// turns the guard off and requires the recording to panic again.
func TestTwobridUnannouncedWitnessReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "twobrid_unannounced_witness")
	stdReproNamed(t, "twobrid_unannounced_witness_b")
}

// twobridPodDeck holds every twobrid card at the two pre-announcement paths'
// pieces: Curse of Opulence makes Gold (the sacrifice-only source the granted
// offer search prices) and Gift of Paradise grants a land a mana ability (the
// census gap the lowering/prefix search prices). The dr-mbc-bbbb pool holds
// only Flame Javelin of the three twobrid cards, so the pod builds the
// table its own decks need, as reproSet does for a custom pool.
func twobridPodDeck() []string {
	return deckOf(12, "Flame Javelin",
		deckOf(4, "Curse of Opulence",
			deckOf(4, "Gift of Paradise",
				deckOf(4, "Young Pyromancer",
					deckOf(4, "Mountain",
						deckOf(4, "Forest",
							deckOf(4, "Swamp",
								deckOf(4, "Island")...)...)...)...)...)...)...)
}

// TestTwobridUnannouncedWitnessFourSeat plays 20 four-seat pods and requires
// no divergence. The stats hook fails the test when no twobrid cost was priced
// before its pips were announced, so the no-divergence check cannot pass
// without the guard being reached.
func TestTwobridUnannouncedWitnessFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.TwobridWitnessStats
	old := dirac.DebugTwobridWitnessStats
	dirac.DebugTwobridWitnessStats = &st
	defer func() { dirac.DebugTwobridWitnessStats = old }()
	deck := twobridPodDeck()
	set := printedDelveSet(t, [2][]string{deck, deck})
	play := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck},
		nil, func(*GorgeSide, Side) *Divergence { return nil }, nil,
		func(seed uint64) Policy { return combatPolicy(seed, nil) })
	fourSeatAtkClean(t, play, 20)
	if st.Priced == 0 {
		t.Fatalf("no twobrid cost was priced before its pips were announced in 20 pods: the guard was never reached")
	}
	t.Logf("%d unannounced twobrid costs priced, %d resolved to a face", st.Priced, st.Resolved)
}

// TestTwobridUnannouncedWitnessMutation replays the lowered-hand-cast
// recording (Magmablood Archaic) with the guard off: the unannounced cost
// reaches the hybrid witness and panics again, so the recording's clean replay
// depends on the guard. The granted-offer recording no longer reaches the
// witness at its root: since 1ff6473618 (#3256) grantSrch.witnessEats returns
// before xWitnessEats when the plain census pays the cast outright.
func TestTwobridUnannouncedWitnessMutation(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "twobrid_unannounced_witness_b.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugTwobridNoWitnessGuard
	dirac.DebugTwobridNoWitnessGuard = true
	defer func() { dirac.DebugTwobridNoWitnessGuard = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatalf("mutation: the replay with the twobrid witness guard off did not diverge")
	}
	if !strings.Contains(res.Div.Detail, "a twobrid cost reached the hybrid witness") {
		t.Fatalf("mutation: diverged for another reason at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	t.Logf("mutation: diverged at %d with the expected witness panic", res.Div.At)
}
