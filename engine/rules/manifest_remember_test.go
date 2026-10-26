package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// RememberManifested$ True: the manifested card joins the resolution's
// Remembered so the chained counter/attach/draw riders find it.

func TestWildcallManifestsAndCountersTheManifestedCard(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Wildcall")
	id := searchMoveByName(t, e, "Wildcall", state.ZHand)
	top := e.G.Zone(state.ZLibrary, 0)[0]
	addMana(t, e, 0, "CCGG")
	castFromPriority(t, e, id)
	announceXCast(t, e, id, 2)
	passUntilStackEmpty(t, e, 20)
	got := manifestedOn(t, e, 0)
	if got != top {
		t.Fatalf("manifested %d, want the library top %d", got, top)
	}
	o := e.G.Obj(got)
	if !o.Manifested {
		t.Fatal("the face-down card is not marked manifested")
	}
	if n := counterCount(o, "P1P1"); n != 2 {
		t.Fatalf("P1P1 counters = %d, want 2", n)
	}
	noUnimplementedManifest(t, e)
	replayCheck(t, e, cfg)
}

func TestLightformAttachesToTheManifestedCard(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Lightform")
	id := searchMoveByName(t, e, "Lightform", state.ZHand)
	top := e.G.Zone(state.ZLibrary, 0)[0]
	addMana(t, e, 0, "CWW")
	castFromPriority(t, e, id)
	passUntilStackEmpty(t, e, 20)
	got := manifestedOn(t, e, 0)
	if got != top {
		t.Fatalf("manifested %d, want the library top %d", got, top)
	}
	lf := e.G.Obj(id)
	if lf.Zone != state.ZBattlefield || lf.AttachedTo != got {
		t.Fatalf("Lightform zone %v attached to %d, want the battlefield and %d", lf.Zone, lf.AttachedTo, got)
	}
	noUnimplementedManifest(t, e)
	replayCheck(t, e, cfg)
}

func TestKozilekBrokenRealityManifestsFromTargetedHandsAndDraws(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Kozilek, the Broken Reality")
	id := searchMoveByName(t, e, "Kozilek, the Broken Reality", state.ZHand)
	if len(e.G.Zone(state.ZHand, 0)) < 3 || len(e.G.Zone(state.ZHand, 1)) < 2 {
		t.Fatal("precondition: both hands hold at least two other cards")
	}
	addMana(t, e, 0, "CCCCCCCCC")
	castFromPriority(t, e, id)
	handBefore := [2]int{len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1))}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) != 2 || d.Max != 2 {
		t.Fatalf("want the up-to-two player target ask, got %+v", d)
	}
	submitChoices(t, e, 0, 1)
	var picked []state.ObjID
	for seat := state.PlayerID(0); seat < 2; seat++ {
		d = passUntilAskKind(t, e, decision.KChoose, 20)
		if d.Player != seat || d.Min != 2 || d.Max != 2 {
			t.Fatalf("ask %d = player %d min %d max %d, want seat %d choosing two", seat, d.Player, d.Min, d.Max, seat)
		}
		for _, o := range d.Options {
			if e.G.Obj(o.Obj).Owner != seat || e.G.Obj(o.Obj).Zone != state.ZHand {
				t.Fatalf("seat %d offered %d outside its own hand", seat, o.Obj)
			}
		}
		picked = append(picked, d.Options[0].Obj, d.Options[1].Obj)
		submitChoices(t, e, 0, 1)
	}
	passUntilStackEmpty(t, e, 40)
	manifested := [2]int{}
	for _, id := range picked {
		o := e.G.Obj(id)
		if o.Zone != state.ZBattlefield || !o.FaceDown || !o.Manifested {
			t.Fatalf("picked card %d zone %v facedown %v manifested %v", id, o.Zone, o.FaceDown, o.Manifested)
		}
		manifested[o.Controller]++
	}
	if manifested != [2]int{2, 2} {
		t.Fatalf("manifested per controller = %v, want 2 and 2", manifested)
	}
	// The caster put Kozilek on the stack before handBefore was read, so its
	// hand: -2 manifested, +4 drawn.
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore[0]-2+4 {
		t.Fatalf("caster hand = %d, want %d", got, handBefore[0]-2+4)
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != handBefore[1]-2 {
		t.Fatalf("opponent hand = %d, want %d", got, handBefore[1]-2)
	}
	noUnimplementedManifest(t, e)
	replayCheck(t, e, cfg)
}
