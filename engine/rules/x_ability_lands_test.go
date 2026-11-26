package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestXAbilityOfferedFromLands: an announced-X activated ability whose mana
// must come from untapped lands is offered (CR 602.2b: mana abilities are
// activated while paying). Domesticated Hydra with three untapped Forests and
// an empty pool is offered {X}{G}{G}{G}: Monstrosity X and activates with
// X = 0.
func TestXAbilityOfferedFromLands(t *testing.T) {
	t.Parallel()
	e, cfg, hydra := monstrosityEngine(t, "Domesticated Hydra", "")
	var forests []state.ObjID
	for i := 0; i < 3; i++ {
		forests = append(forests, searchMoveByName(t, e, "Forest", state.ZBattlefield))
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool holds %d mana, want empty", got)
	}
	for _, f := range forests {
		if o := e.G.Obj(f); o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Forest %d zone=%v tapped=%v", f, o.Zone, o.Tapped)
		}
	}
	idx := monstrosityAbilityIndex(t, e, hydra)
	opt, ok := findAbilityOption(e, hydra, idx)
	if !ok {
		t.Fatalf("Monstrosity X ability not offered from three untapped Forests: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want X announcement, got %+v", d)
	}
	x0 := -1
	for _, o := range d.Options {
		if o.Kind == "x" && o.Amount == 0 {
			x0 = o.Index
		}
		if o.Kind == "x" && o.Amount > 0 {
			t.Fatalf("X = %d offered with only three Forests for {X}{G}{G}{G}: %+v", o.Amount, d.Options)
		}
	}
	if x0 < 0 {
		t.Fatalf("X=0 not offered: %+v", d.Options)
	}
	submitChoices(t, e, x0)
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(hydra)
	if !o.Monstrous || o.Counter("P1P1") != 0 {
		t.Fatalf("Domesticated Hydra after X=0: monstrous=%v counters=%d, want monstrous with no counters", o.Monstrous, o.Counter("P1P1"))
	}
	if marks := monstrousMarkEvents(e); len(marks) != 1 || marks[0].Obj != hydra || marks[0].Amount != 0 {
		t.Fatalf("want one Monstrous mark carrying X=0, got %+v", marks)
	}
	for _, f := range forests {
		if !e.G.Obj(f).Tapped {
			t.Fatalf("Forest %d untapped after paying {G}{G}{G}", f)
		}
	}
	replayCheck(t, e, cfg)
}
