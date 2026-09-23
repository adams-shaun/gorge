package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

const testRepeatEachMessage = "Do you want to create X 1/1 red Elemental creature tokens with haste?"

// repeatEachOptionalSA builds the RepeatOptionalForEachPlayer$ unit under
// test, with the body pointed at a spy SVar so the effects-level tests can
// observe whether a subject's body ran.
func repeatEachOptionalSA() *cards.SA {
	return &cards.SA{API: "RepeatEach", Params: map[string]string{
		"RepeatSubAbility":            "Body",
		"RepeatPlayers":               "Player.Opponent",
		"RepeatOptionalForEachPlayer": "True",
		"RepeatOptionalMessage":       testRepeatEachMessage,
	}}
}

// repeatEachOptionalSPrecondition ties the unit to the real corpus carrier:
// Tempt with Vengeance must still carry the parameter pair this feature
// reads, so the shape under test is the corpus shape, not an invented one.
func repeatEachOptionalSPrecondition(t *testing.T) {
	t.Helper()
	if _, sa := corpusSA(t, "Tempt with Vengeance", "DBRepeat"); sa == nil ||
		sa.Params["RepeatOptionalForEachPlayer"] != "True" || sa.Params["RepeatOptionalMessage"] == "" {
		t.Fatal("precondition failed: Tempt with Vengeance's RepeatOptionalForEachPlayer$ shape changed")
	}
}

// TestRepeatEachOptionalForEachPlayerNoAskDeclines is the R-9 contract: a
// host with no decision channel (the effects double's Ask reports false)
// declines each subject, never parks a suspension and never runs a body.
func TestRepeatEachOptionalForEachPlayerNoAskDeclines(t *testing.T) {
	repeatEachOptionalSPrecondition(t)
	ran := 0
	Register("TestRepeatEachOptionalSpy", func(Host, *Ctx, *cards.SA) { ran++ })
	t.Cleanup(func() { unregister("TestRepeatEachOptionalSpy") })

	h := newHost(t, 3)
	c := &Ctx{Source: 1, Controller: 0, SVars: map[string]string{"Body": "DB$ TestRepeatEachOptionalSpy"}}
	effRepeatEach(h, c, repeatEachOptionalSA())

	if h.askCount != 2 {
		t.Fatalf("no-ask host asked %d times, want 2 (one election per opponent)", h.askCount)
	}
	if ran != 0 {
		t.Fatalf("no-ask host ran %d bodies, want 0 (every election declined)", ran)
	}
	if len(h.repeatSuspensions) != 0 {
		t.Fatalf("no-ask host parked %d suspensions, want 0", len(h.repeatSuspensions))
	}
	if h.lastAsk == nil || h.lastAsk.ResumeKind != "repeat_each_optional" ||
		h.lastAsk.Player != 2 || h.lastAsk.Prompt != testRepeatEachMessage {
		t.Fatalf("last election = %+v, want repeat_each_optional for player 2 with the message prompt", h.lastAsk)
	}
}

// TestRepeatEachOptionalForEachPlayerParksTheElectionCursor pins the payload
// an election suspension carries: the loop cursor with Election set, the
// offered subject index, the full captured subject list and the subject
// itself (the Imprinted binding).
func TestRepeatEachOptionalForEachPlayerParksTheElectionCursor(t *testing.T) {
	repeatEachOptionalSPrecondition(t)
	h := newHost(t, 3)
	h.askResult = true
	h.suspendAfterAsk = true
	c := &Ctx{Source: 1, Controller: 0, SVars: map[string]string{"Body": "DB$ TestRepeatEachOptionalSpy"}}
	effRepeatEach(h, c, repeatEachOptionalSA())

	if len(h.repeatSuspensions) != 1 {
		t.Fatalf("parked %d suspensions, want 1 (the first subject's election)", len(h.repeatSuspensions))
	}
	s := h.repeatSuspensions[0]
	if !s.Election {
		t.Fatalf("suspension Election = false, want true (a per-subject election frame): %+v", s)
	}
	if s.Next != 0 {
		t.Fatalf("suspension Next = %d, want 0 (the offered subject)", s.Next)
	}
	if len(s.Subjects) != 2 || !s.Subjects[0].IsPlayer || s.Subjects[0].Player != 1 {
		t.Fatalf("suspension subjects = %+v, want [player 1, player 2]", s.Subjects)
	}
	if s.Subject.Player != 1 {
		t.Fatalf("suspension subject = %+v, want player 1's offer", s.Subject)
	}
	if h.lastAsk == nil || h.lastAsk.ResumeKind != "repeat_each_optional" || h.lastAsk.Player != 1 {
		t.Fatalf("election ask = %+v, want repeat_each_optional offered to player 1", h.lastAsk)
	}
}

// TestRepeatEachOptionalForEachPlayerReentryHonoursTheAnswer proves the
// re-entry side: an accepted subject runs its own body exactly once (and the
// next subject is still asked), while a declined subject runs no body and the
// loop still continues to the next subject.
func TestRepeatEachOptionalForEachPlayerReentryHonoursTheAnswer(t *testing.T) {
	repeatEachOptionalSPrecondition(t)
	ran := 0
	Register("TestRepeatEachOptionalSpy", func(Host, *Ctx, *cards.SA) { ran++ })
	t.Cleanup(func() { unregister("TestRepeatEachOptionalSpy") })

	subjects := []state.Target{{Player: 1, IsPlayer: true}, {Player: 2, IsPlayer: true}}
	sa := repeatEachOptionalSA()

	// Accept the offer at subject 1: its body runs once, then subject 2 is
	// asked (and declines on the no-ask host).
	h := newHost(t, 3)
	c := &Ctx{Source: 1, Controller: 0, SVars: map[string]string{"Body": "DB$ TestRepeatEachOptionalSpy"},
		Repeat:             &RepeatCursor{SA: sa, Subjects: subjects, Next: 0, Election: true},
		RepeatEachOptional: &RepeatEachOptionalContinuation{Next: 0, Accept: true}}
	effRepeatEach(h, c, sa)
	if ran != 1 {
		t.Fatalf("accepted re-entry ran %d bodies, want 1", ran)
	}
	if h.askCount != 1 || h.lastAsk == nil || h.lastAsk.Player != 2 {
		t.Fatalf("after the accepted body, ask = %+v (count %d), want subject 2's election", h.lastAsk, h.askCount)
	}

	// Decline the offer at subject 1: no body runs and the loop moves on to
	// subject 2's election.
	ran = 0
	h2 := newHost(t, 3)
	c2 := &Ctx{Source: 1, Controller: 0, SVars: map[string]string{"Body": "DB$ TestRepeatEachOptionalSpy"},
		Repeat:             &RepeatCursor{SA: sa, Subjects: subjects, Next: 0, Election: true},
		RepeatEachOptional: &RepeatEachOptionalContinuation{Next: 0, Accept: false}}
	effRepeatEach(h2, c2, sa)
	if ran != 0 {
		t.Fatalf("declined re-entry ran %d bodies, want 0", ran)
	}
	if h2.askCount != 1 || h2.lastAsk == nil || h2.lastAsk.Player != 2 {
		t.Fatalf("after the declined subject, ask = %+v (count %d), want subject 2's election", h2.lastAsk, h2.askCount)
	}
}
