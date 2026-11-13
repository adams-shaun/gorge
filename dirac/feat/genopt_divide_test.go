package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// dividePlayerDecision is a resolution division's per-point ask over two
// player targets: gorge's damage_split KChoose (engine/effects/damage_deal.go)
// poses one DkChoose/ChGenDivide per point with source = the player dealt it.
func dividePlayerDecision() dirac.Decision {
	d := dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChGenDivide, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkChoose, dirac.PlayerTarget0, 0, 0)
	d.Options[1] = dirac.MakeAction(dirac.AkChoose, dirac.PlayerTarget1, 0, 0)
	return d
}

// TestGenDividePlayerOptionNamesSeat: a resolution division over a player is
// gorge's Kind "player" option, so the feat encoder must set the same
// relative-seat one-hot policynet writes into odSeat0.. (columns 24..27).
// Before the fix o.named stayed -1 and every seat column was zero.
func TestGenDividePlayerOptionNamesSeat(t *testing.T) {
	d := dividePlayerDecision()
	v := view.View{}
	for p := 0; p < 2; p++ {
		v.Players = append(v.Players, view.PlayerView{ID: state.PlayerID(p), Life: 20})
	}
	e := &Encoder{}
	for p, target := range []dirac.TCode{dirac.PlayerTarget0, dirac.PlayerTarget1} {
		a := d.Options[p]
		// Precondition: the option really is the player-target pick the fix
		// keys on, not an object pick that happens to sort after it.
		if a.Source() != target || target < dirac.PlayerTarget0 {
			t.Fatalf("setup: option %d source %d is not player target %d", p, a.Source(), target)
		}
		o := e.optionOf(&d, a)
		if o.kind != okPlayer || !o.player {
			t.Fatalf("option %d: kind %d player %v, want okPlayer and a player", p, o.kind, o.player)
		}
		if want := int8(p); o.named != want {
			t.Fatalf("option %d: named %d, want %d", p, o.named, want)
		}
		var got Option
		e.encodeOption(&got, o, p, len(d.Options))
		g := policynet.EncodeOption(v, 0, decision.KChoose,
			decision.Option{Kind: "player", Player: state.PlayerID(p)}, p, len(d.Options))
		if len(g.Dense) != policynet.OptionDenseWidth {
			t.Fatalf("policynet dense width %d", len(g.Dense))
		}
		for k := 0; k < 4; k++ {
			want := float32(0)
			if k == p { // both encoders' viewer is seat 0
				want = 1
			}
			if feat := got.Dense[24+k]; feat != want {
				t.Errorf("option %d: feat seat column %d = %v, want %v", p, k, feat, want)
			}
			if feat, pol := got.Dense[24+k], g.Dense[24+k]; feat != pol {
				t.Errorf("option %d: seat column %d feat %v, policynet %v", p, k, feat, pol)
			}
		}
	}
}
