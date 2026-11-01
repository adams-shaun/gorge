package rules

// TestNestedDiscardDoesNotInheritOuterAnswer on the REAL corpus Gruesome
// Discovery: its TgtChoose Discard chains a RevealYouChoose Discard
// (MorbidDiscard), and the two legs are mutually exclusive on the Morbid
// gate. The inner discard must never inherit the outer's answer: each leg,
// when it runs, poses its own ask, and a gated-off leg stays silent.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestKernelNestedDiscardDoesNotInheritOuterAnswer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	gd, ok := reg.Lookup("Gruesome Discovery")
	if !ok {
		t.Fatal("corpus has no Gruesome Discovery")
	}
	for i, morbid := range []bool{false, true} {
		morbid := morbid
		seed := uint64(240 + i)
		name := "not morbid: outer asks the discarding player, inner gated off"
		if morbid {
			name = "morbid: outer gated off, inner asks the caster fresh"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hand := cardsFromSrcs(t, creatureFixtureSrcs("Frog", "Bird", "Cat", "Dog"))
			e, cfg, id := buildTwoSeat(t, seed, gd, cardsFromSrcs(t, []string{creatureFixtureSrc("Ox")}), hand)
			addMana(t, e, 0, "BBBB")
			clearHandExcept(t, e, 1, 0)
			var ids []state.ObjID
			for _, n := range []string{"Frog", "Bird", "Cat", "Dog"} {
				ids = append(ids, putNamed(t, e, 1, n, state.ZHand))
			}
			if morbid {
				ox := putNamed(t, e, 0, "Ox", state.ZBattlefield)
				e.emit(events.Event{Kind: events.MoveZone, Obj: ox, From: state.ZBattlefield, To: state.ZGraveyard, Text: "died"})
			}
			d := castTargeting(t, e, id, true, 1, 0)
			if d == nil {
				t.Fatal("the resolution posed no discard ask")
			}
			if !morbid {
				if d.Player != 1 {
					t.Fatalf("outer discard chooser = seat %d, want the discarding seat 1", d.Player)
				}
				if p := pickChoices(t, e, optionIdxOfObj(t, d, ids[0]), optionIdxOfObj(t, d, ids[1])); p != nil {
					t.Fatalf("a second ask after the outer choice (the gated-off inner ran or inherited): %+v", p)
				}
				for i, c := range ids {
					if inGy := zoneOfID(e, c) == state.ZGraveyard; inGy != (i < 2) {
						t.Fatalf("hand card %d in graveyard = %v, want %v", i, inGy, i < 2)
					}
				}
				replayCheck(t, e, cfg)
				return
			}
			if d.Player != 0 {
				t.Fatalf("inner discard chooser = seat %d, want the caster seat 0", d.Player)
			}
			if d.Kind != decision.KModes || d.Min != 2 || d.Max != 2 || len(d.Options) != 4 {
				t.Fatalf("inner discard decision = %+v, want a Min==Max==2 KModes over the whole four-card hand", d)
			}
			if p := pickChoices(t, e, optionIdxOfObj(t, d, ids[2]), optionIdxOfObj(t, d, ids[3])); p != nil {
				t.Fatalf("unexpected further ask %+v", p)
			}
			for i, c := range ids {
				if inGy := zoneOfID(e, c) == state.ZGraveyard; inGy != (i >= 2) {
					t.Fatalf("hand card %d in graveyard = %v, want %v", i, inGy, i >= 2)
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}
