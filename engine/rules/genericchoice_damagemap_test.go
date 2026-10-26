package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestZoyowaThreeSeatsDamageAfterBothChoices: both opponents answer the
// GenericChoice before any damage is dealt (CR 608.2c), and both then take 3.
func TestZoyowaThreeSeatsDamageAfterBothChoices(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	zoyowa := mustCorpusCard(t, reg, "Zoyowa Lava-Tongue")
	cfg := seatZeroStart(Config{Seed: 5, Names: []string{"a", "b", "c"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{zoyowa}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	if e.G.Active != 0 {
		t.Fatalf("precondition: active seat %d, want 0", e.G.Active)
	}
	id := findByName(e, "Zoyowa Lava-Tongue", 0)
	if id == 0 {
		t.Fatal("setup: no Zoyowa")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Zoyowa zone %s", e.G.Obj(id).Zone)
	}
	var fodder state.ObjID
	for _, lid := range e.G.Zone(state.ZLibrary, 0) {
		if lid != id {
			fodder = lid
			break
		}
	}
	descendThrough(t, e, fodder, state.ZLibrary)
	e.pending = nil
	e.priorityRound()
	for p := 1; p <= 2; p++ {
		if e.G.Players[p].Life != 20 {
			t.Fatalf("precondition: seat %d life %d", p, e.G.Players[p].Life)
		}
	}

	modes := 0
	firstDamage := -1
	for i := 0; i < 200 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			if d = e.Pending(); d == nil {
				break
			}
		}
		for j := range e.L.Events {
			if firstDamage < 0 && e.L.Events[j].Kind == events.Damage {
				firstDamage = j
			}
		}
		if firstDamage >= 0 {
			break
		}
		var choice []int
		switch d.Kind {
		case decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					choice = []int{o.Index}
				}
			}
		case decision.KModes:
			if d.ResumeKind != "generic_players" {
				// Decline: the option that is not "Pay the cost".
				for _, o := range d.Options {
					if o.Label != "Pay the cost" {
						choice = []int{o.Index}
					}
				}
				if len(choice) == 0 {
					t.Fatalf("no decline option: %+v", d.Options)
				}
				break
			}
			modes++
			choice = []int{0}
		case decision.KTriggerOrder:
			for _, o := range d.Options {
				choice = append(choice, o.Index)
			}
		default:
			// An unless-cost ask: decline (no choices).
			choice = nil
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choice}); err != nil {
			t.Fatalf("submit %v: %v (decision %+v)", d.Kind, err, d)
		}
	}
	// Finish the resolution.
	for i := 0; i < 50 && e.Pending() != nil && e.G.Players[1].Life == 20; i++ {
		d := e.Pending()
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
			t.Fatalf("finish: %v", err)
		}
	}
	if modes != 2 {
		t.Fatalf("modes answers before the first Damage = %d, want 2 (one per opponent)", modes)
	}
	if e.G.Players[1].Life != 17 || e.G.Players[2].Life != 17 {
		t.Fatalf("life %d/%d, want 17/17", e.G.Players[1].Life, e.G.Players[2].Life)
	}
	replayCheck(t, e, cfg)
}
