package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The two riders the bare-Choices$ RemoveCounter arm reads beyond Amy Pond's
// scope, both pinned on the real corpus cards that carry them:
//
//   - ChoiceOptional$ True is Forge's "each of ANY number", not an
//     "up to one". Garnet, Princess of Alexandria removes a lore counter from
//     each of any number of Sagas you control, so a two-Saga board must be
//     able to take BOTH (the earlier reading kept Max at the default 1 and
//     silently misplayed the card).
//   - RememberAmount$ True is the removed-COUNT transport the chained payoff
//     reads through Count$RememberedNumber (Garnet's X +1/+1 counters,
//     Dyadrine, Synthesis Amalgam's ConditionCheckSVar$ Z draw-and-token).
//     Without it the removal happened and the payoff saw zero.
//
// Shapes the arm still withholds keep their loud Note, pinned below.

// sagaOnBattlefield mints a Saga permanent under seat 0 with n lore counters,
// through logged events (never a direct state write).
func sagaOnBattlefield(t *testing.T, h *fakeHost, name string, lore int32) state.ObjID {
	t.Helper()
	id := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Enchantment Saga\nOracle:x\n"), 0).ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "LORE", Amount: lore})
	return id
}

// garnetFixture puts Garnet and two lore-bearing Sagas on the battlefield and
// returns the compiled TrigRemoveCounter SA plus a Ctx carrying Garnet's own
// SVar table (SVar:X:Count$RememberedNumber is what the payoff reads).
func garnetFixture(t *testing.T) (*fakeHost, *Ctx, *cards.SA, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	card, trig := corpusSA(t, "Garnet, Princess of Alexandria", "TrigRemoveCounter")
	if trig.API != "RemoveCounter" ||
		!strings.EqualFold(trig.Params["ChoiceOptional"], "True") ||
		!strings.EqualFold(trig.Params["RememberAmount"], "True") ||
		trig.Params["ChoiceNum"] != "" {
		t.Fatalf("Garnet's compiled trigger = %s %+v, want RemoveCounter with ChoiceOptional$/RememberAmount$ and no ChoiceNum$", trig.API, trig.Params)
	}
	h := newHost(t, 2)
	garnet := h.g.AddObject(card, 0).ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: garnet, From: state.ZLibrary, To: state.ZBattlefield})
	a := sagaOnBattlefield(t, h, "Saga A", 2)
	b := sagaOnBattlefield(t, h, "Saga B", 3)
	c := &Ctx{Controller: 0, Source: garnet, SVars: h.g.Obj(garnet).Face().SVars}
	return h, c, trig, garnet, a, b
}

// Finding 1: the election Garnet poses is 0..(every eligible Saga), so both
// Sagas can be chosen. A Max of 1 here is the misplay this pins against.
func TestGarnetAnyNumberElectionOffersEverySaga(t *testing.T) {
	h, c, trig, _, a, b := garnetFixture(t)
	h.askResult = false // record the decision, then take the stand-in
	head := *trig
	head.Sub = nil
	Resolve(h, c, &head)
	d := h.lastAsk
	if d == nil {
		t.Fatal("no election posed for Garnet's any-number lore removal")
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("election range = %d..%d, want 0..2 (ChoiceOptional$ True is \"each of any number\")", d.Min, d.Max)
	}
	if len(d.Options) != 2 || d.Options[0].Obj != a || d.Options[1].Obj != b {
		t.Fatalf("options = %+v, want both Sagas in battlefield order (%d, %d)", d.Options, a, b)
	}
}

// Findings 1 and 2 together, end to end on the compiled card: BOTH Sagas are
// answered, each loses one lore counter, and the chained DBPutCounter reads
// X = Count$RememberedNumber = 2, so Garnet gets two +1/+1 counters.
func TestGarnetAnyNumberRemovesFromBothAndPaysOffTwice(t *testing.T) {
	h, c, trig, garnet, a, b := garnetFixture(t)
	c.CounterPick, c.CounterPickDone = []state.ObjID{a, b}, true
	Resolve(h, c, trig)
	if got := h.g.Obj(a).Counter("LORE"); got != 1 {
		t.Fatalf("Saga A LORE = %d, want 1 (one removed of 2)", got)
	}
	if got := h.g.Obj(b).Counter("LORE"); got != 2 {
		t.Fatalf("Saga B LORE = %d, want 2 (one removed of 3)", got)
	}
	if got := h.g.Obj(garnet).Counter("P1P1"); got != 2 {
		t.Fatalf("Garnet P1P1 = %d, want 2 (one per lore counter removed this way)", got)
	}
	assertNoRemoveCounterChoiceNote(t, h)
}

// Answering with ONE Saga pays off once: the transport counts counters, not
// eligible objects.
func TestGarnetAnyNumberOneSagaPaysOffOnce(t *testing.T) {
	h, c, trig, garnet, _, b := garnetFixture(t)
	c.CounterPick, c.CounterPickDone = []state.ObjID{b}, true
	Resolve(h, c, trig)
	if got := h.g.Obj(garnet).Counter("P1P1"); got != 1 {
		t.Fatalf("Garnet P1P1 = %d, want 1", got)
	}
}

// A Min-0 decline removes nothing and pays off nothing (the "you may" half).
func TestGarnetAnyNumberDeclineRemovesNothing(t *testing.T) {
	h, c, trig, garnet, a, b := garnetFixture(t)
	c.CounterPick, c.CounterPickDone = nil, true
	Resolve(h, c, trig)
	if h.g.Obj(a).Counter("LORE") != 2 || h.g.Obj(b).Counter("LORE") != 3 {
		t.Fatalf("declined election still removed lore: %d/%d, want 2/3",
			h.g.Obj(a).Counter("LORE"), h.g.Obj(b).Counter("LORE"))
	}
	if got := h.g.Obj(garnet).Counter("P1P1"); got != 0 {
		t.Fatalf("Garnet P1P1 = %d, want 0 after declining", got)
	}
}

// Finding 2 on the exact-count shape: Dyadrine, Synthesis Amalgam's
// ChoiceNum$ 2 carries RememberAmount$ True too, so its Remembered list holds
// one entry per counter removed (its DBDraw/DBToken gate on
// ConditionCheckSVar$ Z = Count$RememberedNumber).
func TestDyadrineRememberAmountFeedsItsPayoffGate(t *testing.T) {
	card, trig := corpusSA(t, "Dyadrine, Synthesis Amalgam", "TrigCounters")
	if trig.API != "RemoveCounter" || trig.Params["ChoiceNum"] != "2" ||
		!strings.EqualFold(trig.Params["RememberAmount"], "True") {
		t.Fatalf("Dyadrine's compiled trigger = %s %+v, want RemoveCounter ChoiceNum$ 2 RememberAmount$ True", trig.API, trig.Params)
	}
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0).ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZLibrary, To: state.ZBattlefield})
	var creatures []state.ObjID
	for _, n := range []string{"Bear One", "Bear Two"} {
		id := h.g.AddObject(mkCard(t, "Name:"+n+"\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0).ID
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
		creatures = append(creatures, id)
	}
	c := &Ctx{Controller: 0, Source: src, SVars: h.g.Obj(src).Face().SVars}
	c.CounterPick, c.CounterPickDone = creatures, true
	head := *trig
	head.Sub = nil // stop before DBDraw/DBToken/DBCleanup so Remembered survives
	Resolve(h, c, &head)
	for _, id := range creatures {
		if got := h.g.Obj(id).Counter("P1P1"); got != 0 {
			t.Fatalf("creature %d P1P1 = %d, want 0", id, got)
		}
	}
	if got := len(c.Remembered); got != 2 {
		t.Fatalf("Remembered = %+v (len %d), want one entry per counter removed (2)", c.Remembered, got)
	}
	assertNoRemoveCounterChoiceNote(t, h)
}

// A line WITHOUT RememberAmount$ still remembers nothing: the rider is read,
// not assumed (Amy Pond's trigger carries neither rider).
func TestRemoveCounterChoicesWithoutRememberAmountRemembersNothing(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 2)
	c := &Ctx{Controller: 0, Source: ids["myBear"],
		CounterPick: []state.ObjID{ids["myBear"]}, CounterPickDone: true}
	Resolve(h, c, sa(t, "DB$ RemoveCounter | Choices$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1"))
	if len(c.Remembered) != 0 {
		t.Fatalf("Remembered = %+v, want empty without RememberAmount$", c.Remembered)
	}
}

// ChoiceOptional$ True does NOT unlock the shapes the arm still withholds:
// Eventide's Shadow pairs it with CounterType$/CounterNum$ Any (a which-kind
// and a how-many-per-card ask), which stays one loud Note with nothing moved.
func TestEventidesShadowAnyAmountStaysLoudlyWithheld(t *testing.T) {
	card, trig := corpusSA(t, "Eventide's Shadow", "")
	if trig.API != "RemoveCounter" || !strings.EqualFold(trig.Params["CounterNum"], "Any") ||
		!strings.EqualFold(trig.Params["ChoiceOptional"], "True") {
		t.Fatalf("Eventide's Shadow spell = %s %+v, want RemoveCounter ChoiceOptional$ True CounterNum$ Any", trig.API, trig.Params)
	}
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0).ID
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0).ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2})
	c := &Ctx{Controller: 0, Source: src, SVars: h.g.Obj(src).Face().SVars}
	head := *trig
	head.Sub = nil
	Resolve(h, c, &head)
	if got := h.g.Obj(bear).Counter("P1P1"); got != 2 {
		t.Fatalf("bear P1P1 = %d, want 2 (a withheld shape moves nothing)", got)
	}
	if len(c.Remembered) != 0 {
		t.Fatalf("Remembered = %+v, want empty (nothing was removed)", c.Remembered)
	}
	var loud bool
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented RemoveCounter choice shape") &&
			strings.Contains(ev.Text, "CounterNum$ Any") {
			loud = true
		}
	}
	if !loud {
		t.Fatalf("withheld shape emitted no loud Note; log = %+v", h.log)
	}
}

func assertNoRemoveCounterChoiceNote(t *testing.T, h *fakeHost) {
	t.Helper()
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
			t.Fatalf("unexpected unimplemented Note: %q", ev.Text)
		}
	}
}
