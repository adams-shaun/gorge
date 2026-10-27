package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gixRun is a Gix, Yawgmoth Praetor activation fixture: Gix on seat 0's
// battlefield, exactly handSize Grizzly Bears in hand, eight black mana, and
// the opponent's top two library cards recorded before the activation.
type gixRun struct {
	e        *Engine
	cfg      Config
	gix      state.ObjID
	bears    []state.ObjID
	oppTop   []state.ObjID
	maxX     int
	minX     int
	xOffered bool
}

func gixSetup(t *testing.T, handSize int) *gixRun {
	t.Helper()
	e, cfg := raiseEngine(t, []string{"Gix, Yawgmoth Praetor"}, nil)
	r := &gixRun{e: e, cfg: cfg, maxX: -1, minX: -1}
	r.gix = blightMove(t, e, 0, "Gix, Yawgmoth Praetor", state.ZBattlefield)
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	for i := 0; i < handSize; i++ {
		r.bears = append(r.bears, raiseMoveAnother(t, e, 0, "Grizzly Bears", state.ZHand))
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handSize {
		t.Fatalf("precondition: seat 0 hand = %d cards, want %d", got, handSize)
	}
	lib := e.G.Zone(state.ZLibrary, 1)
	if len(lib) < 3 {
		t.Fatalf("precondition: opponent library has %d cards", len(lib))
	}
	r.oppTop = append([]state.ObjID(nil), lib[:2]...)
	addMana(t, e, 0, "BBBBBBBB")
	return r
}

// activate runs the activation answering the X ask with wantX.
func (r *gixRun) activate(t *testing.T, wantX int) {
	t.Helper()
	ab := -1
	for i, sa := range r.e.G.Obj(r.gix).Face().Abilities {
		if sa.Kind == "AB" && sa.API == "Dig" {
			ab = i
		}
	}
	if ab < 0 {
		t.Fatal("Gix has no Dig activated ability")
	}
	submitChoices(t, r.e, abilityOption(t, r.e, r.gix, ab).Index)
	raiseDrive(t, r.e, func(d *decision.Decision) []int {
		var pick []int
		for _, o := range d.Options {
			if o.Kind != "x" {
				continue
			}
			r.xOffered = true
			if r.maxX < 0 || o.Amount > r.maxX {
				r.maxX = o.Amount
			}
			if r.minX < 0 || o.Amount < r.minX {
				r.minX = o.Amount
			}
			if o.Amount == wantX {
				pick = []int{o.Index}
			}
		}
		return pick
	})
	// The resolving Dig offers the exiled cards for free play: decline it.
	for n := 0; n < 30 && !r.e.G.Over && len(r.e.G.Stack) > 0; n++ {
		d := r.e.Pending()
		if d == nil {
			t.Fatal("stalled with no decision while resolving Gix's ability")
		}
		switch {
		case d.ResumeKind == "play":
			submitChoices(t, r.e)
		case d.Kind == decision.KPriority:
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			if pass < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d.Options)
			}
			submitChoices(t, r.e, pass)
		default:
			t.Fatalf("unexpected decision while resolving: kind=%v prompt=%q", d.Kind, d.Prompt)
		}
	}
	if d := r.e.Pending(); d != nil && d.ResumeKind == "play" {
		submitChoices(t, r.e)
	}
}

func (r *gixRun) zone(id state.ObjID) state.Zone { return raiseZone(r.e, id) }

func TestGixDiscardXExilesX(t *testing.T) {
	t.Parallel()
	r := gixSetup(t, 3)
	r.activate(t, 2)
	if !r.xOffered {
		t.Fatal("Gix's Discard<X> activation never asked for X")
	}
	discarded := 0
	for _, b := range r.bears {
		if r.zone(b) == state.ZGraveyard {
			discarded++
		}
	}
	if discarded != 2 || len(r.e.G.Zone(state.ZHand, 0)) != 1 {
		t.Fatalf("X=2 discarded %d bears, hand=%d; want 2 discarded and 1 left", discarded, len(r.e.G.Zone(state.ZHand, 0)))
	}
	for _, id := range r.oppTop {
		if z := r.zone(id); z != state.ZExile {
			t.Fatalf("opponent top card %d in %s, want exile", id, z)
		}
	}
	if got := len(r.e.G.Zone(state.ZExile, 1)); got != 2 {
		t.Fatalf("opponent has %d exiled cards, want exactly 2", got)
	}
	if left := r.e.G.Players[0].Pool.Total(); left != 1 {
		t.Fatalf("pool after activation = %d, want 1 (cost exactly {4}{B}{B}{B})", left)
	}
	replayCheck(t, r.e, r.cfg)
}

func TestGixDiscardXCappedByHand(t *testing.T) {
	t.Parallel()
	r := gixSetup(t, 1)
	r.activate(t, 1)
	if !r.xOffered {
		t.Fatal("Gix's Discard<X> activation never asked for X")
	}
	if r.maxX != 1 {
		t.Fatalf("largest X offered = %d, want 1 (one card in hand)", r.maxX)
	}
	if z := r.zone(r.bears[0]); z != state.ZGraveyard {
		t.Fatalf("the lone card is in %s, want graveyard", z)
	}
	replayCheck(t, r.e, r.cfg)
}

func TestGixDiscardXZero(t *testing.T) {
	t.Parallel()
	r := gixSetup(t, 3)
	r.activate(t, 0)
	if !r.xOffered || r.minX != 0 {
		t.Fatalf("X=0 not offered (offered=%v min=%d)", r.xOffered, r.minX)
	}
	for _, b := range r.bears {
		if z := r.zone(b); z != state.ZHand {
			t.Fatalf("bear %d in %s after X=0, want hand", b, z)
		}
	}
	if got := len(r.e.G.Zone(state.ZExile, 1)); got != 0 {
		t.Fatalf("opponent has %d exiled cards after X=0, want 0", got)
	}
	if left := r.e.G.Players[0].Pool.Total(); left != 1 {
		t.Fatalf("pool after activation = %d, want 1", left)
	}
	replayCheck(t, r.e, r.cfg)
}
