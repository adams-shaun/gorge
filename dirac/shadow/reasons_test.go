package shadow_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// reasonsInput is a legal observation: seat 0's main phase, seat 0 holding
// seven Plains, 13 cards in each library, the opponent holding seven.
func reasonsInput() *shadow.Input {
	in := &shadow.Input{Tbl: vanilla.Table, Cfg: dirac.LockstepConfig(dirac.SurfaceFull)}
	in.ObjOf = func(id int) (uint8, bool) { return uint8(id), true }
	v := &in.View
	v.Viewer, v.Turn, v.Step, v.Active, v.Priority = 0, 1, "main1", 0, 0
	for p := 0; p < 2; p++ {
		pv := view.PlayerView{ID: state.PlayerID(p), Life: 20, HandSize: 7, LibrarySize: 13}
		for i := 0; i < 7; i++ {
			if p == 0 {
				pv.Hand = append(pv.Hand, view.CardView{ID: state.ObjID(i), Name: "Plains"})
			} else {
				in.Hidden.Hand[1] = append(in.Hidden.Hand[1], shadow.Card{Obj: uint8(20 + i), Name: "Plains"})
			}
		}
		for i := 0; i < 13; i++ {
			in.Hidden.Library[p] = append(in.Hidden.Library[p], shadow.Card{Obj: uint8(p*20 + 7 + i), Name: "Savannah Lions"})
		}
		v.Players = append(v.Players, pv)
	}
	return in
}

// TestWorldFromViewReasons: the legal observation stages, and each way out of
// the minimal scope returns its own reason (this package's and dirac's that an
// observation can reach).
func TestWorldFromViewReasons(t *testing.T) {
	t.Parallel()
	if _, err := shadow.StageFromView(reasonsInput()); err != nil {
		t.Fatalf("the legal observation does not stage: %v", err)
	}
	bf := func(in *shadow.Input, c view.CardView) {
		in.View.Players[0].Hand = in.View.Players[0].Hand[1:]
		in.View.Players[0].HandSize--
		c.ID, c.Name = 0, "Savannah Lions"
		in.View.Players[0].Battlefield = []view.CardView{c}
	}
	// damage and +1/+1 counters on a battlefield creature stage
	for _, c := range []view.CardView{{Damage: 1}, {Counters: map[string]int32{"P1P1": 2}}} {
		in := reasonsInput()
		bf(in, c)
		if _, err := shadow.StageFromView(in); err != nil {
			t.Errorf("%+v does not stage: %v", c, err)
		}
	}
	cases := map[string]func(*shadow.Input){
		shadow.ReasonOver:     func(in *shadow.Input) { in.View.Over = true },
		shadow.ReasonSeats:    func(in *shadow.Input) { in.View.Players = in.View.Players[:1] },
		shadow.ReasonStack:    func(in *shadow.Input) { in.View.Stack = []view.StackView{{}} },
		shadow.ReasonPending:  func(in *shadow.Input) { in.View.Pending = []view.PendingView{{}} },
		shadow.ReasonDecision: func(in *shadow.Input) { in.View.Decision = &decision.Decision{Kind: decision.KTarget} },
		shadow.ReasonUnknown:  func(in *shadow.Input) { in.View.Players[0].Hand[0].Name = "Black Lotus" },
		shadow.ReasonUnmapped: func(in *shadow.Input) { in.ObjOf = func(int) (uint8, bool) { return 0, false } },
		shadow.ReasonHidden:   func(in *shadow.Input) { in.Hidden.Library[1] = in.Hidden.Library[1][1:] },
		shadow.ReasonExile:    func(in *shadow.Input) { in.View.Players[1].Exile = []view.CardView{{Name: "Plains"}} },
		shadow.ReasonToken:    func(in *shadow.Input) { bf(in, view.CardView{IsToken: true}) },
		shadow.ReasonCounter:  func(in *shadow.Input) { bf(in, view.CardView{Counters: map[string]int32{"M1M1": 1}}) },
		shadow.ReasonAttach:   func(in *shadow.Input) { bf(in, view.CardView{AttachedTo: 5}) },
		shadow.ReasonMark:     func(in *shadow.Input) { bf(in, view.CardView{Blocked: true}) },
		shadow.ReasonPool:     func(in *shadow.Input) { in.View.Players[0].Pool = map[string]int32{"R": 1} },
		shadow.ReasonStepName: func(in *shadow.Input) { in.View.Step = "" },
		dirac.ReasonStep:      func(in *shadow.Input) { in.View.Step = "declare-blockers" },
		dirac.ReasonControl:   func(in *shadow.Input) { bf(in, view.CardView{Controller: 1}) },
		dirac.ReasonTable:     func(in *shadow.Input) { in.Tbl = nil },
		dirac.ReasonStackEntry: func(in *shadow.Input) {
			// a Savannah Lions "ability" entry: not a self-counter ability
			in.View.Stack = []view.StackView{{Kind: "ability", Source: 0, Card: &view.CardView{Name: "Hungry Ghoul"}}}
		},
	}
	for _, r := range shadow.Reasons {
		if _, ok := cases[r]; !ok {
			t.Errorf("reason %q has no case", r)
		}
	}
	for r, mut := range cases {
		if !slices.Contains(shadow.Reasons, r) && !slices.Contains(dirac.UnstageableReasons, r) {
			t.Errorf("case %q is in neither reason list", r)
		}
		in := reasonsInput()
		mut(in)
		s, err := shadow.StageFromView(in)
		var u dirac.ErrUnstageable
		if !errors.As(err, &u) || u.Reason != r || s != nil {
			t.Errorf("reason %q: got state %v, error %v", r, s != nil, err)
		}
	}
}
