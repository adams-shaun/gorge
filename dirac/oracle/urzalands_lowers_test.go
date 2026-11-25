package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Count$UrzaLands (Urza's Mine, Urza's Power Plant, Urza's Tower: {C}, or
// {C}{C} / {C}{C}{C} once their controller controls all three Urza lands) in
// lockstep with gorge: the fixture pool (dirac/cardset/pools/fixtures/urzalands)
// plays the three lands beside Wastes and colourless creatures up to Wurmcoil
// Engine, so an assembled Tron's extra mana opens casts a plain land base
// cannot afford (dirac/kgen/value_urzalands.go; gorge count_body_heads.go
// UrzaLands, count_zone_fold.go controlsAllUrzaLands).

var urzaLandNames = map[string]bool{"Urza's Mine": true, "Urza's Power Plant": true, "Urza's Tower": true}

var urzaTron = []cardset.Entry{
	{Name: "Urza's Mine", Count: 5}, {Name: "Urza's Power Plant", Count: 5}, {Name: "Urza's Tower", Count: 5},
	{Name: "Wastes", Count: 3}, {Name: "Ornithopter", Count: 4}, {Name: "Bronze Sable", Count: 6},
	{Name: "Juggernaut", Count: 6}, {Name: "Wurmcoil Engine", Count: 4}, {Name: "Steel Wall", Count: 2},
}

var urzaWastes = []cardset.Entry{
	{Name: "Wastes", Count: 18}, {Name: "Ornithopter", Count: 4}, {Name: "Bronze Sable", Count: 6},
	{Name: "Juggernaut", Count: 6}, {Name: "Wurmcoil Engine", Count: 4}, {Name: "Steel Wall", Count: 2},
}

var urzaLandsDecks = []fixtureDecks{{urzaTron, urzaTron}, {urzaTron, urzaWastes}, {urzaWastes, urzaTron}}

// urzaLandsPolicy plays randomly but takes an Urza land play whenever one is
// offered and a cast three times in four, so Tron assembles and its mana is
// spent; it counts the Urza land plays under "land" and the casts of Wurmcoil
// Engine.
func urzaLandsPolicy(seed uint64, casts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x75727a61)) // "urza"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActPlayLand && urzaLandNames[refCardName(c.G, a.Src())] {
					casts["land"]++
					return a
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && prng.IntN(4) != 0 {
					casts[refCardName(c.G, a.Src())]++
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// flattenUrzaMana makes every Urza land's dynamic mana amount the constant 1
// (the lowering reverted to a plain {C}), on copies.
func flattenUrzaMana(t *testing.T) func(*kop.Table) {
	return func(tbl *kop.Table) {
		n := 0
		for i := range tbl.Cards {
			cd := &tbl.Cards[i]
			if !urzaLandNames[cd.Name] {
				continue
			}
			cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
			for j := range cd.Abilities {
				if a := &cd.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfManaDynamic != 0 {
					a.ManaV = kop.Const(1)
					n++
				}
			}
		}
		if n == 0 {
			t.Fatal("precondition: no Urza land has a dynamic mana ability")
		}
	}
}

func TestUrzaLandsLower(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstepPolicy(t, "urzalands", urzaLandsDecks, 160, true, nil, urzaLandsPolicy)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["land"] < 200 {
		t.Errorf("Urza lands played %d times", run.casts["land"])
	}
	if run.casts["Wurmcoil Engine"] < 20 {
		t.Errorf("Wurmcoil Engine cast %d times", run.casts["Wurmcoil Engine"])
	}
	mut := runFixtureLockstepPolicy(t, "urzalands", urzaLandsDecks, 160, false, flattenUrzaMana(t), urzaLandsPolicy)
	mut.requireDivergence(t, "the Urza land amount reverted")
}
