package rules

// CR 702.134 Mentor: "Whenever this creature attacks, put a +1/+1 counter on
// target attacking creature with lesser power." cards/kw_mentor.go expands a
// printed K:Mentor into an ordinary Attacks trigger whose body is a targeted
// PutCounter carrying the Mentor$ marker, and rules/mentor.go's mentorAdmits
// enforces the strict lesser-power restriction at both the target offer and
// the CR 608.2b recheck. checkGrantedMentorTriggers synthesizes the same body
// for a layer-6 AddKeyword$ Mentor grant (the Dethrone/Training precedent).
//
// The closest existing fixture is rules/training_keyword_test.go, which
// reuses trainingDeck/battlefieldID/passUntilStackEmpty/replayCheck here.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// mentorAnswerTarget drains priority until the Mentor trigger's target ask
// appears, asserts want is offered, submits it, and returns the ask's
// Decision so the caller can inspect which candidates were offered. A plain
// (non-copy) KTarget would make passUntilStackEmpty Fatalf, so the ask must be
// answered by the test -- which is the point: the restriction is proved
// against the real option list, not inferred from the counter alone.
func mentorAnswerTarget(t *testing.T, e *Engine, want state.ObjID, limit int) *decision.Decision {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision while draining Mentor (stack %d)", len(e.G.Stack))
		}
		switch d.Kind {
		case decision.KTarget:
			idx := -1
			for _, o := range d.Options {
				if o.Obj == want {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("Mentor did not offer target %d; options: %+v", want, d.Options)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("submit Mentor target: %v", err)
			}
			return d
		case decision.KPriority:
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
		default:
			t.Fatalf("unexpected decision %+v while draining Mentor", d)
		}
	}
	t.Fatalf("Mentor target ask never appeared within %d steps", limit)
	return nil
}

// TestMentorCounterOnLesserPowerAttacker drives a real corpus Mentor creature
// (Legion Warboss, 2/2) attacking alongside a strictly smaller attacker
// (Memnite, 1/1), an EQUAL-power attacker (Grizzly Bears, 2/2) and a
// greater-power attacker (Craw Wurm, 6/4), plus a 1/1 that stays home. It
// asserts the offered target list contains exactly the lesser-power attacker
// (not the source, not the equal/greater attackers, not the non-attacker),
// that the chosen target receives exactly one +1/+1 counter, and that no
// other creature does.
func TestMentorCounterOnLesserPowerAttacker(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	warboss := mustCorpusCard(t, reg, "Legion Warboss") // 2/2, K:Mentor
	memnite := mustCorpusCard(t, reg, "Memnite")        // 1/1 lesser-power attacker
	bears := mustCorpusCard(t, reg, "Grizzly Bears")    // 2/2 equal power
	wurm := mustCorpusCard(t, reg, "Craw Wurm")         // 6/4 greater power
	homebody := mustCorpusCard(t, reg, "Memnite")       // 1/1, does not attack
	if d := warboss.Link(); len(d) != 0 {
		t.Fatalf("link Legion Warboss: %v", d)
	}
	if d := memnite.Link(); len(d) != 0 {
		t.Fatalf("link Memnite: %v", d)
	}
	if d := bears.Link(); len(d) != 0 {
		t.Fatalf("link Grizzly Bears: %v", d)
	}
	if d := wurm.Link(); len(d) != 0 {
		t.Fatalf("link Craw Wurm: %v", d)
	}
	if d := homebody.Link(); len(d) != 0 {
		t.Fatalf("link second Memnite: %v", d)
	}

	// Preconditions: the source really prints Mentor (so the expansion is the
	// printed path, not a granted synthesis), and the power values under
	// comparison actually differ as the assertions need.
	if !warboss.Faces[0].HasKeyword("Mentor") {
		t.Fatal("Legion Warboss does not print Mentor in the corpus")
	}
	if memnite.Faces[0].Power() != 1 {
		t.Fatalf("Memnite power = %d, want 1", memnite.Faces[0].Power())
	}
	if bears.Faces[0].Power() != 2 {
		t.Fatalf("Grizzly Bears power = %d, want 2", bears.Faces[0].Power())
	}
	if wurm.Faces[0].Power() != 6 {
		t.Fatalf("Craw Wurm power = %d, want 6", wurm.Faces[0].Power())
	}

	e, cfg := trainingDeck(t, 301, warboss, memnite, bears, wurm, homebody)
	warbossID := battlefieldID(t, e, "Legion Warboss")
	memniteID := battlefieldID(t, e, "Memnite")
	var homebodyID state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner == 0 && o.Face() != nil && o.Face().Name == "Memnite" && o.ID != memniteID {
			homebodyID = o.ID
			break
		}
	}
	if homebodyID == 0 {
		t.Fatal("second Memnite not found on the battlefield")
	}
	bearsID := battlefieldID(t, e, "Grizzly Bears")
	wurmID := battlefieldID(t, e, "Craw Wurm")

	for _, id := range []state.ObjID{warbossID, memniteID, bearsID, wurmID, homebodyID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	}
	// Attackers need not be summoning-sick-free: the declaration is emitted
	// directly (the training fixtures' licence), so no summoning-sickness state
	// is touched and replayCheck rebuilds the identical log-only board.
	for _, id := range []state.ObjID{warbossID, memniteID, bearsID, wurmID} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("attacker %d not on the battlefield", id)
		}
	}
	if e.Power(warbossID) != 2 || e.Power(memniteID) != 1 || e.Power(bearsID) != 2 || e.Power(wurmID) != 6 {
		t.Fatalf("derived powers changed before the attack: warboss=%d memnite=%d bears=%d wurm=%d",
			e.Power(warbossID), e.Power(memniteID), e.Power(bearsID), e.Power(wurmID))
	}
	if e.G.Obj(homebodyID).IsAttacking {
		t.Fatal("the homebody is attacking before the declaration")
	}

	// The four attackers declare against seat 1; the second Memnite stays home.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1,
		IDs: []state.ObjID{warbossID, memniteID, bearsID, wurmID}})
	e.priorityRound()

	d := mentorAnswerTarget(t, e, memniteID, 20)
	offered := map[state.ObjID]bool{}
	for _, o := range d.Options {
		offered[o.Obj] = true
	}
	if !offered[memniteID] {
		t.Fatalf("lesser-power attacker Memnite (%d) was not offered: %+v", memniteID, d.Options)
	}
	if offered[warbossID] {
		t.Error("Mentor offered its own source (equal power) as a target")
	}
	if offered[bearsID] {
		t.Error("Mentor offered an EQUAL-power attacker as a target")
	}
	if offered[wurmID] {
		t.Error("Mentor offered a GREATER-power attacker as a target")
	}
	if offered[homebodyID] {
		t.Error("Mentor offered a NON-ATTACKING creature as a target")
	}

	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(memniteID).Counter("P1P1"); got != 1 {
		t.Fatalf("Mentor target got %d +1/+1 counters, want 1", got)
	}
	for _, id := range []state.ObjID{warbossID, bearsID, wurmID, homebodyID} {
		if got := e.G.Obj(id).Counter("P1P1"); got != 0 {
			t.Errorf("Mentor wrongly countered a non-target %d: %d counters", id, got)
		}
	}
	replayCheck(t, e, cfg)
}

// mentorGrantSelfSrc is a creature whose ONLY source of Mentor is a layer-6
// AddKeyword$ grant on itself (no printed K:Mentor), so a counter can only
// come from checkGrantedMentorTriggers.
const mentorGrantSelfSrc = "Name:Mentor Adept\nManaCost:1 R\nTypes:Creature Human Soldier\nPT:2/2\n" +
	"S:Mode$ Continuous | Affected$ Card.Self | AddKeyword$ Mentor | Description$ NICKNAME has mentor.\nOracle:x\n"

// mentorPrintAndGrantSrc prints K:Mentor AND self-grants it, the dedup shape:
// the printed expansion and the granted synthesis must not both fire.
const mentorPrintAndGrantSrc = "Name:Mentor Veteran\nManaCost:1 R\nTypes:Creature Human Soldier\nPT:2/2\n" +
	"K:Mentor\n" +
	"S:Mode$ Continuous | Affected$ Card.Self | AddKeyword$ Mentor | Description$ NICKNAME has mentor.\nOracle:x\n"

// TestGrantedMentorFiresAndDoesNotStackWithAPrintedKeyword covers the two
// corpus AddKeyword$ Mentor carriers' route (Aegis of the Legion, Nyxborn
// Unicorn): a creature granted Mentor but printing none still mentors. It
// also pins the Dethrone/Training dedup -- a creature that PRINTS Mentor and
// is granted it again fires exactly once, so the granted synthesis must skip
// a face that already carries the printed keyword.
func TestGrantedMentorFiresAndDoesNotStackWithAPrintedKeyword(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	memnite := mustCorpusCard(t, reg, "Memnite") // 1/1, strictly lesser
	if d := memnite.Link(); len(d) != 0 {
		t.Fatalf("link Memnite: %v", d)
	}
	if memnite.Faces[0].Power() != 1 {
		t.Fatalf("Memnite power = %d, want 1", memnite.Faces[0].Power())
	}

	grantOnly := card(t, mentorGrantSelfSrc)
	printAndGrant := card(t, mentorPrintAndGrantSrc)
	// Preconditions: one card has no printed Mentor (the granted route is the
	// only path), the other prints it (the dedup path).
	if grantOnly.Faces[0].HasKeyword("Mentor") {
		t.Fatal("the grant-only fixture must not print Mentor")
	}
	if !printAndGrant.Faces[0].HasKeyword("Mentor") {
		t.Fatal("the dedup fixture must print Mentor")
	}

	// Granted-only: exactly one counter, from the synthesized trigger.
	e, cfg := trainingDeck(t, 302, grantOnly, memnite)
	adeptID := battlefieldID(t, e, "Mentor Adept")
	memID := battlefieldID(t, e, "Memnite")
	e.emit(events.Event{Kind: events.MoveZone, Obj: adeptID, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: memID, From: state.ZHand, To: state.ZBattlefield})
	if !e.HasKeyword(adeptID, "Mentor") {
		t.Fatal("the layer-6 self-grant did not give Mentor Adept the keyword")
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{adeptID, memID}})
	e.priorityRound()
	mentorAnswerTarget(t, e, memID, 20)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(memID).Counter("P1P1"); got != 1 {
		t.Fatalf("granted Mentor gave %d counters, want 1", got)
	}
	replayCheck(t, e, cfg)

	// Dedup: printed PLUS granted fires once, not twice. A second fire would
	// pose a second KTarget (which passUntilStackEmpty rejects) or a second
	// counter; either way this fails loudly.
	e2, cfg2 := trainingDeck(t, 303, printAndGrant, memnite)
	vetID := battlefieldID(t, e2, "Mentor Veteran")
	mem2ID := battlefieldID(t, e2, "Memnite")
	e2.emit(events.Event{Kind: events.MoveZone, Obj: vetID, From: state.ZHand, To: state.ZBattlefield})
	e2.emit(events.Event{Kind: events.MoveZone, Obj: mem2ID, From: state.ZHand, To: state.ZBattlefield})
	if !e2.HasKeyword(vetID, "Mentor") {
		t.Fatal("the dedup fixture lost Mentor")
	}
	e2.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{vetID, mem2ID}})
	e2.priorityRound()
	mentorAnswerTarget(t, e2, mem2ID, 20)
	passUntilStackEmpty(t, e2, 20)
	if got := e2.G.Obj(mem2ID).Counter("P1P1"); got != 1 {
		t.Fatalf("printed + granted Mentor stacked: %d counters, want 1", got)
	}
	replayCheck(t, e2, cfg2)
}
