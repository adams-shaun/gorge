package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 118.12a / 118.3 (cr-g113): an "unless [player] [cost]" cost outside the
// old strict grammar (tapXType, AddCounter, DamageYou<X>, Waterbend, Blight,
// CollectEvidence, ExileFromStack, a folded Draw<X/...>) is offered whenever
// the payer CAN pay it, and paying it settles through the same payer-choice
// continuation as Sac/Discard. Each test drives the real corpus card through
// the kernel's tape and has a pay row and a decline row.

// ugOffer fails unless the pending decision is the unless-pay ask for seat p
// and returns whether a pay option is offered.
func ugOffer(t *testing.T, e *Engine, p state.PlayerID) bool {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "unless_pay" || d.Player != p {
		t.Fatalf("pending = %s, want the unless-pay ask for seat %d", ugDesc(d), p)
	}
	for _, o := range d.Options {
		if o.Mode == decision.ModeUnlessPay {
			return true
		}
	}
	return false
}

// ugPick answers the pending unless_cost KChoose with the options whose Obj
// is in want (all of them, in offered order, when want is nil).
func ugPick(t *testing.T, e *Engine, kind string, want ...state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "unless_cost" {
		t.Fatalf("pending = %s, want the unless_cost KChoose", ugDesc(d))
	}
	var idx []int
	for _, o := range d.Options {
		if o.Kind != kind {
			t.Fatalf("option kind = %q, want %q", o.Kind, kind)
		}
		if len(want) == 0 {
			idx = append(idx, o.Index)
			continue
		}
		for _, w := range want {
			if o.Obj == w {
				idx = append(idx, o.Index)
			}
		}
	}
	if len(idx) < d.Min || len(idx) > d.Max {
		t.Fatalf("picked %d of %+v, want %d..%d", len(idx), d.Options, d.Min, d.Max)
	}
	submitChoices(t, e, idx...)
}

func ugCorpus(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	return onBoardCard(t, e, p, mustCorpusCard(t, testutil.CorpusRegistry(t), name))
}

// ugPushTrigger puts the source's first trigger on the stack and resolves it
// as a kernel probe, leaving the unless-pay ask pending.
func ugPushTrigger(e *Engine, src state.ObjID, controller state.PlayerID, targets ...state.ObjID) {
	e.emit(events.Event{Kind: events.TriggerPush, Obj: src, Player: controller, Amount: 0})
	if len(targets) > 0 {
		ability := e.G.Stack[len(e.G.Stack)-1]
		e.emit(events.Event{Kind: events.TargetsChosen, Obj: ability, IDs: targets})
	}
	kr9ResolveTop(e)
}

// ugBridge is Command Bridge as it is when its enters-tapped replacement has
// applied: tapped, so it cannot pay its own tap cost.
func ugBridge(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	bridge := ugCorpus(t, e, 0, "Command Bridge")
	e.emit(events.Event{Kind: events.Tap, Obj: bridge})
	return bridge
}

// ugDesc is a compact description of a decision for failure messages.
func ugDesc(d *decision.Decision) string {
	if d == nil {
		return "<none>"
	}
	return fmt.Sprintf("{%s/%s seat %d %q, %d options, %d..%d}", d.Kind, d.ResumeKind, d.Player, d.Prompt, len(d.Options), d.Min, d.Max)
}

func ugZone(e *Engine, id state.ObjID) state.Zone { return e.G.Obj(id).Zone }

// Command Bridge: "sacrifice it unless you tap an untapped permanent you
// control" (CR 701.26a: only an untapped permanent can be tapped).
func TestUnlessTapTypeCommandBridge(t *testing.T) {
	t.Parallel()
	t.Run("pay taps a chosen permanent and the Bridge stays", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8101)
		bridge := ugBridge(t, e)
		bear1 := ugCorpus(t, e, 0, "Grizzly Bears")
		bear2 := ugCorpus(t, e, 0, "Grizzly Bears")
		ugPushTrigger(e, bridge, 0)
		if !ugOffer(t, e, 0) {
			t.Fatal("two untapped permanents, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		ugPick(t, e, "tapcost", bear2)
		if !e.G.Obj(bear2).Tapped || e.G.Obj(bear1).Tapped {
			t.Fatalf("tapped = [%v %v], want only the chosen permanent tapped", e.G.Obj(bear1).Tapped, e.G.Obj(bear2).Tapped)
		}
		if z := ugZone(e, bridge); z != state.ZBattlefield {
			t.Fatalf("Bridge zone = %v, want it kept", z)
		}
	})
	t.Run("a lone candidate is tapped without a pick", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8102)
		bridge := ugBridge(t, e)
		bear := ugCorpus(t, e, 0, "Grizzly Bears")
		ugPushTrigger(e, bridge, 0)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, true)
		if !e.G.Obj(bear).Tapped || ugZone(e, bridge) != state.ZBattlefield {
			t.Fatalf("bear tapped=%v bridge zone=%v, want tapped and kept", e.G.Obj(bear).Tapped, ugZone(e, bridge))
		}
	})
	t.Run("decline sacrifices the Bridge", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8103)
		bridge := ugBridge(t, e)
		bear := ugCorpus(t, e, 0, "Grizzly Bears")
		ugPushTrigger(e, bridge, 0)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		if z := ugZone(e, bridge); z != state.ZGraveyard {
			t.Fatalf("Bridge zone = %v, want sacrificed", z)
		}
		if e.G.Obj(bear).Tapped {
			t.Fatal("a declined payment tapped a permanent")
		}
	})
	t.Run("no untapped permanent offers only the decline", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8104)
		bridge := ugBridge(t, e)
		bear := ugCorpus(t, e, 0, "Grizzly Bears")
		e.emit(events.Event{Kind: events.Tap, Obj: bear})
		ugPushTrigger(e, bridge, 0)
		if ugOffer(t, e, 0) {
			t.Fatal("pay offered with every other permanent tapped (CR 118.3, 701.26a)")
		}
		answerUnlessPay(t, e, false)
		if z := ugZone(e, bridge); z != state.ZGraveyard {
			t.Fatalf("Bridge zone = %v, want sacrificed", z)
		}
	})
}

// ugDriveToUnlessAsk passes priority (and nothing else) until the unless-pay
// ask is pending.
func ugDriveToUnlessAsk(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 60; i++ {
		d := e.Pending()
		if d != nil && d.ResumeKind == "unless_pay" {
			return
		}
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("pending = %s while driving to the unless ask", ugDesc(d))
		}
		submitPass(t, e)
	}
	t.Fatal("no unless ask within 60 passes")
}

// Rent Is Due (SWITCHED): "you may tap two untapped creatures and/or Treasures
// you control. If you do, draw a card. Otherwise, sacrifice this enchantment."
func TestUnlessTapTypeRentIsDue(t *testing.T) {
	t.Parallel()
	t.Run("pay taps both and draws", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8201)
		rent := ugCorpus(t, e, 0, "Rent Is Due")
		b1 := ugCorpus(t, e, 0, "Grizzly Bears")
		b2 := ugCorpus(t, e, 0, "Grizzly Bears")
		before := countDraws(e, 0)
		ugPushTrigger(e, rent, 0)
		if !ugOffer(t, e, 0) {
			t.Fatal("two untapped creatures, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		if !e.G.Obj(b1).Tapped || !e.G.Obj(b2).Tapped {
			t.Fatalf("tapped = [%v %v], want both", e.G.Obj(b1).Tapped, e.G.Obj(b2).Tapped)
		}
		if got := countDraws(e, 0) - before; got != 1 {
			t.Fatalf("draws = %d, want 1", got)
		}
		if z := ugZone(e, rent); z != state.ZBattlefield {
			t.Fatalf("Rent Is Due zone = %v, want kept", z)
		}
	})
	t.Run("decline sacrifices and never draws", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8202)
		rent := ugCorpus(t, e, 0, "Rent Is Due")
		ugCorpus(t, e, 0, "Grizzly Bears")
		ugCorpus(t, e, 0, "Grizzly Bears")
		before := countDraws(e, 0)
		ugPushTrigger(e, rent, 0)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		if got := countDraws(e, 0) - before; got != 0 {
			t.Fatalf("draws = %d, want 0", got)
		}
		if z := ugZone(e, rent); z != state.ZGraveyard {
			t.Fatalf("Rent Is Due zone = %v, want sacrificed", z)
		}
	})
	t.Run("one creature is not two", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8203)
		rent := ugCorpus(t, e, 0, "Rent Is Due")
		ugCorpus(t, e, 0, "Grizzly Bears")
		ugPushTrigger(e, rent, 0)
		if ugOffer(t, e, 0) {
			t.Fatal("pay offered with one creature for a two-permanent tap")
		}
	})
}

// Gutsplitter Gang: "you may blight 2. If you don't, you lose 3 life."
// (CR 701.68a: put two -1/-1 counters on a creature you control.)
func TestUnlessBlightGutsplitter(t *testing.T) {
	t.Parallel()
	t.Run("pay blights the chosen creature and loses no life", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8301)
		gang := ugCorpus(t, e, 0, "Gutsplitter Gang")
		bear := ugCorpus(t, e, 0, "Grizzly Bears")
		life := e.G.Players[0].Life
		ugPushTrigger(e, gang, 0)
		if !ugOffer(t, e, 0) {
			t.Fatal("a creature is controlled, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		ugPick(t, e, "blightcost", gang)
		if got := e.G.Obj(gang).Counter("M1M1"); got != 2 {
			t.Fatalf("Gang -1/-1 counters = %d, want 2", got)
		}
		if got := e.G.Obj(bear).Counter("M1M1"); got != 0 {
			t.Fatalf("Bear -1/-1 counters = %d, want 0", got)
		}
		if got := e.G.Players[0].Life; got != life {
			t.Fatalf("life = %d, want %d", got, life)
		}
	})
	t.Run("decline loses 3 life", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8302)
		gang := ugCorpus(t, e, 0, "Gutsplitter Gang")
		ugCorpus(t, e, 0, "Grizzly Bears")
		life := e.G.Players[0].Life
		ugPushTrigger(e, gang, 0)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		if got := e.G.Players[0].Life; got != life-3 {
			t.Fatalf("life = %d, want %d", got, life-3)
		}
		if got := e.G.Obj(gang).Counter("M1M1"); got != 0 {
			t.Fatalf("a declined blight put %d counters", got)
		}
	})
}

// Combustion Man: "destroy target permanent unless its controller has
// Combustion Man deal damage to them equal to his power." DamageYou<X> folds
// from SVar:X (its CURRENT power), and the offer is any API's.
func TestUnlessDamageYouXCombustionMan(t *testing.T) {
	t.Parallel()
	drive := func(t *testing.T, seed uint64) (*Engine, state.ObjID, state.ObjID) {
		t.Helper()
		e := stealEngine(t, seed)
		man := ugCorpus(t, e, 0, "Combustion Man")
		victim := ugCorpus(t, e, 1, "Grizzly Bears")
		// Pump his power from 4 to 6: the damage is the power NOW.
		e.emit(events.Event{Kind: events.CounterChange, Obj: man, Counter: "P1P1", Amount: 2})
		ugPushTrigger(e, man, 0, victim)
		return e, man, victim
	}
	t.Run("pay takes damage equal to the pumped power and spares the permanent", func(t *testing.T) {
		t.Parallel()
		e, man, victim := drive(t, 8401)
		life := e.G.Players[1].Life
		if !ugOffer(t, e, 1) {
			t.Fatal("no pay option for the damage payment")
		}
		answerUnlessPay(t, e, true)
		if got := e.G.Players[1].Life; got != life-6 {
			t.Fatalf("life = %d, want %d (6 damage: power 4 + two +1/+1 counters)", got, life-6)
		}
		if z := ugZone(e, victim); z != state.ZBattlefield {
			t.Fatalf("target zone = %v, want spared", z)
		}
		if z := ugZone(e, man); z != state.ZBattlefield {
			t.Fatalf("Combustion Man zone = %v", z)
		}
	})
	t.Run("decline destroys the permanent", func(t *testing.T) {
		t.Parallel()
		e, _, victim := drive(t, 8402)
		life := e.G.Players[1].Life
		ugOffer(t, e, 1)
		answerUnlessPay(t, e, false)
		if z := ugZone(e, victim); z != state.ZGraveyard {
			t.Fatalf("target zone = %v, want destroyed", z)
		}
		if got := e.G.Players[1].Life; got != life {
			t.Fatalf("life = %d, want unchanged %d", got, life)
		}
	})
}

// Cheering Crowd (SWITCHED, payer = the triggering player): "that player may
// put a +1/+1 counter on this creature. If they do, they add {C} for each
// counter on it." The payer is the OPPONENT on their turn.
func TestUnlessAddCounterCheeringCrowd(t *testing.T) {
	t.Parallel()
	drive := func(t *testing.T, seed uint64) (*Engine, state.ObjID) {
		t.Helper()
		e := stealEngine(t, seed)
		crowd := ugCorpus(t, e, 0, "Cheering Crowd")
		driveToStep(t, e, 2, 1, state.StepMain1)
		ugDriveToUnlessAsk(t, e)
		return e, crowd
	}
	t.Run("pay puts the counter on the Crowd and the payer adds the mana", func(t *testing.T) {
		t.Parallel()
		e, crowd := drive(t, 8501)
		if !ugOffer(t, e, 1) {
			t.Fatal("no pay option for the triggering player")
		}
		answerUnlessPay(t, e, true)
		if got := e.G.Obj(crowd).Counter("P1P1"); got != 1 {
			t.Fatalf("Crowd +1/+1 counters = %d, want 1", got)
		}
		if got := e.G.Players[1].Pool[state.MC]; got != 1 {
			t.Fatalf("seat 1 {C} = %d, want 1 (one counter)", got)
		}
		if got := e.G.Players[0].Pool[state.MC]; got != 0 {
			t.Fatalf("seat 0 {C} = %d, want 0", got)
		}
	})
	t.Run("decline puts no counter and adds no mana", func(t *testing.T) {
		t.Parallel()
		e, crowd := drive(t, 8502)
		ugOffer(t, e, 1)
		answerUnlessPay(t, e, false)
		if got := e.G.Obj(crowd).Counter("P1P1"); got != 0 {
			t.Fatalf("Crowd +1/+1 counters = %d, want 0", got)
		}
		if got := e.G.Players[1].Pool[state.MC]; got != 0 {
			t.Fatalf("seat 1 {C} = %d, want 0", got)
		}
	})
}

// ugPlace puts a corpus card straight into zone z of seat p (eventless, like
// onBoardCard) and returns its id.
func ugPlace(t *testing.T, e *Engine, p state.PlayerID, name string, z state.Zone) state.ObjID {
	t.Helper()
	o := e.G.AddObject(mustCorpusCard(t, testutil.CorpusRegistry(t), name), p)
	o.Zone = z
	e.G.SetZone(z, p, append(e.G.Zone(z, p), o.ID))
	return o.ID
}

// ugCast submits the cast option for id at seat 0's priority (after the
// fixture's eventless placement is made visible) and passes until the unless
// ask is pending.
func ugCast(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.pending = nil
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %s", ugDesc(d))
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			submitChoices(t, e, o.Index)
			ugDriveToUnlessAsk(t, e)
			return
		}
	}
	t.Fatalf("no cast option for %d: %+v", id, d.Options)
}

// Waterbending Lesson: "Discard a card unless you waterbend {2}." (CR
// 701.67a: pay {2}; each untapped artifact or creature tapped pays {1}.)
func TestUnlessWaterbendLesson(t *testing.T) {
	t.Parallel()
	drive := func(t *testing.T, seed uint64, floating string) (*Engine, state.ObjID) {
		t.Helper()
		e := stealEngine(t, seed)
		bear := ugCorpus(t, e, 0, "Grizzly Bears")
		lesson := ugPlace(t, e, 0, "Waterbending Lesson", state.ZHand)
		addMana(t, e, 0, "UUUU"+floating)
		ugCast(t, e, lesson)
		return e, bear
	}
	t.Run("pay: tap one creature and pay {1}, no discard", func(t *testing.T) {
		t.Parallel()
		e, bear := drive(t, 8601, "U")
		hand := len(e.G.Zone(state.ZHand, 0))
		if !ugOffer(t, e, 0) {
			t.Fatal("one creature plus {1} floating covers waterbend {2}, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		if !e.G.Obj(bear).Tapped {
			t.Fatal("the creature was not tapped for waterbend")
		}
		if got := e.G.Players[0].Pool.Total(); got != 0 {
			t.Fatalf("floating mana = %d, want 0 (the {1} was paid)", got)
		}
		if d := e.Pending(); d != nil && d.ResumeKind == "discard" {
			t.Fatalf("pending = %s, want no discard ask after paying", ugDesc(d))
		}
		if got := len(e.G.Zone(state.ZHand, 0)); got != hand {
			t.Fatalf("hand = %d, want %d (no discard)", got, hand)
		}
	})
	t.Run("decline discards a card", func(t *testing.T) {
		t.Parallel()
		e, bear := drive(t, 8602, "U")
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		if e.G.Obj(bear).Tapped {
			t.Fatal("a declined waterbend tapped a creature")
		}
		d := e.Pending()
		if d == nil || d.ResumeKind != "discard" {
			t.Fatalf("pending = %s, want the discard choice", ugDesc(d))
		}
		discarded := d.Options[0].Obj
		submitChoices(t, e, d.Options[0].Index)
		if z := ugZone(e, discarded); z != state.ZGraveyard {
			t.Fatalf("discarded card zone = %v, want the graveyard", z)
		}
	})
	t.Run("pool and one creature short of {2} offers only the decline", func(t *testing.T) {
		t.Parallel()
		e, _ := drive(t, 8603, "")
		if ugOffer(t, e, 0) {
			t.Fatal("pay offered with {0} floating and one creature for waterbend {2}")
		}
	})
}

// Sanctuary Wall (SWITCHED): "You may put a stun counter on [the targeted
// creature]. If you do, put a stun counter on CARDNAME."
func TestUnlessAddCounterSanctuaryWall(t *testing.T) {
	t.Parallel()
	drive := func(t *testing.T, seed uint64) (*Engine, state.ObjID, state.ObjID) {
		t.Helper()
		e := stealEngine(t, seed)
		wall := ugCorpus(t, e, 0, "Sanctuary Wall")
		e.G.Obj(wall).SummonSick = false
		target := ugCorpus(t, e, 1, "Grizzly Bears")
		addMana(t, e, 0, "WWW")
		d := e.Pending()
		for _, o := range d.Options {
			if o.Kind == "ability" && o.Obj == wall {
				submitChoices(t, e, o.Index)
				break
			}
		}
		d = e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("pending = %s, want the target ask", ugDesc(d))
		}
		for _, o := range d.Options {
			if o.Obj == target {
				submitChoices(t, e, o.Index)
				break
			}
		}
		ugDriveToUnlessAsk(t, e)
		return e, wall, target
	}
	t.Run("pay stuns the target, then the Wall", func(t *testing.T) {
		t.Parallel()
		e, wall, target := drive(t, 8701)
		if !ugOffer(t, e, 0) {
			t.Fatal("no pay option for the stun counter")
		}
		answerUnlessPay(t, e, true)
		if got := e.G.Obj(target).Counter("STUN"); got != 1 {
			t.Fatalf("target stun counters = %d, want 1", got)
		}
		if got := e.G.Obj(wall).Counter("STUN"); got != 1 {
			t.Fatalf("Wall stun counters = %d, want 1", got)
		}
		if !e.G.Obj(target).Tapped {
			t.Fatal("the target was not tapped")
		}
	})
	t.Run("decline puts no counter on either", func(t *testing.T) {
		t.Parallel()
		e, wall, target := drive(t, 8702)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		if a, b := e.G.Obj(target).Counter("STUN"), e.G.Obj(wall).Counter("STUN"); a != 0 || b != 0 {
			t.Fatalf("stun counters = target %d, Wall %d, want none", a, b)
		}
	})
}

// Sphinx's Approach (SWITCHED): "you may exile this spell and four cards
// named Sphinx's Approach from your graveyard. If you do, search your library
// for a Sphinx creature card, put it onto the battlefield." The pay option is
// offered only while four named copies sit in the graveyard (CR 118.3).
func TestUnlessExileFromStackSphinxsApproach(t *testing.T) {
	t.Parallel()
	drive := func(t *testing.T, seed uint64, copies int) (*Engine, state.ObjID, []state.ObjID, state.ObjID) {
		t.Helper()
		e := stealEngine(t, seed)
		sphinx := ugPlace(t, e, 0, "Sphinx of the Steel Wind", state.ZLibrary)
		var graves []state.ObjID
		for range copies {
			graves = append(graves, ugPlace(t, e, 0, "Sphinx's Approach", state.ZGraveyard))
		}
		spell := ugPlace(t, e, 0, "Sphinx's Approach", state.ZHand)
		addMana(t, e, 0, "UUU")
		ugCast(t, e, spell)
		return e, spell, graves, sphinx
	}
	t.Run("pay exiles the spell and four copies and searches", func(t *testing.T) {
		t.Parallel()
		e, spell, graves, sphinx := drive(t, 8801, 4)
		if !ugOffer(t, e, 0) {
			t.Fatal("four copies in the graveyard, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		for d := e.Pending(); d != nil && d.Kind == decision.KChoose; d = e.Pending() {
			// The search selection (and the shuffle-free tail).
			submitChoices(t, e, d.Options[0].Index)
		}
		for _, g := range graves {
			if z := ugZone(e, g); z != state.ZExile {
				t.Fatalf("graveyard copy zone = %v, want exiled", z)
			}
		}
		if z := ugZone(e, spell); z != state.ZExile {
			t.Fatalf("the spell's zone = %v, want exiled as part of the cost", z)
		}
		if z := ugZone(e, sphinx); z != state.ZBattlefield {
			t.Fatalf("Sphinx zone = %v, want put onto the battlefield", z)
		}
	})
	t.Run("decline: the spell goes to the graveyard, no search", func(t *testing.T) {
		t.Parallel()
		e, spell, graves, sphinx := drive(t, 8802, 4)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		passUntilStackEmpty(t, e, 20)
		if z := ugZone(e, sphinx); z != state.ZLibrary {
			t.Fatalf("Sphinx zone = %v, want it still in the library", z)
		}
		for _, g := range graves {
			if z := ugZone(e, g); z != state.ZGraveyard {
				t.Fatalf("graveyard copy zone = %v, want untouched", z)
			}
		}
		if z := ugZone(e, spell); z != state.ZGraveyard {
			t.Fatalf("the spell's zone = %v, want the graveyard", z)
		}
	})
	t.Run("three copies: the pay option is not offered", func(t *testing.T) {
		t.Parallel()
		e, _, _, _ := drive(t, 8803, 3)
		if ugOffer(t, e, 0) {
			t.Fatal("pay offered with three of four copies in the graveyard")
		}
	})
}

// Armor Wars chapter I (SWITCHED): "You may draw a card for each artifact you
// control. If you do, each opponent draws a card." Draw<X/You> folds from
// SVar:X, the artifact count.
func TestUnlessDrawXArmorWars(t *testing.T) {
	t.Parallel()
	drive := func(t *testing.T, seed uint64, artifacts int) *Engine {
		t.Helper()
		e := stealEngine(t, seed)
		saga := ugCorpus(t, e, 0, "Armor Wars")
		for range artifacts {
			onBoard(t, e, 0, "Name:Trinket\nTypes:Artifact\nOracle:x\n")
		}
		e.emit(events.Event{Kind: events.CounterChange, Obj: saga, Counter: "LORE", Amount: 1})
		e.pending = nil
		e.priorityRound()
		ugDriveToUnlessAsk(t, e)
		return e
	}
	t.Run("pay draws one card per artifact and each opponent draws one", func(t *testing.T) {
		t.Parallel()
		e := drive(t, 8901, 3)
		you, opp := countDraws(e, 0), countDraws(e, 1)
		if !ugOffer(t, e, 0) {
			t.Fatal("three artifacts, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		if got := countDraws(e, 0) - you; got != 3 {
			t.Fatalf("you drew %d, want 3 (one per artifact)", got)
		}
		if got := countDraws(e, 1) - opp; got != 1 {
			t.Fatalf("opponent drew %d, want 1", got)
		}
	})
	t.Run("decline draws nothing", func(t *testing.T) {
		t.Parallel()
		e := drive(t, 8902, 3)
		you, opp := countDraws(e, 0), countDraws(e, 1)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		if a, b := countDraws(e, 0)-you, countDraws(e, 1)-opp; a != 0 || b != 0 {
			t.Fatalf("draws = you %d, opponent %d, want none", a, b)
		}
	})
	t.Run("no artifact: there is nothing to pay", func(t *testing.T) {
		t.Parallel()
		e := drive(t, 8903, 0)
		if ugOffer(t, e, 0) {
			t.Fatal("pay offered with zero artifacts")
		}
	})
}

// ugTrigFixture is an inline permanent whose first trigger resolves one
// switched Draw gated by cost (paying causes the draw).
func ugTrigFixture(cost string) string {
	return "Name:Cost Probe\nTypes:Creature Probe\nPT:1/1\n" +
		"T:Mode$ Phase | Phase$ End of Turn | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigDraw | TriggerDescription$ probe\n" +
		"SVar:TrigDraw:DB$ Draw | UnlessCost$ " + cost + " | UnlessPayer$ You | UnlessSwitched$ True\nOracle:x\n"
}

// Waterbend with an empty pool owes every {1} as a tap: the payer picks which
// permanents (CR 701.67a), and only artifacts and creatures qualify.
func TestUnlessWaterbendPicksTaps(t *testing.T) {
	t.Parallel()
	e := stealEngine(t, 8611)
	probe := onBoard(t, e, 0, ugTrigFixture("Waterbend<2>"))
	b1 := ugCorpus(t, e, 0, "Grizzly Bears")
	b2 := ugCorpus(t, e, 0, "Grizzly Bears")
	b3 := ugCorpus(t, e, 0, "Grizzly Bears")
	land := ugCorpus(t, e, 0, "Command Bridge")
	before := countDraws(e, 0)
	ugPushTrigger(e, probe, 0)
	if !ugOffer(t, e, 0) {
		t.Fatal("four creatures cover waterbend {2}, yet no pay option")
	}
	answerUnlessPay(t, e, true)
	d := e.Pending()
	if d == nil || d.Min != 2 || d.Max != 2 || len(d.Options) != 4 {
		t.Fatalf("pending = %s, want a pick of exactly 2 among the 4 creatures", ugDesc(d))
	}
	ugPick(t, e, "waterbend_generic", b1, b3)
	if !e.G.Obj(b1).Tapped || e.G.Obj(b2).Tapped || !e.G.Obj(b3).Tapped || e.G.Obj(land).Tapped {
		t.Fatal("waterbend must tap exactly the two chosen creatures")
	}
	if got := countDraws(e, 0) - before; got != 1 {
		t.Fatalf("draws = %d, want 1 (the switched body ran on pay)", got)
	}
}

// CollectEvidence<4> (CR 701.59a): exile cards from your graveyard with total
// mana value 4 or more. The pay option needs the total to be reachable.
func TestUnlessCollectEvidence(t *testing.T) {
	t.Parallel()
	drive := func(t *testing.T, seed uint64, bears int) (*Engine, []state.ObjID) {
		t.Helper()
		e := stealEngine(t, seed)
		probe := onBoard(t, e, 0, ugTrigFixture("CollectEvidence<4>"))
		var graves []state.ObjID
		for range bears {
			graves = append(graves, ugPlace(t, e, 0, "Grizzly Bears", state.ZGraveyard))
		}
		ugPushTrigger(e, probe, 0)
		return e, graves
	}
	t.Run("pay exiles cards whose total mana value reaches 4", func(t *testing.T) {
		t.Parallel()
		e, graves := drive(t, 8631, 3)
		before := countDraws(e, 0)
		if !ugOffer(t, e, 0) {
			t.Fatal("6 mana value in the graveyard, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		d := e.Pending()
		if d == nil || d.Min != 2 || d.Max != 3 {
			t.Fatalf("pending = %s, want a pick of 2..3 cards", ugDesc(d))
		}
		ugPick(t, e, "evidence", graves[0], graves[2])
		if ugZone(e, graves[0]) != state.ZExile || ugZone(e, graves[2]) != state.ZExile || ugZone(e, graves[1]) != state.ZGraveyard {
			t.Fatal("evidence must exile exactly the two chosen cards")
		}
		if got := countDraws(e, 0) - before; got != 1 {
			t.Fatalf("draws = %d, want 1", got)
		}
	})
	t.Run("a pick below the total is a decline", func(t *testing.T) {
		t.Parallel()
		e, graves := drive(t, 8632, 3)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, true)
		d := e.Pending()
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err == nil {
			for _, g := range graves {
				if ugZone(e, g) != state.ZGraveyard {
					t.Fatal("an under-total evidence pick exiled a card")
				}
			}
		}
	})
	t.Run("decline exiles nothing", func(t *testing.T) {
		t.Parallel()
		e, graves := drive(t, 8633, 3)
		before := countDraws(e, 0)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		for _, g := range graves {
			if ugZone(e, g) != state.ZGraveyard {
				t.Fatal("a declined evidence exiled a card")
			}
		}
		if got := countDraws(e, 0) - before; got != 0 {
			t.Fatalf("draws = %d, want 0", got)
		}
	})
	t.Run("one mana value 2 card is not 4", func(t *testing.T) {
		t.Parallel()
		e, _ := drive(t, 8634, 1)
		if ugOffer(t, e, 0) {
			t.Fatal("pay offered with total mana value 2 for CollectEvidence<4>")
		}
	})
}

// AddCounter<1/M1M1/Creature.YouCtrl> (Thelon's Chant's shape): the payer
// chooses which of their creatures takes the counter.
func TestUnlessAddCounterChoosesRecipient(t *testing.T) {
	t.Parallel()
	e := stealEngine(t, 8651)
	probe := onBoard(t, e, 0, ugTrigFixture("AddCounter<1/M1M1/Creature.YouCtrl/a creature you control>"))
	bear := ugCorpus(t, e, 0, "Grizzly Bears")
	theirs := ugCorpus(t, e, 1, "Grizzly Bears")
	before := countDraws(e, 0)
	ugPushTrigger(e, probe, 0)
	if !ugOffer(t, e, 0) {
		t.Fatal("no pay option")
	}
	answerUnlessPay(t, e, true)
	d := e.Pending()
	if d == nil || len(d.Options) != 2 {
		t.Fatalf("pending = %s, want the probe and our Bear as recipients (not the opponent's)", ugDesc(d))
	}
	ugPick(t, e, "countercost", bear)
	if got := e.G.Obj(bear).Counter("M1M1"); got != 1 {
		t.Fatalf("Bear -1/-1 counters = %d, want 1", got)
	}
	if got := e.G.Obj(theirs).Counter("M1M1"); got != 0 {
		t.Fatalf("opponent's Bear got %d counters", got)
	}
	if got := countDraws(e, 0) - before; got != 1 {
		t.Fatalf("draws = %d, want 1", got)
	}
}
