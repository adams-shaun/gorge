package rules

// CR 702.134b: "If a creature has multiple instances of mentor, each triggers
// separately." A printed K:Mentor is one instance and every layer-6
// AddKeyword$ Mentor grant (Nyxborn Unicorn's Bestow aura, Aegis of the
// Legion's equipment) is another, so the granted-keyword walk must queue one
// Mentor trigger per granting static beside the printed expansion's own --
// never fold them into one. engine/cards/kw_mentor.go is the printed shape;
// scriptfacts.GrantedKeywordTrigger synthesizes the granted one.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mentorPendingCount is the number of Mentor triggers queued from source: a
// queued trigger whose inline body carries the Mentor$ marker. The printed
// expansion and the granted synthesis share that marker, so the count is one
// per instance regardless of route.
func mentorPendingCount(e *Engine, source state.ObjID) int {
	n := 0
	for _, pt := range e.pendingTriggers {
		if pt.Source != source || pt.SA == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(pt.SA.ParamStr(cards.PKMentor)), "True") {
			n++
		}
	}
	return n
}

// TestGrantedMentorTriggersPerInstance measures the three shapes the CR
// separates: a printer granted one more, a vanilla creature granted two, and
// a vanilla creature granted one.
func TestGrantedMentorTriggersPerInstance(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	banneret := mustCorpusCard(t, reg, "Goblin Banneret") // printed K:Mentor
	unicorn := mustCorpusCard(t, reg, "Nyxborn Unicorn")  // AddKeyword$ Mentor
	memnite := mustCorpusCard(t, reg, "Memnite")          // lesser-power attacker
	bears := mustCorpusCard(t, reg, "Grizzly Bears")      // vanilla recipient
	for _, c := range []*cards.Card{banneret, unicorn, memnite, bears} {
		if d := c.Link(); len(d) != 0 {
			t.Fatalf("link %s: %v", c.Faces[0].Name, d)
		}
	}
	if !banneret.Faces[0].HasKeyword("Mentor") {
		t.Fatal("precondition: Goblin Banneret does not print Mentor")
	}
	if bears.Faces[0].HasKeyword("Mentor") {
		t.Fatal("precondition: Grizzly Bears must be a vanilla recipient")
	}

	for _, tc := range []struct {
		name    string
		printed bool
		grants  int
		want    int
	}{
		{"printed plus one grant", true, 1, 2},
		{"vanilla with two grants", false, 2, 2},
		{"vanilla with one grant", false, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deck := []*cards.Card{memnite}
			if tc.printed {
				deck = append(deck, banneret)
			} else {
				deck = append(deck, bears)
			}
			for i := 0; i < tc.grants; i++ {
				deck = append(deck, unicorn)
			}
			e, _ := trainingDeck(t, 311+uint64(tc.grants), deck...)
			var recipient state.ObjID
			if tc.printed {
				recipient = battlefieldID(t, e, "Goblin Banneret")
				e.emit(events.Event{Kind: events.MoveZone, Obj: recipient, From: state.ZHand, To: state.ZBattlefield})
			} else {
				recipient = battlefieldID(t, e, "Grizzly Bears")
				e.emit(events.Event{Kind: events.MoveZone, Obj: recipient, From: state.ZHand, To: state.ZBattlefield})
			}
			target := battlefieldID(t, e, "Memnite")
			e.emit(events.Event{Kind: events.MoveZone, Obj: target, From: state.ZHand, To: state.ZBattlefield})

			// A bestowed Nyxborn Unicorn is an Aura attached to the creature;
			// attaching the real corpus card puts the layer-6 grant static in
			// exactly that state, which is what the trigger walk reads.
			for i := 0; i < tc.grants; i++ {
				lossAttach(t, e, "Nyxborn Unicorn", 0, recipient)
			}
			// Precondition: the layer-6 grants really gave the recipient the
			// keyword, so a missing trigger below is the multi-instance bug and
			// not a missing grant.
			if !e.HasKeyword(recipient, "Mentor") {
				t.Fatal("precondition: the AddKeyword$ Mentor grants did not reach the recipient")
			}
			if got := e.Power(recipient); got < 1 {
				t.Fatalf("precondition: recipient power = %d, want a positive attacker", got)
			}
			e.pendingTriggers = nil
			e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{recipient, target}})
			if got := mentorPendingCount(e, recipient); got != tc.want {
				t.Fatalf("queued %d Mentor triggers, want %d (%s)", got, tc.want, tc.name)
			}
		})
	}
}
