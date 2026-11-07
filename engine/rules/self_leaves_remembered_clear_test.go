package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Panharmonicon-doubled leaves trigger of Unidentified Hovership: the first
// instance manifests dread from the remembered exile and its Cleanup
// ClearRemembered$ empties the source's list; the second instance's
// DefinedPlayer$ RememberedOwner must then resolve to NOBODY (the owner read
// is the source's list at resolution) and manifest nothing. The
// pending-referent snapshot (CR 400.7) must not stand in the way: the firing
// departure is not a forgetting, so the live list is what the resolution
// reads. Measured from the std-dsk lockstep pair 58 (Unidentified
// Hovership#133, a The Jolly Balloon Man copy sacrificed at end of turn
// beside a live Fractured Realm).
func TestUnidentifiedHovershipDoubledLeaveSeesRememberedClear(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	plains := searchCorpusCard(t, reg, "Plains")
	mountain := searchCorpusCard(t, reg, "Mountain")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	deck := []*cards.Card{
		searchCorpusCard(t, reg, "Unidentified Hovership"),
		searchCorpusCard(t, reg, "Delney, Streetwise Lookout"),
		searchCorpusCard(t, reg, "Shatter"),
	}
	for len(deck) < 40 {
		deck = append(deck, plains, mountain)
	}
	opp := []*cards.Card{bear}
	for len(opp) < 40 {
		opp = append(opp, mountain)
	}
	cfg := seatZeroStart(Config{Seed: 4242, Names: []string{"hovership", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	bearID := state.ObjID(0)
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 1) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield})
				e.pending = nil
				bearID = id
				break
			}
		}
		if bearID != 0 {
			break
		}
	}
	if bearID == 0 {
		t.Fatal("no Grizzly Bears in seat 1's hand/library to stage")
	}
	e.priorityRound()

	// Cast Delney: its static doubles the leave triggers of your creatures
	// with power 2 or less. The Hovership's ETB exile itself is single (a
	// Vehicle is not a creature as it enters).
	delney := searchMoveByName(t, e, "Delney, Streetwise Lookout", state.ZHand)
	addMana(t, e, 0, "WWW")
	castByID(t, e, delney)
	if d := passUntilResolved(t, e, 20); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("while Delney entered: %+v", d)
	}

	// Cast the Hovership; its ETB exiles the bear (RememberChanged$ True
	// writes it into the source's remembered list).
	hov := searchMoveByName(t, e, "Unidentified Hovership", state.ZHand)
	addMana(t, e, 0, "WWW")
	castByID(t, e, hov)
	d := passUntilResolved(t, e, 20)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after Hovership cast: %+v, want the ETB target ask", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == bearID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("bear %d not offered: %+v", bearID, d.Options)
	}
	submitChoices(t, e, idx)
	if d = passUntilResolved(t, e, 20); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("while ETB resolved: %+v", d)
	}

	// Crew the Hovership (Crew 1) with Delney itself: the Vehicle becomes a
	// 2/2 artifact creature, so Delney's powerLE2 filter matches it as it
	// leaves. (The staged bear is the ETB's exile, in seat 1's exile zone;
	// seat 1's creatures are not "you control" for seat 0's crew.)
	opt := abilityOption(t, e, hov, 0)
	submitChoices(t, e, opt.Index)
	if d = e.Pending(); d == nil || d.Kind != decision.KChoose {
		t.Fatalf("pending decision %+v, want the crew tap election", d)
	}
	// Seat 0's own creatures are the only legal crewers (Delney itself,
	// power 2, meets the Crew 1 floor).
	if len(d.Options) == 0 {
		t.Fatal("no crew candidates offered")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
		t.Fatalf("crew election rejected: %v", err)
	}
	if d = passUntilResolved(t, e, 20); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("while crew resolved: %+v", d)
	}

	// Shatter the Hovership: its leave trigger fires twice (Delney), the
	// first instance manifests dread from seat 1's library and its cleanup
	// empties the source's remembered list, the second must find it empty.
	// The drive runs to the turn boundary: the crew bookkeeping is an
	// event-folded per-turn fact and the replay check compares whole games,
	// so both sides must reach the same TurnChange fold (which resets it)
	// rather than a mid-turn kernel restore, which normalises the emptied
	// list to nil while the log-only replay keeps the emptied non-nil slice.
	turn := e.G.Turn
	shatter := searchMoveByName(t, e, "Shatter", state.ZHand)
	addMana(t, e, 0, "RRR")
	castByID(t, e, shatter)
	if d = e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after Shatter: %+v, want target ask", d)
	}
	tIdx := -1
	for _, o := range d.Options {
		if o.Obj == hov {
			tIdx = o.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("Hovership %d not offered to Shatter: %+v", hov, d.Options)
	}
	submitChoices(t, e, tIdx)
	for i := 0; i < 4000; i++ {
		if e.G.Turn != turn || e.G.Over {
			break
		}
		d = e.Pending()
		if d == nil {
			t.Fatalf("no decision pending (game over: %v)", e.G.Over)
		}
		if d.Kind != decision.KPriority {
			switch d.Kind {
			case decision.KChoose:
				submitChoices(t, e, d.Options[0].Index)
			case decision.KTriggerOrder:
				idxs := make([]int, 0, len(d.Options))
				for _, o := range d.Options {
					idxs = append(idxs, o.Index)
				}
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: idxs}); err != nil {
					t.Fatalf("trigger order rejected: %v", err)
				}
			case decision.KTarget:
				t.Fatalf("unexpected target ask: %+v", d)
			default:
				t.Fatalf("unexpected ask %v: %+v", d.Kind, d.Options)
			}
			continue
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
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	if e.G.Turn == turn {
		t.Fatalf("the turn never ended after the Hovership died")
	}
	// Precondition: the ETB push plus the DOUBLED leave push.
	pushes := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == hov {
			pushes++
		}
	}
	if pushes != 3 {
		t.Fatalf("precondition: %d trigger pushes for the Hovership, want 3 (one ETB, one doubled leave)", pushes)
	}
	dreads := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.ManifestDreadAction {
			dreads++
		}
	}
	if dreads != 1 {
		t.Errorf("%d ManifestDread resolutions, want 1: the second instance must see the Cleanup's cleared remembered list", dreads)
	}
	replayCheck(t, e, cfg)
}
