package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// This is the Curator of Destinies ETB pin. Curator's real
// script splits the top five into a pile that leaves through ChangeZone
// (ChosenPile$ DBHand) and a pile that leaves through ChangeZoneAll
// (UnchosenPile$ DBGraveyard, `ChangeType$ Card.IsRemembered | Origin$
// Library`). noShuffleBody forces NoShuffle$ True onto BOTH bodies; ChangeZone
// read it, ChangeZoneAll did not, so the unchosen-pile sweep reported
// "ChangeZoneAll ignores unread parameter(s) NoShuffle$" once per resolution.
// The oracle text never shuffles, so the forced flag is correct and the note
// was false noise. These tests drive both chooser answers through the ask
// seam and pin: the right pile to hand, the other to the graveyard, no
// events.Shuffle, and no unread-parameter Note.

// curatorTapeHost serves the two mid-resolution asks a Curator ETB poses (the
// split KChoose and the pile pick KChoose) through the ask seam, so an
// effects-level test can drive each chooser answer without an engine. Every
// other decision falls through to fakeHost's deterministic stand-in, exactly
// as fakeHost.Ask does.
type curatorTapeHost struct {
	*fakeHost
	// splitIdx are the option indices the separator puts in pile A (the
	// "twopiles" split KChoose).
	splitIdx []int
	// pickIdx is the option index the opponent picks: 0 = pile-a, 1 = pile-b.
	pickIdx int
	// served records each ask the seam actually answered (a precondition for
	// the movement asserts).
	served []*decision.Decision
}

func (h *curatorTapeHost) AskCount() uint64 { return uint64(h.askCount) }

func (h *curatorTapeHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	if d.Kind != decision.KChoose || len(d.Options) == 0 {
		return decision.Intent{}, false
	}
	switch d.Options[0].Kind {
	case "twopiles":
		h.served = append(h.served, d)
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: append([]int(nil), h.splitIdx...)}, true
	case "pile-a", "pile-b":
		h.served = append(h.served, d)
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{h.pickIdx}}, true
	default:
		return decision.Intent{}, false
	}
}

// curatorSA parses Curator of Destinies' real TwoPiles sub-ability and its two
// pile bodies (its Forge script's Separate/DBHand/DBGraveyard SVars).
func curatorSA(t *testing.T) *cards.SA {
	t.Helper()
	src := "Name:Curator of Destinies\nTypes:Creature Sphinx\nPT:5/5\nOracle:x\n" +
		"SVar:Separate:DB$ TwoPiles | Defined$ You | Separator$ You | Chooser$ Opponent | DefinedCards$ Remembered | ChosenPile$ DBHand | UnchosenPile$ DBGraveyard | Zone$ Library | FaceDown$ One\n" +
		"SVar:DBHand:DB$ ChangeZone | Defined$ Remembered | Origin$ Library | Destination$ Hand\n" +
		"SVar:DBGraveyard:DB$ ChangeZoneAll | ChangeType$ Card.IsRemembered | Origin$ Library | Destination$ Graveyard\n"
	c, d := cards.ParseBytes("curator.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	sa := cards.ResolveSVar(c.Faces[0].SVars, "Separate")
	if sa == nil {
		t.Fatalf("Separate did not resolve")
	}
	return sa
}

// curatorSVars is the SVar table curatorSA resolves its pile bodies through.
func curatorSVars() map[string]string {
	return map[string]string{
		"DBHand":      "DB$ ChangeZone | Defined$ Remembered | Origin$ Library | Destination$ Hand",
		"DBGraveyard": "DB$ ChangeZoneAll | ChangeType$ Card.IsRemembered | Origin$ Library | Destination$ Graveyard",
	}
}

// runCurator drives one Curator ETB on a five-card library: the separator
// (seat 0) puts the first and third revealed cards in pile A, the opponent
// (seat 1) picks pickIdx. It returns the host and the five library ids in
// reveal order.
func runCurator(t *testing.T, pickIdx int) (*curatorTapeHost, []state.ObjID) {
	t.Helper()
	h, ids := twopilesBoard(t)
	sh := &curatorTapeHost{fakeHost: h, splitIdx: []int{0, 2}, pickIdx: pickIdx}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: curatorSVars()}
	Resolve(sh, ctx, curatorSA(t))
	if len(sh.served) != 2 {
		t.Fatalf("the seam served %d asks, want 2 (the split then the pick): %+v", len(sh.served), sh.served)
	}
	return sh, ids
}

// TestCuratorOfDestiniesETBMovesPilesWithoutShuffleOrNote drives both chooser
// answers: whichever pile the opponent picks goes to the controller's hand
// through ChangeZone, the other to the graveyard through ChangeZoneAll, and
// the resolution emits no events.Shuffle and no unread-parameter Note.
func TestCuratorOfDestiniesETBMovesPilesWithoutShuffleOrNote(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		pickIdx int
	}{
		{"opponent-takes-the-first-pile", 0},
		{"opponent-takes-the-second-pile", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sh, ids := runCurator(t, tc.pickIdx)
			pileA := []state.ObjID{ids[0], ids[2]}
			pileB := []state.ObjID{ids[1], ids[3], ids[4]}
			hand, grave := pileA, pileB
			if tc.pickIdx == 1 {
				hand, grave = pileB, pileA
			}
			for _, id := range hand {
				if z := sh.g.Obj(id).Zone; z != state.ZHand {
					t.Fatalf("chosen-pile card %d zone = %s, want hand", id, z)
				}
			}
			for _, id := range grave {
				if z := sh.g.Obj(id).Zone; z != state.ZGraveyard {
					t.Fatalf("unchosen-pile card %d zone = %s, want graveyard", id, z)
				}
			}
			for _, ev := range sh.log {
				switch {
				case ev.Kind == events.Shuffle:
					t.Fatalf("Curator ETB emitted a Shuffle for player %d; its oracle text never shuffles: %+v", ev.Player, sh.log)
				case ev.Kind == events.Note && strings.Contains(ev.Text, "unread parameter"):
					t.Fatalf("Curator ETB emitted an unread-parameter Note: %q", ev.Text)
				}
			}
		})
	}
}

// TestTwoPilesSplitExcludesTriggerCapture pins the rememberedExcludingCapture
// read. A real ETB trigger seeds Ctx.Captured and Ctx.Remembered
// with the event object, then RememberPeeked$ appends the peeked cards, so the
// raw Remembered list holds the entering source FIRST. Without this read the
// split offers the source as a pile card, and the no-host default (and the oracle
// generator) picks it. The split must offer only the peeked cards.
func TestTwoPilesSplitExcludesTriggerCapture(t *testing.T) {
	t.Parallel()
	h, ids := twopilesBoard(t)
	src := ids[5]
	// Mirror the real seeding: the source captured first, then the five
	// peeked cards appended.
	remembered := append([]state.Target{{Obj: src}}, twopilesRemembered(ids[:5])...)
	sh := &curatorTapeHost{fakeHost: h, splitIdx: []int{0}, pickIdx: 0}
	ctx := &Ctx{
		Controller: 0,
		Source:     src,
		Remembered: remembered,
		Captured:   []state.Target{{Obj: src}},
		SVars:      curatorSVars(),
	}
	Resolve(sh, ctx, curatorSA(t))
	if len(sh.served) == 0 {
		t.Fatal("the split ask was not served")
	}
	split := sh.served[0]
	if len(split.Options) != 5 {
		t.Fatalf("split offered %d options, want the 5 peeked cards: %+v", len(split.Options), split.Options)
	}
	for _, o := range split.Options {
		if o.Obj == src {
			t.Fatalf("split offered the trigger source %d as a pile card: %+v", src, split.Options)
		}
	}
}
