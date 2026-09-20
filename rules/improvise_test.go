package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// addArtifacts puts n synthetic artifacts (each built from src) under
// seat 0 on the battlefield and returns their ids in battlefield order.
func addArtifacts(t *testing.T, e *Engine, n int, src string) []state.ObjID {
	t.Helper()
	var ids []state.ObjID
	for i := 0; i < n; i++ {
		o := e.G.AddObject(card(t, src), 0)
		o.Zone = state.ZBattlefield
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), ids...))
	return ids
}

const plainArtifactSrc = "Name:Gear\nTypes:Artifact\nOracle:x\n"
const manaArtifactSrc = "Name:Mana Gear\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n"

// TestImproviseOrganicExtinctionAnnouncesArtifactsAndPays is the corpus pin:
// {8}{W}{W} DestroyAll with Improvise. With eight untapped artifacts and a
// pool holding only the two white pips, the cast is offered, the CR 601.2b
// announcement offers one improvise_generic option per untapped artifact
// (a tapped artifact none), the chosen artifacts are committed and TAPPED
// as the payment, and the pool paid exactly its {W}{W} -- the artifacts
// contributed no mana.
func TestImproviseOrganicExtinctionAnnouncesArtifactsAndPays(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Organic Extinction"))
	ids := addArtifacts(t, e, 8, plainArtifactSrc)
	// A tapped artifact is never an Improvise candidate (CR 702.66a).
	tapped := addArtifacts(t, e, 1, plainArtifactSrc)[0]
	e.G.Obj(tapped).Tapped = true
	spell := e.G.Zone(state.ZHand, 0)[0]
	if !e.HasKeyword(spell, "Improvise") {
		t.Fatal("corpus Organic Extinction lost Improvise before cast")
	}
	e.G.Players[0].Pool[state.MW] = 2
	castMode(t, e, spell, "")
	d := e.Pending()
	if d == nil || len(d.Options) == 0 {
		t.Fatalf("Improvise did not announce its artifact payment: %+v", d)
	}
	var byID map[state.ObjID]int
	for i, o := range d.Options {
		if o.Kind != "improvise_generic" {
			t.Fatalf("announcement offered a non-improvise option: %+v", o)
		}
		if o.Obj == tapped {
			t.Fatalf("tapped artifact %v was offered as an Improvise payment", tapped)
		}
		if byID == nil {
			byID = map[state.ObjID]int{}
		}
		byID[o.Obj] = i
	}
	if len(byID) != len(ids) {
		t.Fatalf("announcement offered %d artifacts, want %d: %+v", len(byID), len(ids), d.Options)
	}
	// Pay all eight generic pips with the artifacts.
	var picks []int
	for _, id := range ids {
		picks = append(picks, byID[id])
	}
	submitChoices(t, e, picks...)
	if e.cast != nil {
		t.Fatalf("Improvise payment did not complete the cast: %+v", e.cast)
	}
	if e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("spell went to %s, want stack", e.G.Obj(spell).Zone)
	}
	for _, id := range ids {
		if !e.G.Obj(id).Tapped {
			t.Fatalf("committed Improvise artifact %d was not tapped", id)
		}
	}
	if !e.G.Obj(tapped).Tapped {
		t.Fatal("the pre-tapped artifact was untapped")
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("pool left %v after Improvise payment, want exactly the two white pips spent", pool)
	}
}

// TestImproviseCommittedArtifactExcludedFromManaWindow: CR 702.66b taps the
// artifacts after mana abilities are done, so an artifact announced for
// Improvise is reserved (convokeCommitted) and is NOT offered again in the
// CR 601.2g mana window; an unannounced mana source still is.
func TestImproviseCommittedArtifactExcludedFromManaWindow(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Organic Extinction"))
	ids := addArtifacts(t, e, 8, plainArtifactSrc)
	manaGear := addArtifacts(t, e, 1, manaArtifactSrc)[0]
	land := e.G.AddObject(card(t, "Name:Land\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n"), 0)
	land.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), land.ID))
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MW] = 2
	castMode(t, e, spell, "")
	d := e.Pending()
	if d == nil {
		t.Fatalf("Improvise did not announce: %+v", d)
	}
	var byID map[state.ObjID]int
	for i, o := range d.Options {
		if o.Kind != "improvise_generic" {
			t.Fatalf("announcement offered a non-improvise option: %+v", o)
		}
		if byID == nil {
			byID = map[state.ObjID]int{}
		}
		byID[o.Obj] = i
	}
	if _, ok := byID[manaGear]; !ok {
		t.Fatalf("mana artifact %v was not offered as an Improvise payment", manaGear)
	}
	// Announce seven of the eight plain artifacts, INCLUDING the mana
	// gear, leaving one generic for the mana window.
	var picks []int
	for _, id := range append([]state.ObjID{manaGear}, ids[:6]...) {
		picks = append(picks, byID[id])
	}
	submitChoices(t, e, picks...)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("remaining generic did not open the mana window: %+v", d)
	}
	var windowIDs []state.ObjID
	for _, o := range d.Options {
		if o.Obj != 0 {
			windowIDs = append(windowIDs, o.Obj)
		}
	}
	for _, wid := range windowIDs {
		if e.convokeCommitted(e.cast, wid) {
			t.Fatalf("committed Improvise artifact %v was offered again in the mana window", wid)
		}
		if wid != land.ID {
			t.Fatalf("unexpected mana-window source %v", wid)
		}
	}
	if len(windowIDs) != 1 {
		t.Fatalf("mana window offered %v, want only the land", windowIDs)
	}
	submitChoices(t, e, 0)
	if e.cast != nil {
		t.Fatalf("cast did not complete after the mana window: %+v", e.cast)
	}
	if e.G.Obj(spell).Zone != state.ZStack || !e.G.Obj(manaGear).Tapped || !e.G.Obj(land.ID).Tapped {
		t.Fatalf("payment did not tap and cast: spell=%s gear=%v land=%v",
			e.G.Obj(spell).Zone, e.G.Obj(manaGear).Tapped, e.G.Obj(land.ID).Tapped)
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("pool left %v after payment", pool)
	}
}

// TestImproviseNoArtifactsNoAnnouncement is the control: with zero
// artifacts the announcement is never posed (convokeAsk's empty-options
// early return) and a pool-paid cast completes without any decision.
func TestImproviseNoArtifactsNoAnnouncement(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Organic Extinction"))
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MW] = 8, 2
	castMode(t, e, spell, "")
	if d := e.Pending(); d != nil {
		t.Fatalf("no-artifact Improvise posed an announcement: %+v", d)
	}
	if e.cast != nil || e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("pool-paid Improvise cast did not complete: cast=%+v zone=%s", e.cast, e.G.Obj(spell).Zone)
	}
}

// TestImproviseArtifactCreatureQualifies: CR 702.66a says "artifact", and
// an artifact creature is an artifact independently of being a creature.
func TestImproviseArtifactCreatureQualifies(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Organic Extinction"))
	addArtifacts(t, e, 7, plainArtifactSrc)
	creature := e.G.AddObject(card(t, "Name:Gear Golem\nManaCost:4\nTypes:Artifact Creature Golem\nPT:4/4\nOracle:x\n"), 0)
	creature.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), creature.ID))
	if !creature.EffectiveIsArtifact() {
		t.Fatal("artifact creature did not read as an artifact")
	}
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MW] = 2
	castMode(t, e, spell, "")
	d := e.Pending()
	announced := map[state.ObjID]bool{}
	for _, o := range d.Options {
		if o.Kind != "improvise_generic" {
			t.Fatalf("announcement offered a non-improvise option: %+v", o)
		}
		announced[o.Obj] = true
	}
	if !announced[creature.ID] {
		t.Fatalf("artifact creature %v was not offered as an Improvise payment: %+v", creature.ID, d.Options)
	}
	picks := []int{}
	for i, o := range d.Options {
		if announced[o.Obj] {
			picks = append(picks, i)
		}
	}
	if len(picks) != 8 {
		t.Fatalf("announcement offered %d artifacts, want 8", len(picks))
	}
	submitChoices(t, e, picks...)
	if e.cast != nil || e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("cast did not complete: cast=%+v zone=%s", e.cast, e.G.Obj(spell).Zone)
	}
	if !e.G.Obj(creature.ID).Tapped {
		t.Fatal("artifact creature was not tapped as the payment")
	}
}
