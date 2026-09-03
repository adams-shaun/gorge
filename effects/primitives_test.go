package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestDealDamageHitsATargetedPlayer(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[1].Life = 20
	c := &Ctx{Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, sa(t, "SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3"))
	if h.g.Players[1].Life != 17 {
		t.Fatalf("life = %d, want 17", h.g.Players[1].Life)
	}
}

func TestDealDamageMarksAPermanent(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}
	Resolve(h, c, sa(t, "SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2"))
	if got := g.Obj(ids["myBear"]).Damage; got != 2 {
		t.Fatalf("marked damage = %d, want 2", got)
	}
}

func TestDealDamageDefaultsMissingNumDmgToZero(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[1].Life = 20
	c := &Ctx{Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, sa(t, "SP$ DealDamage | ValidTgts$ Any"))
	if h.g.Players[1].Life != 20 {
		t.Fatalf("life = %d, want unchanged at 20", h.g.Players[1].Life)
	}
}

func TestAddManaFillsTheControllersPool(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T | Produced$ R | Amount$ 1"))
	if h.g.Players[0].Pool[state.MR] != 1 {
		t.Fatalf("pool = %v, want 1 red", h.g.Players[0].Pool)
	}
	// The other seat's pool must be untouched.
	if h.g.Players[1].Pool.Total() != 0 {
		t.Fatalf("player 1 pool = %v, want empty", h.g.Players[1].Pool)
	}
}

func TestAddManaDefaultsToColorlessAndAmountOne(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T"))
	if h.g.Players[0].Pool[state.MC] != 1 {
		t.Fatalf("pool = %v, want 1 colourless", h.g.Players[0].Pool)
	}
}

// TestPrimitivesAreRegistered guards against a silent unregistration: if
// either primitive were ever dropped, Resolve would fall back to emitting a
// Note event ("unimplemented API ...") instead of the real effect, and the
// tests above would need to fail loudly rather than quietly pass on a no-op.
func TestPrimitivesAreRegistered(t *testing.T) {
	sup := Supported()
	for _, api := range []string{"DealDamage", "Mana"} {
		if !sup["api:"+api] {
			t.Fatalf("api:%s not registered", api)
		}
	}
}
