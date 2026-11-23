package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
)

// A convoke cast whose picks leave a cost the pool and the CR 601.2g window
// can no longer pay must not be offered a target: gorge's
// affordableTargetCandidates withholds every candidate (its census skips the
// committed picks) and a required target reverses the cast (CR 601.2c,
// 601.2h, "cast aborted: no legal target"). The kernel's
// convokeTargetAbort -> targetWindowShort hid the picks from s.occ, but the
// CopyFrom(saved) between the battlefield-order walk and the pool-pips-first
// path restored s.occ with the picks back in, so windowPoolPipsFirst tapped a
// committed convoke creature (Firdoch Core) a second time and read the cast
// payable: the spell stayed on the stack where gorge had reversed it.
// targetWindowShort now re-applies the pick hide before the second path, so
// both engines reverse the cast (dirac.DebugConvokeAbortCensus restores the
// old behaviour for the mutation test).
//
// TestConvokeCastAbortUnpayableReprosLockstep replays the recorded game,
// TestConvokeCastAbortUnpayableFourSeat plays four-seat pods and requires the
// reversal to be reached, and TestConvokeCastAbortUnpayableMutation turns the
// re-hide off and requires the recording to diverge again.

// TestConvokeCastAbortUnpayableReprosLockstep replays the recorded game:
// after three convoke picks leave {W} unpaid with every land tapped, gorge
// reverses the cast and the kernel kept it on the stack (divergence at
// decision 606: projection Cards[40].Zone).
func TestConvokeCastAbortUnpayableReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "convoke_cast_abort_unpayable")
}

// convokeAbortDeck is a WG convoke pod deck: few lands, the convoke spells,
// and their convoke-able creatures (Firdoch Core is the changeling whose any
// colour ability the old behaviour tapped a second time). Every creature
// convokes, so a cast announced with too few picks is short and reversed.
func convokeAbortDeck() []string {
	return deckOf(4, "Appeal to Eirdu",
		deckOf(4, "Protective Response",
			deckOf(4, "Firdoch Core",
				deckOf(4, "Virulent Emissary",
					deckOf(4, "Moonlit Lamenter",
						deckOf(4, "Blossoming Defense",
							deckOf(4, "Reluctant Dounguard",
								deckOf(4, "Plains",
									deckOf(4, "Forest")...)...)...)...)...)...)...)...)
}

// TestConvokeCastAbortUnpayableFourSeat plays 20 four-seat pods on the std-ecl
// set and requires no divergence. A finish hook counts the convoke casts gorge
// reversed, so the no-divergence check cannot pass without the mechanism being
// reached.
func TestConvokeCastAbortUnpayableFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	var reversed int
	finish := func(g *GorgeSide) {
		var pushed [4]string // each seat's last pushed spell's name, in event order
		for _, ev := range g.E.L.Events {
			switch ev.Kind {
			case events.PutOnStack:
				if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && convokeAbortCard(ob.Face().Name) {
					pushed[ev.Player] = ob.Face().Name
				}
			case events.SpellCastReversed:
				if pushed[ev.Player] != "" {
					reversed++
				}
			}
		}
	}
	deck := convokeAbortDeck()
	set := printedDelveSet(t, [2][]string{deck, deck})
	play := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck},
		nil, func(*GorgeSide, Side) *Divergence { return nil }, finish,
		func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 20; seed++ {
		if res := play(seed); res.Div != nil {
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
	}
	if reversed == 0 {
		t.Fatal("no convoke cast was reversed in 20 pods: the mechanism was never reached")
	}
	t.Logf("%d convoke casts reversed across 20 pods", reversed)
}

// convokeAbortCard reports whether name is a convoke spell of the pod decks.
func convokeAbortCard(name string) bool {
	return name == "Appeal to Eirdu" || name == "Protective Response"
}

// TestConvokeCastAbortUnpayableMutation replays the recording with the pick
// re-hide before the pool-pips-first path off: the committed convoke creature
// then pays the cost a second time and the cast stays payable, so the
// recording diverges again (the spell stays on the kernel's stack).
func TestConvokeCastAbortUnpayableMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "convoke_cast_abort_unpayable.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugConvokeAbortCensus
	dirac.DebugConvokeAbortCensus = true
	defer func() { dirac.DebugConvokeAbortCensus = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("mutation: the replay with the convoke pick re-hide off did not diverge")
	}
	if !strings.Contains(res.Div.Field, "Cards[") {
		t.Fatalf("mutation: diverged for another reason at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	t.Logf("mutation: diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}
