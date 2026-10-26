// setstate_turnup_guard_test.go — an effect's "turn it face up" (SetState
// Mode$ TurnFaceUp) honours CR 701.40g/701.58g (a manifested or cloaked
// instant or sorcery is revealed and stays face down) and Hauntwoods
// Shrieker's RevealFirst$/ValidNewFace$ riders. Corpus cards only.
package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// turnUpGuardCounts reports the TurnFaceUp events and the public reveal Notes
// naming card after log index from.
func turnUpGuardCounts(e *Engine, from int, card string) (turnUps, reveals int) {
	for _, ev := range e.L.Events[from:] {
		switch {
		case ev.Kind == events.TurnFaceUp:
			turnUps++
		case ev.Kind == events.Note && !ev.Secret && len(ev.IDs) == 1 && strings.Contains(ev.Text, card):
			reveals++
		}
	}
	return
}

// hauntwoodsActivate funds and activates Hauntwoods Shrieker targeting tgt,
// stopping at the first non-priority decision after the target is chosen.
// It returns the setstate_optional ask when one was posed.
func hauntwoodsActivate(t *testing.T, e *Engine, shrieker, tgt state.ObjID) *decision.Decision {
	t.Helper()
	addMana(t, e, 0, "GGGGGG")
	activateAbility(t, e, shrieker)
	d := e.Pending()
	if d != nil && d.Kind == decision.KTarget {
		idx := -1
		for _, o := range d.Options {
			if o.Obj == tgt {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("target %d not offered: %+v", tgt, d.Options)
		}
		submitChoices(t, e, idx)
	}
	for i := 0; i < 60; i++ {
		d = e.Pending()
		if d == nil {
			return nil
		}
		if d.ResumeKind == "setstate_optional" {
			return d
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				return nil
			}
			passOnce(t, e)
			continue
		}
		t.Fatalf("unexpected decision: %+v", d)
	}
	t.Fatal("stack never emptied")
	return nil
}

func TestHauntwoodsRevealsNoncreatureAndLeavesItFaceDown(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Hauntwoods Shrieker", "Lightning Bolt")
	shrieker := moveByName(t, e, 0, "Hauntwoods Shrieker", state.ZBattlefield)
	bolt := manifestCard(t, e, 0, "Lightning Bolt")
	if o := e.G.Obj(bolt); o.Zone != state.ZBattlefield || !o.FaceDown {
		t.Fatalf("precondition: the Bolt is not face down on the battlefield: %+v", o)
	}
	from := len(e.L.Events)
	if ask := hauntwoodsActivate(t, e, shrieker, bolt); ask != nil {
		t.Fatalf("a setstate_optional ask was posed for a noncreature card: %+v", ask)
	}
	passUntilStackEmpty(t, e, 50)
	turnUps, reveals := turnUpGuardCounts(e, from, "Lightning Bolt")
	if turnUps != 0 || !e.G.Obj(bolt).FaceDown {
		t.Errorf("Bolt turned face up: %d TurnFaceUp events, FaceDown=%v", turnUps, e.G.Obj(bolt).FaceDown)
	}
	if reveals != 1 {
		t.Errorf("public reveal Notes naming Lightning Bolt = %d, want 1", reveals)
	}
	replayCheck(t, e, cfg)
}

func TestHauntwoodsRejectsNoncreaturePermanentOnValidNewFace(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Hauntwoods Shrieker", "Darksteel Ingot")
	shrieker := moveByName(t, e, 0, "Hauntwoods Shrieker", state.ZBattlefield)
	ingot := manifestCard(t, e, 0, "Darksteel Ingot")
	if o := e.G.Obj(ingot); o.Zone != state.ZBattlefield || !o.FaceDown {
		t.Fatalf("precondition: the Ingot is not face down on the battlefield: %+v", o)
	}
	from := len(e.L.Events)
	if ask := hauntwoodsActivate(t, e, shrieker, ingot); ask != nil {
		t.Fatalf("a setstate_optional ask was posed for a card outside ValidNewFace$: %+v", ask)
	}
	passUntilStackEmpty(t, e, 50)
	turnUps, reveals := turnUpGuardCounts(e, from, "Darksteel Ingot")
	if turnUps != 0 || !e.G.Obj(ingot).FaceDown {
		t.Errorf("Ingot turned face up past ValidNewFace$ Creature: %d TurnFaceUp events, FaceDown=%v",
			turnUps, e.G.Obj(ingot).FaceDown)
	}
	if reveals != 1 {
		t.Errorf("public reveal Notes naming Darksteel Ingot = %d, want 1", reveals)
	}
	replayCheck(t, e, cfg)
}

func TestHauntwoodsTurnsUpAManifestedCreatureOnYes(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Hauntwoods Shrieker", "Grizzly Bears")
	shrieker := moveByName(t, e, 0, "Hauntwoods Shrieker", state.ZBattlefield)
	bear := manifestCard(t, e, 0, "Grizzly Bears")
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield || !o.FaceDown {
		t.Fatalf("precondition: the bear is not face down on the battlefield: %+v", o)
	}
	from := len(e.L.Events)
	ask := hauntwoodsActivate(t, e, shrieker, bear)
	if ask == nil {
		t.Fatal("no setstate_optional ask for a creature card")
	}
	if _, reveals := turnUpGuardCounts(e, from, "Grizzly Bears"); reveals != 1 {
		t.Errorf("reveal Notes before the ask = %d, want 1", reveals)
	}
	if !e.G.Obj(bear).FaceDown {
		t.Fatal("the bear turned up before the election")
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 50)
	if turnUps, _ := turnUpGuardCounts(e, from, "Grizzly Bears"); turnUps != 1 || e.G.Obj(bear).FaceDown {
		t.Errorf("after yes: %d TurnFaceUp events, FaceDown=%v", turnUps, e.G.Obj(bear).FaceDown)
	}
	replayCheck(t, e, cfg)
}

func TestEffectTurnUpLeavesManifestedSorceryFaceDown(t *testing.T) {
	reg := searchTestRegistry(t)
	mtn := searchCorpusCard(t, reg, "Mountain")
	deck := []*cards.Card{searchCorpusCard(t, reg, "Break Open")}
	opp := []*cards.Card{searchCorpusCard(t, reg, "Divination")}
	for len(deck) < 40 {
		deck = append(deck, mtn)
	}
	for len(opp) < 40 {
		opp = append(opp, mtn)
	}
	cfg := seatZeroStart(Config{Seed: 7702, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	div := manifestCard(t, e, 1, "Divination")
	if o := e.G.Obj(div); o.Zone != state.ZBattlefield || !o.FaceDown {
		t.Fatalf("precondition: the sorcery is not face down on the battlefield: %+v", o)
	}
	from := len(e.L.Events)
	castCorpusSpell(t, e, "Break Open", "1R", div)
	turnUps, reveals := turnUpGuardCounts(e, from, "Divination")
	if turnUps != 0 || !e.G.Obj(div).FaceDown {
		t.Errorf("sorcery turned face up: %d TurnFaceUp events, FaceDown=%v", turnUps, e.G.Obj(div).FaceDown)
	}
	if reveals != 1 {
		t.Errorf("public reveal Notes naming Divination = %d, want 1", reveals)
	}
	replayCheck(t, e, cfg)
}
