package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// selectionStage builds a two-seat staged position (no ETB triggers) and
// returns the exact Config that replays its setup events. NewStaged builds its
// decks from the Stage itself, so the caller's Config needs the same deck
// order for replayCheck.
func selectionStage(t *testing.T, reg *cards.Registry, st Stage) (*Engine, StagedObjects, Config) {
	t.Helper()
	cfg := Config{Seed: 1, Names: []string{"a", "b"}, Tokens: reg.Tokens}
	decks := make([][]*cards.Card, 2)
	for p := range st.Players {
		sp := st.Players[p]
		for _, zone := range [][]*cards.Card{sp.Library, sp.Hand, sp.Graveyard, sp.Exile} {
			decks[p] = append(decks[p], zone...)
		}
	}
	for _, pm := range st.Permanents {
		if pm.Card != nil {
			decks[pm.Owner] = append(decks[pm.Owner], pm.Card)
		}
	}
	e, ids, err := NewStaged(cfg, st)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	cfg.Decks = decks
	return e, ids, cfg
}

func selectionSVars(face *cards.Face) map[string]string {
	sv := make(map[string]string, len(face.SVars))
	for k, v := range face.SVars {
		sv[k] = v
	}
	return sv
}

func selectionCard(t *testing.T, reg *cards.Registry, name string) *cards.Card {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("card %q not in the corpus", name)
	}
	return c
}

func selectionOptionObj(d *decision.Decision, index int) state.ObjID {
	for _, o := range d.Options {
		if o.Index == index {
			return o.Obj
		}
	}
	return 0
}

func selectionOptionsWithPower(d *decision.Decision, power string) []int {
	var out []int
	for _, o := range d.Options {
		if len(o.SetProps) == 1 && o.SetProps[0] == power {
			out = append(out, o.Index)
		}
	}
	return out
}

func selectionNoteIDs(e *Engine, player state.PlayerID, ids []state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Player == player && !ev.Secret && sameObjIDs(ev.IDs, ids) {
			return true
		}
	}
	return false
}

func selectionNoteContains(e *Engine, substr string) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, substr) {
			return true
		}
	}
	return false
}

func selectionHasID(ids []state.ObjID, id state.ObjID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestRipDifferentPowersRejectsEqualPowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rip := selectionCard(t, reg, "Rip, Spawn Hunter")
	hill := selectionCard(t, reg, "Hill Giant")
	centaur := selectionCard(t, reg, "Centaur Courser")
	grizzly := selectionCard(t, reg, "Grizzly Bears")
	llanowar := selectionCard(t, reg, "Llanowar Elves")

	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain2,
		Players: []StagedPlayer{
			{Life: 20, Library: []*cards.Card{hill, centaur, grizzly, llanowar}},
			{Life: 20},
		},
		Permanents: []StagedPermanent{{Card: rip, Owner: 0, Controller: 0, Tapped: true}},
	})
	ripID := ids.Permanents[0]
	if o := e.G.Obj(ripID); o == nil || !o.Tapped || e.G.Step != state.StepMain2 {
		t.Fatalf("precondition: tapped %v step %v", o != nil && o.Tapped, e.G.Step)
	}

	sa := cards.ResolveSVar(e.G.Obj(ripID).Face().SVars, "TrigDig")
	d := kr0Run(t, e, sa, func() *effects.Ctx {
		return &effects.Ctx{Source: ripID, Controller: 0, SVars: selectionSVars(e.G.Obj(ripID).Face())}
	}, nil)
	if d == nil || d.ResumeKind != "dig" {
		t.Fatalf("dig decision %+v", d)
	}
	if d.SetPropMode != decision.SetPropDistinct {
		t.Fatalf("SetPropMode %q, want distinct", d.SetPropMode)
	}
	p3 := selectionOptionsWithPower(d, "3")
	p2 := selectionOptionsWithPower(d, "2")
	if len(p3) != 2 || len(p2) != 1 {
		t.Fatalf("powers: three=%v two=%v options=%+v", p3, p2, d.Options)
	}
	chosen := []state.ObjID{selectionOptionObj(d, p3[0]), selectionOptionObj(d, p2[0])}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: p3}); err == nil {
		t.Fatal("two power-3 cards were accepted under WithDifferentPowers")
	}
	if e.Pending() == nil {
		t.Fatal("the rejected intent consumed the pending decision")
	}
	kr0Answer(t, e, p3[0], p2[0])

	hand := e.G.Zone(state.ZHand, 0)
	if !zoneContains(hand, chosen[0]) || !zoneContains(hand, chosen[1]) {
		t.Fatalf("hand %v, want the two chosen cards %v", hand, chosen)
	}
	replayCheck(t, e, cfg)
}

func TestKlawRevealNumberTargetChoosesReveal(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	klaw := selectionCard(t, reg, "Klaw, Sonic Subjugator")
	grizzly := selectionCard(t, reg, "Grizzly Bears")
	hill := selectionCard(t, reg, "Hill Giant")
	centaur := selectionCard(t, reg, "Centaur Courser")
	llanowar := selectionCard(t, reg, "Llanowar Elves")

	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain1,
		Players: []StagedPlayer{
			{Life: 20, Graveyard: []*cards.Card{grizzly}},
			{Life: 20, Hand: []*cards.Card{hill, centaur, grizzly, llanowar}},
		},
		Permanents: []StagedPermanent{{Card: klaw, Owner: 0, Controller: 0}},
	})
	klawID := ids.Permanents[0]
	sa := cards.ResolveSVar(e.G.Obj(klawID).Face().SVars, "TrigDiscard")
	d := kr0Run(t, e, sa, func() *effects.Ctx {
		return &effects.Ctx{
			Source:     klawID,
			Controller: 0,
			SVars:      selectionSVars(e.G.Obj(klawID).Face()),
			Targets:    []state.Target{{Player: 1, IsPlayer: true}},
			OfferedSA:  sa,
		}
	}, nil)
	if d == nil || d.Player != 1 || d.Kind != decision.KChoose || d.Min != 2 || d.Max != 2 || d.ResumeKind != "discard_reveal" {
		t.Fatalf("reveal decision %+v", d)
	}
	if len(d.Options) != 4 {
		t.Fatalf("reveal options %d, want 4: %+v", len(d.Options), d.Options)
	}
	chosen := []state.ObjID{d.Options[0].Obj, d.Options[2].Obj}
	if chosen[0] == chosen[1] {
		t.Fatal("fixture picked the same card twice")
	}
	d2 := kr0Answer(t, e, d.Options[0].Index, d.Options[2].Index)
	if d2 == nil || d2.Player != 0 || d2.Kind != decision.KModes || d2.ResumeKind != "discard" {
		t.Fatalf("discard decision %+v", d2)
	}
	var offered []state.ObjID
	for _, o := range d2.Options {
		offered = append(offered, o.Obj)
	}
	if len(offered) != 2 || !zoneContains(offered, chosen[0]) || !zoneContains(offered, chosen[1]) {
		t.Fatalf("discard pool %v, want exactly the revealed %v", offered, chosen)
	}
	if !selectionNoteIDs(e, 1, chosen) {
		t.Fatalf("no public reveal Note naming %v", chosen)
	}
	discardIdx := d2.Options[0].Index
	kr0Answer(t, e, discardIdx)
	if e.G.Obj(chosen[0]).Zone != state.ZGraveyard {
		t.Fatalf("chosen card %d in %v, want graveyard", chosen[0], e.G.Obj(chosen[0]).Zone)
	}
	if e.G.Obj(chosen[1]).Zone != state.ZHand {
		t.Fatalf("unrevealed card %d in %v, want hand", chosen[1], e.G.Obj(chosen[1]).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestGhostVacuumReturnsWithFlyingCounter(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	gv := selectionCard(t, reg, "Ghost Vacuum")
	ozolith := selectionCard(t, reg, "The Ozolith")
	hill := selectionCard(t, reg, "Hill Giant")
	centaur := selectionCard(t, reg, "Centaur Courser")

	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain1,
		Players: []StagedPlayer{
			{Life: 20, Exile: []*cards.Card{hill, centaur}},
			{Life: 20},
		},
		Permanents: []StagedPermanent{
			{Card: gv, Owner: 0, Controller: 0},
			{Card: ozolith, Owner: 0, Controller: 0},
		},
	})
	gvID := ids.Permanents[0]
	ozolithID := ids.Permanents[1]
	exiled := append([]state.ObjID(nil), ids.Exile[0]...)
	e.emit(events.Event{Kind: events.Imprint, Obj: gvID, IDs: exiled, Text: "exiled-with"})

	o := e.G.Obj(gvID)
	if len(o.Face().Abilities) < 2 {
		t.Fatalf("Ghost Vacuum abilities %d", len(o.Face().Abilities))
	}
	sa := o.Face().Abilities[1]
	if sa.API != "ChangeZoneAll" {
		t.Fatalf("ability[1] is %q, want ChangeZoneAll", sa.API)
	}
	kr0Run(t, e, sa, func() *effects.Ctx {
		return &effects.Ctx{Source: gvID, Controller: 0, SVars: selectionSVars(o.Face())}
	}, nil)

	for _, id := range exiled {
		got := e.G.Obj(id)
		if got == nil || got.Zone != state.ZBattlefield || got.Controller != 0 {
			t.Fatalf("exiled card %d: %+v, want battlefield under seat 0", id, got)
		}
		if got.Counter("Flying") != 1 {
			t.Fatalf("exiled card %d has %d Flying counters, want 1", id, got.Counter("Flying"))
		}
	}
	// The Ozolith's leaves-the-battlefield trigger reads the returned card's
	// counters ("if it had counters on it"), proving the entry event carried
	// the flying counter into the trigger's view.
	e.emit(events.Event{Kind: events.MoveZone, Obj: exiled[0], From: state.ZBattlefield, To: state.ZGraveyard})
	e.putTriggersOnStack()
	if len(e.G.Stack) == 0 {
		t.Fatal("The Ozolith's counters trigger did not queue")
	}
	e.resolveTop()
	if got := e.G.Obj(ozolithID).Counter("Flying"); got != 1 {
		t.Fatalf("The Ozolith has %d Flying counters, want 1 copied from the returned card", got)
	}
	replayCheck(t, e, cfg)
}

func TestFightExcessBindsRhinosRampage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rhino := selectionCard(t, reg, "Rhino's Rampage")
	air := selectionCard(t, reg, "Air Elemental")
	grizzly := selectionCard(t, reg, "Grizzly Bears")
	bonesplitter := selectionCard(t, reg, "Bonesplitter")

	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain1,
		Players: []StagedPlayer{
			{Life: 20, Graveyard: []*cards.Card{rhino}},
			{Life: 20},
		},
		Permanents: []StagedPermanent{
			{Card: air, Owner: 0, Controller: 0},
			{Card: grizzly, Owner: 1, Controller: 1, Sick: true},
			{Card: bonesplitter, Owner: 1, Controller: 1, Sick: true},
		},
	})
	rhinoID := ids.Graveyard[0][0]
	fighter := ids.Permanents[0]
	victim := ids.Permanents[1]
	artifact := ids.Permanents[2]
	face := e.G.Obj(rhinoID).Face()
	var last *effects.Ctx
	pumpSA := face.Abilities[0]
	d := kr0Run(t, e, pumpSA, func() *effects.Ctx {
		return &effects.Ctx{
			Source:     rhinoID,
			Controller: 0,
			SVars:      selectionSVars(face),
			Targets:    []state.Target{{Obj: fighter}},
			OfferedSA:  pumpSA,
			SubPreAsk:  map[string][]state.Target{pumpSA.Sub.Line: {{Obj: victim}}},
		}
	}, &last)
	if last == nil || last.SVars["Excess"] != "3" {
		t.Fatalf("Excess %q, want 3", last.SVars["Excess"])
	}
	if e.Power(fighter) != 5 || e.G.Obj(victim).Damage != 5 {
		t.Fatalf("fighter power %d victim damage %d, want 5/5", e.Power(fighter), e.G.Obj(victim).Damage)
	}
	if d != nil {
		if d.Kind != decision.KChoose {
			t.Fatalf("excess decision %+v", d)
		}
		kr0Answer(t, e, kr0Opt(t, d, artifact))
	} else {
		if len(e.pendingTriggers) == 0 {
			t.Fatal("the reflexive destroy trigger was not queued")
		}
		e.putTriggersOnStack()
		kr0Answer(t, e, kr0Opt(t, e.Pending(), artifact))
		passUntilStackEmpty(t, e, 20)
	}
	if e.G.Obj(artifact).Zone != state.ZGraveyard {
		t.Fatalf("artifact in %v, want graveyard", e.G.Obj(artifact).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestFightExcessNoExcessDoesNotTrigger(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rhino := selectionCard(t, reg, "Rhino's Rampage")
	grizzly := selectionCard(t, reg, "Grizzly Bears")
	hill := selectionCard(t, reg, "Hill Giant")
	bonesplitter := selectionCard(t, reg, "Bonesplitter")

	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain1,
		Players: []StagedPlayer{
			{Life: 20, Graveyard: []*cards.Card{rhino}},
			{Life: 20},
		},
		Permanents: []StagedPermanent{
			{Card: grizzly, Owner: 0, Controller: 0},
			{Card: hill, Owner: 1, Controller: 1, Sick: true},
			{Card: bonesplitter, Owner: 1, Controller: 1, Sick: true},
		},
	})
	rhinoID := ids.Graveyard[0][0]
	fighter := ids.Permanents[0]
	victim := ids.Permanents[1]
	artifact := ids.Permanents[2]
	face := e.G.Obj(rhinoID).Face()
	var last *effects.Ctx
	pumpSA := face.Abilities[0]
	d := kr0Run(t, e, pumpSA, func() *effects.Ctx {
		return &effects.Ctx{
			Source:     rhinoID,
			Controller: 0,
			SVars:      selectionSVars(face),
			Targets:    []state.Target{{Obj: fighter}},
			OfferedSA:  pumpSA,
			SubPreAsk:  map[string][]state.Target{pumpSA.Sub.Line: {{Obj: victim}}},
		}
	}, &last)
	if d != nil {
		t.Fatalf("unexpected decision %+v", d)
	}
	if last == nil || last.SVars["Excess"] != "0" {
		t.Fatalf("Excess %q, want 0", last.SVars["Excess"])
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("queued %d triggers with no excess", len(e.pendingTriggers))
	}
	if e.G.Obj(artifact).Zone != state.ZBattlefield {
		t.Fatalf("artifact in %v, want battlefield", e.G.Obj(artifact).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestRepeatEachStartingWithYou(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	yangchen := selectionCard(t, reg, "The Legend of Yangchen")
	first := selectionCard(t, reg, "Hill Giant")
	second := selectionCard(t, reg, "Centaur Courser")
	third := selectionCard(t, reg, "Grizzly Bears")

	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 1, Starting: 1, Step: state.StepMain1,
		Players: []StagedPlayer{{Life: 20}, {Life: 20}},
		Permanents: []StagedPermanent{
			{Card: yangchen, Owner: 1, Controller: 1},
			{Card: first, Owner: 0, Controller: 0, Sick: true},
			{Card: second, Owner: 0, Controller: 0, Sick: true},
			{Card: third, Owner: 1, Controller: 1},
		},
	})
	yangchenID := ids.Permanents[0]
	face := e.G.Obj(yangchenID).Face()
	sa := cards.ResolveSVar(face.SVars, "DBRepeatEach")
	d := kr0Run(t, e, sa, func() *effects.Ctx {
		return &effects.Ctx{Source: yangchenID, Controller: 1, SVars: selectionSVars(face)}
	}, nil)
	if selectionNoteContains(e, "StartingWith") {
		t.Fatal("StartingWith$ is still reported unread")
	}
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("first choice %+v", d)
	}
	if d.Player != 1 {
		t.Fatalf("first chooser is seat %d, want the controller seat 1 (StartingWith$ You)", d.Player)
	}

	var chosen []state.ObjID
	for i := 0; i < 2; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			t.Fatalf("iteration %d choice %+v", i, d)
		}
		if i == 1 && d.Player != 0 {
			t.Fatalf("second chooser is seat %d, want seat 0", d.Player)
		}
		pick := d.Options[0]
		for _, o := range d.Options {
			if !selectionHasID(chosen, o.Obj) {
				pick = o
				break
			}
		}
		chosen = append(chosen, pick.Obj)
		kr0Answer(t, e, pick.Index)
	}
	for _, id := range chosen {
		if e.G.Obj(id).Zone != state.ZExile {
			t.Fatalf("chosen permanent %d in %v, want exile", id, e.G.Obj(id).Zone)
		}
	}
	if e.G.Obj(yangchenID).Zone != state.ZBattlefield {
		t.Fatalf("Yangchen in %v, want battlefield", e.G.Obj(yangchenID).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestKlawRevealNumberCoversHand(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	klaw := selectionCard(t, reg, "Klaw, Sonic Subjugator")
	grizzly := selectionCard(t, reg, "Grizzly Bears")
	hill := selectionCard(t, reg, "Hill Giant")
	centaur := selectionCard(t, reg, "Centaur Courser")
	llanowar := selectionCard(t, reg, "Llanowar Elves")

	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain1,
		Players: []StagedPlayer{
			{Life: 20, Graveyard: []*cards.Card{grizzly, hill, centaur}},
			{Life: 20, Hand: []*cards.Card{hill, centaur, grizzly, llanowar}},
		},
		Permanents: []StagedPermanent{{Card: klaw, Owner: 0, Controller: 0}},
	})
	klawID := ids.Permanents[0]
	sa := cards.ResolveSVar(e.G.Obj(klawID).Face().SVars, "TrigDiscard")
	d := kr0Run(t, e, sa, func() *effects.Ctx {
		return &effects.Ctx{
			Source:     klawID,
			Controller: 0,
			SVars:      selectionSVars(e.G.Obj(klawID).Face()),
			Targets:    []state.Target{{Player: 1, IsPlayer: true}},
			OfferedSA:  sa,
		}
	}, nil)
	if d == nil || d.Player != 0 || d.Kind != decision.KModes || d.ResumeKind != "discard" {
		t.Fatalf("first decision %+v, want the caster's discard over the whole hand", d)
	}
	if len(d.Options) != 4 {
		t.Fatalf("discard options %d, want the whole 4-card hand: %+v", len(d.Options), d.Options)
	}
	if selectionNoteContains(e, "reveal") {
		t.Fatal("N covering the hand must not ask for or note a reveal")
	}
	discarded := d.Options[0].Obj
	kr0Answer(t, e, d.Options[0].Index)
	if e.G.Obj(discarded).Zone != state.ZGraveyard {
		t.Fatalf("discarded card %d in %v, want graveyard", discarded, e.G.Obj(discarded).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestFightExcessUsesLandedDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rhino := selectionCard(t, reg, "Rhino's Rampage")
	sengir := selectionCard(t, reg, "Sengir Vampire")
	cho := selectionCard(t, reg, "Cho-Manno, Revolutionary")
	bonesplitter := selectionCard(t, reg, "Bonesplitter")

	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain1,
		Players: []StagedPlayer{
			{Life: 20, Graveyard: []*cards.Card{rhino}},
			{Life: 20},
		},
		Permanents: []StagedPermanent{
			{Card: sengir, Owner: 0, Controller: 0},
			{Card: cho, Owner: 1, Controller: 1, Sick: true},
			{Card: bonesplitter, Owner: 1, Controller: 1, Sick: true},
		},
	})
	rhinoID := ids.Graveyard[0][0]
	fighter := ids.Permanents[0]
	victim := ids.Permanents[1]
	artifact := ids.Permanents[2]
	face := e.G.Obj(rhinoID).Face()
	var last *effects.Ctx
	pumpSA := face.Abilities[0]
	d := kr0Run(t, e, pumpSA, func() *effects.Ctx {
		return &effects.Ctx{
			Source:     rhinoID,
			Controller: 0,
			SVars:      selectionSVars(face),
			Targets:    []state.Target{{Obj: fighter}},
			OfferedSA:  pumpSA,
			SubPreAsk:  map[string][]state.Target{pumpSA.Sub.Line: {{Obj: victim}}},
		}
	}, &last)
	if e.Power(fighter) != 5 {
		t.Fatalf("fighter power %d, want 5 after Rhino's +1/+0", e.Power(fighter))
	}
	if last == nil {
		t.Fatal("resolution context was not captured")
	}
	if last.SVars["Excess"] != "0" {
		t.Fatalf("Excess %q, want 0: Cho-Manno prevented the whole 5", last.SVars["Excess"])
	}
	if d != nil {
		t.Fatalf("unexpected excess decision %+v", d)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("queued %d triggers although the damage was prevented", len(e.pendingTriggers))
	}
	if e.G.Obj(artifact).Zone != state.ZBattlefield {
		t.Fatalf("artifact in %v, want battlefield", e.G.Obj(artifact).Zone)
	}
	replayCheck(t, e, cfg)
}
