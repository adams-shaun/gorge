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
	in.ObjOf = func(id int) (dirac.Obj, bool) { return dirac.Obj(id), true }
	v := &in.View
	v.Viewer, v.Turn, v.Step, v.Active, v.Priority = 0, 1, "main1", 0, 0
	for p := 0; p < 2; p++ {
		pv := view.PlayerView{ID: state.PlayerID(p), Life: 20, HandSize: 7, LibrarySize: 13}
		for i := 0; i < 7; i++ {
			if p == 0 {
				pv.Hand = append(pv.Hand, view.CardView{ID: state.ObjID(i), Name: "Plains"})
			} else {
				in.Hidden.Hand[1] = append(in.Hidden.Hand[1], shadow.Card{Obj: dirac.Obj(20 + i), Name: "Plains"})
			}
		}
		for i := 0; i < 13; i++ {
			in.Hidden.Library[p] = append(in.Hidden.Library[p], shadow.Card{Obj: dirac.Obj(p*20 + 7 + i), Name: "Savannah Lions"})
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
		shadow.ReasonOver:  func(in *shadow.Input) { in.View.Over = true },
		shadow.ReasonSeats: func(in *shadow.Input) { in.View.Players = in.View.Players[:1] },
		shadow.ReasonSeatLeft: func(in *shadow.Input) {
			// a four-player view with the last seat out
			for p := 2; p < 4; p++ {
				pv := view.PlayerView{ID: state.PlayerID(p), Life: 20, HandSize: 7, LibrarySize: 13, Lost: p == 3}
				if p == 3 {
					// a departed seat whose view still shows a hand card
					pv.Hand = []view.CardView{{ID: 60, Name: "Plains"}}
				}
				for i := 0; i < 7; i++ {
					in.Hidden.Hand[p] = append(in.Hidden.Hand[p], shadow.Card{Obj: dirac.Obj(p*20 + i), Name: "Plains"})
				}
				for i := 0; i < 13; i++ {
					in.Hidden.Library[p] = append(in.Hidden.Library[p], shadow.Card{Obj: dirac.Obj(p*20 + 7 + i), Name: "Savannah Lions"})
				}
				in.View.Players = append(in.View.Players, pv)
			}
		},
		shadow.ReasonStack:    func(in *shadow.Input) { in.View.Stack = []view.StackView{{}} },
		shadow.ReasonPending:  func(in *shadow.Input) { in.View.Pending = []view.PendingView{{}} },
		shadow.ReasonDecision: func(in *shadow.Input) { in.View.Decision = &decision.Decision{Kind: decision.KTarget} },
		shadow.ReasonUnknown:  func(in *shadow.Input) { in.View.Players[0].Hand[0].Name = "Black Lotus" },
		shadow.ReasonUnmapped: func(in *shadow.Input) { in.ObjOf = func(int) (dirac.Obj, bool) { return 0, false } },
		shadow.ReasonHidden:   func(in *shadow.Input) { in.Hidden.Library[1] = in.Hidden.Library[1][1:] },
		shadow.ReasonExile:    func(in *shadow.Input) { in.View.Players[1].Exile = []view.CardView{{Name: "Plains"}} },
		dirac.ReasonCommander: func(in *shadow.Input) {
			// a partner pair: two commanders in one roster
			in.View.Players[0].Commanders = []view.CardView{{ID: 0, Name: "Plains"}, {ID: 1, Name: "Plains"}}
			in.View.Players[0].CommanderCasts = []int32{0, 0}
		},
		shadow.ReasonToken:    func(in *shadow.Input) { bf(in, view.CardView{IsToken: true}) },
		shadow.ReasonCounter:  func(in *shadow.Input) { bf(in, view.CardView{Counters: map[string]int32{"M1M1": 1}}) },
		shadow.ReasonAttach:   func(in *shadow.Input) { bf(in, view.CardView{AttachedTo: 5}) },
		shadow.ReasonMark:     func(in *shadow.Input) { bf(in, view.CardView{FaceDown: true}) },
		shadow.ReasonPool:     func(in *shadow.Input) { in.View.Players[0].Pool = map[string]int32{"R": 1} },
		shadow.ReasonStepName: func(in *shadow.Input) { in.View.Step = "" },
		dirac.ReasonStep:      func(in *shadow.Input) { in.View.Step = "combat-damage" },
		dirac.ReasonCombat:    func(in *shadow.Input) { bf(in, view.CardView{Blocked: true}) },
		dirac.ReasonControl:   func(in *shadow.Input) { bf(in, view.CardView{Controller: 1}) },
		dirac.ReasonTable:     func(in *shadow.Input) { in.Tbl = nil },
		dirac.ReasonStackEntry: func(in *shadow.Input) {
			// a Savannah Lions "ability" entry: not a self-counter ability
			in.View.Stack = []view.StackView{{Kind: "ability", Source: 0, Card: &view.CardView{Name: "Hungry Ghoul"}}}
		},
	}
	// a targeted entry whose target is no battlefield object the view shows
	// (a card in a hand: it left; or a player) is ReasonStack
	for name, tv := range map[string]view.TargetView{
		"off the battlefield": {Obj: 3},
		"a player":            {Player: 1, IsPlayer: true},
	} {
		in := reasonsInput()
		bf(in, view.CardView{})
		in.View.Stack = []view.StackView{{Kind: "ability", Source: 0, Card: &view.CardView{Name: "Hungry Ghoul"}, Targets: []view.TargetView{tv}}}
		s, err := shadow.StageFromView(in)
		var u dirac.ErrUnstageable
		if !errors.As(err, &u) || u.Reason != shadow.ReasonStack || s != nil {
			t.Errorf("target %s: got state %v, error %v; want %q", name, s != nil, err, shadow.ReasonStack)
		}
	}
	// a card in a command zone that is no commander of its seat is exile's
	in := reasonsInput()
	in.View.Players[1].Command = []view.CardView{{ID: 40, Name: "Plains"}}
	if _, err := shadow.StageFromView(in); !errors.As(err, new(dirac.ErrUnstageable)) || err.(dirac.ErrUnstageable).Reason != shadow.ReasonExile {
		t.Errorf("a non-commander in the command zone: got error %v, want %q", err, shadow.ReasonExile)
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

// TestStageCombatPositions: a combat position the view settles stages, and a
// combat position it cannot settle, or one the staging cannot rebuild, returns
// its reason.
func TestStageCombatPositions(t *testing.T) {
	t.Parallel()
	seat1 := state.PlayerID(1)
	// seat 0 attacks seat 1 with Savannah Lions (object 0)
	attack := func(in *shadow.Input, c view.CardView) {
		in.View.Step = "declare-attackers"
		in.View.Players[0].Hand = in.View.Players[0].Hand[1:]
		in.View.Players[0].HandSize--
		c.ID, c.Name = 0, "Savannah Lions"
		in.View.Players[0].Battlefield = []view.CardView{c}
	}
	// seat 1 blocks object 0 with its object 20, taken from its hidden hand
	block := func(in *shadow.Input) {
		attack(in, view.CardView{Attacking: true, AttackingPlayer: &seat1, Blocked: true, BlockedBy: []state.ObjID{20}})
		in.View.Step = "declare-blockers"
		in.Hidden.Hand[1] = in.Hidden.Hand[1][1:]
		in.View.Players[1].HandSize--
		in.View.Players[1].Battlefield = []view.CardView{{ID: 20, Name: "Savannah Lions", Owner: 1, Controller: 1}}
	}
	ok := map[string]func(*shadow.Input){
		"begin combat":       func(in *shadow.Input) { in.View.Step = "begin-combat" },
		"attackers declared": func(in *shadow.Input) { attack(in, view.CardView{Attacking: true, AttackingPlayer: &seat1}) },
		"blocks declared":    block,
		"declare attackers pose": func(in *shadow.Input) {
			attack(in, view.CardView{})
			in.View.Decision = &decision.Decision{Kind: decision.KAttackers}
		},
		"declare blockers pose": func(in *shadow.Input) {
			block(in)
			in.View.Players[0].Battlefield[0].Blocked, in.View.Players[0].Battlefield[0].BlockedBy = false, nil
			in.View.Decision = &decision.Decision{Kind: decision.KBlockers}
		},
	}
	for name, mut := range ok {
		in := reasonsInput()
		mut(in)
		if _, err := shadow.StageFromView(in); err != nil {
			t.Errorf("%s does not stage: %v", name, err)
		}
	}
	bad := map[string]struct {
		reason string
		mut    func(*shadow.Input)
	}{
		"declare attackers, no decision shown, no attacker": {shadow.ReasonDecision, func(in *shadow.Input) { attack(in, view.CardView{}) }},
		"attackers decision at main":                        {shadow.ReasonDecision, func(in *shadow.Input) { in.View.Decision = &decision.Decision{Kind: decision.KAttackers} }},
		"combat damage":                                     {dirac.ReasonStep, func(in *shadow.Input) { in.View.Step = "combat-damage" }},
		"end of combat":                                     {dirac.ReasonStep, func(in *shadow.Input) { in.View.Step = "end-combat" }},
		"blocked with no blocker listed": {dirac.ReasonCombat, func(in *shadow.Input) {
			block(in)
			in.View.Players[0].Battlefield[0].BlockedBy = nil
		}},
		"attacking at main": {dirac.ReasonCombat, func(in *shadow.Input) {
			attack(in, view.CardView{Attacking: true, AttackingPlayer: &seat1})
			in.View.Step = "main1"
		}},
		"a creature attacking a permanent": {shadow.ReasonMark, func(in *shadow.Input) {
			attack(in, view.CardView{Attacking: true, AttackingPlayer: &seat1, AttackingPermanent: true})
		}},
	}
	for name, c := range bad {
		in := reasonsInput()
		c.mut(in)
		var u dirac.ErrUnstageable
		if _, err := shadow.StageFromView(in); !errors.As(err, &u) || u.Reason != c.reason {
			t.Errorf("%s: want reason %q, got %v", name, c.reason, err)
		}
	}
}
