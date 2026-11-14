package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// taporuntapallHost is fakeHost plus the ask seam, serving option index idx
// to the "taporuntapall" election and recording every one it was offered.
type taporuntapallHost struct {
	*fakeHost
	idx    int
	served []*decision.Decision
}

func (h *taporuntapallHost) AskCount() uint64 { return uint64(h.askCount) }

func (h *taporuntapallHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	if d.ResumeKind != "taporuntapall" {
		return decision.Intent{}, false
	}
	h.served = append(h.served, d)
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{h.idx}}, true
}

// TestTapOrUntapAllOneElection: Turnabout's TapOrUntapAll (ValidTgts$ Player,
// its chosen type pinned to Creature) over a target player with one tapped
// and one untapped creature poses ONE tap-or-untap election and leaves every
// matching permanent in the elected state, never toggling each one.
func TestTapOrUntapAllOneElection(t *testing.T) {
	const turnabout = "SP$ TapOrUntapAll | ValidTgts$ Player | ValidCards$ Creature | ValidMessage$ permanents of the chosen type target player controls"
	for _, tc := range []struct {
		idx        int
		kind       string
		wantTapped bool
	}{{0, "tap", true}, {1, "untap", false}} {
		t.Run(tc.kind, func(t *testing.T) {
			g, ids := board(t)
			h := &taporuntapallHost{fakeHost: &fakeHost{g: g}, idx: tc.idx}
			mine := []state.ObjID{ids["myBear"], ids["myFlier"]}
			for _, id := range mine {
				if o := g.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
					t.Fatalf("precondition: %d is not seat 0's battlefield creature", id)
				}
			}
			h.EmitTap(ids["myBear"], 1, false)
			if !g.Obj(ids["myBear"]).Tapped || g.Obj(ids["myFlier"]).Tapped {
				t.Fatal("precondition: expected a mixed tapped/untapped board")
			}
			if g.Obj(ids["theirBig"]).Tapped || g.Obj(ids["theirBig"]).Controller == 0 {
				t.Fatal("precondition: theirBig is the untapped non-target creature")
			}

			Resolve(h, playerCtx(0), sa(t, turnabout))

			if len(h.served) != 1 {
				t.Fatalf("elections posed = %d, want 1", len(h.served))
			}
			d := h.served[0]
			if d.Player != 0 || d.Kind != decision.KChoose || len(d.Options) != 2 ||
				d.Options[0].Kind != "tap" || d.Options[1].Kind != "untap" {
				t.Fatalf("election = %+v, want the controller's KChoose tap/untap", d)
			}
			for _, id := range mine {
				if got := g.Obj(id).Tapped; got != tc.wantTapped {
					t.Errorf("%s: %s tapped = %v, want %v", tc.kind, g.Obj(id).Face().Name, got, tc.wantTapped)
				}
			}
			if g.Obj(ids["theirBig"]).Tapped {
				t.Errorf("%s: the non-target player's creature changed state", tc.kind)
			}
		})
	}
}
