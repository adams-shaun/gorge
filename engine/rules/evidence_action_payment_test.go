package rules

// CR 701.59a: paying a collect-evidence cost is the "collect evidence" action
// wherever the cost is paid. The Ward and unless-cost settlements exiled the
// cards but left no CollectEvidenceAction marker, so a Mode$ CollectEvidence
// trigger (Evidence Examiner) never saw the payment.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func countEvidenceActions(e *Engine, p state.PlayerID) int {
	return countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.CollectEvidenceAction && ev.Player == p
	})
}

func TestWardCollectEvidencePaymentRecordsAction(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, nil, nil)
	addToBattlefield(t, e, searchCorpusCard(t, reg, "Evidence Examiner"), 1)
	feroxID := addToBattlefield(t, e, searchCorpusCard(t, reg, "Axebane Ferox"), 0)
	giant := e.G.AddObject(searchCorpusCard(t, reg, "Hill Giant"), 1)
	giant.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 1, append(append([]state.ObjID{}, e.G.Zone(state.ZGraveyard, 1)...), giant.ID))
	bolt := e.G.AddObject(card(t, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n"), 1)
	e.G.SetZone(state.ZHand, 1, []state.ObjID{bolt.ID})
	addMana(t, e, 1, "R")

	if n := countEvidenceActions(e, 1); n != 0 {
		t.Fatalf("precondition: %d CollectEvidenceAction events before payment", n)
	}
	if n := countTokensNamedOnSeat(t, e, 1, "Clue Token"); n != 0 {
		t.Fatalf("precondition: %d Clue tokens before payment", n)
	}
	if e.G.Obj(giant.ID).Zone != state.ZGraveyard {
		t.Fatal("precondition: Hill Giant must start in the graveyard")
	}

	opponentBoltAt(t, e, feroxID)
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.Player != 1 {
		t.Fatalf("want the ward pay-or-decline ask for seat 1, got %+v", d)
	}
	pay := -1
	for _, o := range d.Options {
		if o.Mode == decision.ModeUnlessPay {
			pay = o.Index
		}
	}
	if pay < 0 {
		t.Fatalf("ward ask has no ModeUnlessPay option: %+v", d.Options)
	}
	submitChoices(t, e, pay)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "ward_evidence" {
		t.Fatalf("want the ward_evidence pick, got %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == giant.ID {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("Hill Giant not offered: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 20)

	if z := e.G.Obj(giant.ID).Zone; z != state.ZExile {
		t.Fatalf("Hill Giant zone = %v, want exile", z)
	}
	if n := countEvidenceActions(e, 1); n != 1 {
		t.Fatalf("CollectEvidenceAction by seat 1 = %d, want 1", n)
	}
	if n := countTokensNamedOnSeat(t, e, 1, "Clue Token"); n != 1 {
		t.Fatalf("Clue tokens on seat 1 = %d, want 1 (Evidence Examiner investigated)", n)
	}
	if e.G.Obj(feroxID).Zone != state.ZBattlefield {
		t.Fatal("Axebane Ferox must survive once the ward is paid")
	}
}

func TestUnlessCollectEvidencePaymentRecordsAction(t *testing.T) {
	t.Parallel()
	drive := func(t *testing.T, seed uint64) (*Engine, []state.ObjID) {
		t.Helper()
		e := stealEngine(t, seed)
		probe := onBoard(t, e, 0, ugTrigFixture("CollectEvidence<4>"))
		var graves []state.ObjID
		for range 3 {
			graves = append(graves, ugPlace(t, e, 0, "Grizzly Bears", state.ZGraveyard))
		}
		ugPushTrigger(e, probe, 0)
		if n := countEvidenceActions(e, 0); n != 0 {
			t.Fatalf("precondition: %d CollectEvidenceAction events before payment", n)
		}
		return e, graves
	}
	t.Run("pay", func(t *testing.T) {
		t.Parallel()
		e, graves := drive(t, 9631)
		if !ugOffer(t, e, 0) {
			t.Fatal("6 mana value in the graveyard, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		ugPick(t, e, "evidence", graves[0], graves[2])
		if ugZone(e, graves[0]) != state.ZExile || ugZone(e, graves[2]) != state.ZExile || ugZone(e, graves[1]) != state.ZGraveyard {
			t.Fatal("evidence must exile exactly the two chosen cards")
		}
		if n := countEvidenceActions(e, 0); n != 1 {
			t.Fatalf("CollectEvidenceAction by seat 0 = %d, want 1", n)
		}
	})
	t.Run("decline", func(t *testing.T) {
		t.Parallel()
		e, graves := drive(t, 9632)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		for _, g := range graves {
			if ugZone(e, g) != state.ZGraveyard {
				t.Fatal("a declined evidence cost exiled a card")
			}
		}
		if n := countEvidenceActions(e, 0); n != 0 {
			t.Fatalf("CollectEvidenceAction after decline = %d, want 0", n)
		}
	})
}
