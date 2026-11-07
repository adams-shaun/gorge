package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func wardLifeEngine(t *testing.T, ward, spell, mana string) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t)
	o := e.G.AddObject(corpusAlternativeCard(t, ward), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	if e.G.Obj(o.ID).Zone != state.ZBattlefield {
		t.Fatal("precondition: warded permanent must be on the battlefield")
	}
	b := e.G.AddObject(corpusAlternativeCard(t, spell), 1)
	e.G.SetZone(state.ZHand, 1, append(e.G.Zone(state.ZHand, 1), b.ID))
	addMana(t, e, 1, mana)
	return e, o.ID
}

func isWardPayAsk(d *decision.Decision) bool {
	if d == nil || d.Kind != decision.KModes {
		return false
	}
	for _, o := range d.Options {
		if o.Mode == decision.ModeUnlessPay {
			return true
		}
	}
	return false
}

func TestWardPayLifeUnpayableCountersWithoutAsking(t *testing.T) {
	e, p := wardLifeEngine(t, "Prismari, the Inspiration", "Lightning Bolt", "R")
	e.G.Players[1].Life = 3
	bolt := e.G.Zone(state.ZHand, 1)[0]
	opponentBoltAt(t, e, p)
	for i := 0; i < 30 && e.Pending() != nil; i++ {
		d := e.Pending()
		if isWardPayAsk(d) {
			t.Fatalf("unpayable life ward offered Pay: %+v", d.Options)
		}
		wardLifePass(t, e)
	}
	if e.G.Obj(bolt).Zone != state.ZGraveyard || e.G.Obj(p).Damage != 0 || e.G.Players[1].Life != 3 {
		t.Fatalf("bolt zone=%s damage=%d life=%d", e.G.Obj(bolt).Zone, e.G.Obj(p).Damage, e.G.Players[1].Life)
	}
}

func TestWardPayLifePayableStillAsksAndCharges(t *testing.T) {
	e, p := wardLifeEngine(t, "Prismari, the Inspiration", "Lightning Bolt", "R")
	e.G.Players[1].Life = 22
	opponentBoltAt(t, e, p)
	d := e.Pending()
	if !isWardPayAsk(d) {
		t.Fatalf("payable ward must ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Mode == decision.ModeUnlessPay {
			submitChoices(t, e, o.Index)
		}
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Players[1].Life != 17 || e.G.Obj(p).Damage != 3 {
		t.Fatalf("life=%d damage=%d, want 17 and 3", e.G.Players[1].Life, e.G.Obj(p).Damage)
	}
}

func TestWardPayLifePowerUnpayableCountersWithoutAsking(t *testing.T) {
	e, p := wardLifeEngine(t, "Phyrexian Fleshgorger", "Lightning Bolt", "R")
	if e.Power(p) != 7 {
		t.Fatalf("precondition: power %d, want 7", e.Power(p))
	}
	e.G.Players[1].Life = 5
	bolt := e.G.Zone(state.ZHand, 1)[0]
	opponentBoltAt(t, e, p)
	for i := 0; i < 30 && e.Pending() != nil; i++ {
		if d := e.Pending(); isWardPayAsk(d) {
			t.Fatalf("unpayable power ward offered Pay: %+v", d.Options)
		}
		wardLifePass(t, e)
	}
	if e.G.Obj(bolt).Zone != state.ZGraveyard || e.G.Players[1].Life != 5 {
		t.Fatalf("bolt zone=%s life=%d", e.G.Obj(bolt).Zone, e.G.Players[1].Life)
	}
}

func wardLifePass(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "pass" {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no pass option: %+v", d.Options)
}
