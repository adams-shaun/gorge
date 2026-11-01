package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorgeKatildaFixTime is gorge son-g-script-own-tap (04c2c4c46, 2026-10-26
// 18:00:44Z, on top of 10772317e's Defined.Self reflected-mana planning): a
// prefix script never taps the searched ability's own {T} source, which
// Katilda's own "{4}{G}{W}, {T}" counters ability needs (she is a Human and
// first in battlefield order). Before it gorge's search ends in a dead-end
// script and drops the play.
const gorgeKatildaFixTime = "20261026180044"

// The Katilda, Dawnhart Prime lockstep: "Human creatures you
// control have '{T}: Add one mana of any of this creature's colors'" (a
// granted AB$ ManaReflected, Valid$ Defined.Self, ReflectProperty$ Is) and
// "{4}{G}{W}, {T}: put a +1/+1 counter on each creature you control"
// (PutCounterAll, ValidCardsDesc$ description text). The deck is mostly
// Humans of one colour (Elite Vanguard, Thraben Inspector), black
// (Phyrexian Rager), a colourless one (Cybernetic Specialist reflects
// nothing) and Katilda herself (green and white: a colour ask), beside
// non-Human spells to pay with them; few lands, so the Humans' mana is
// needed. The opponent burns and pings the Humans.
var kkatildaMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Katilda, Dawnhart Prime", Count: 6}, {Name: "Elite Vanguard", Count: 6}, {Name: "Thraben Inspector", Count: 3},
		{Name: "Phyrexian Rager", Count: 2}, {Name: "Cybernetic Specialist", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 5},
		{Name: "Swamp", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Island", Count: 1},
	},
	{
		{Name: "Shock", Count: 6}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Suntail Hawk", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Island", Count: 9},
	},
}, {
	{
		{Name: "Katilda, Dawnhart Prime", Count: 6}, {Name: "Elite Vanguard", Count: 6}, {Name: "Thraben Inspector", Count: 3},
		{Name: "Phyrexian Rager", Count: 2}, {Name: "Cybernetic Specialist", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 5},
		{Name: "Swamp", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Island", Count: 1},
	},
	{
		{Name: "Katilda, Dawnhart Prime", Count: 6}, {Name: "Elite Vanguard", Count: 6}, {Name: "Thraben Inspector", Count: 3},
		{Name: "Phyrexian Rager", Count: 2}, {Name: "Cybernetic Specialist", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 5},
		{Name: "Swamp", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Island", Count: 1},
	},
}}

// kkatildaTally counts, on gorge's side, the granted reflected mana
// activations (by the source: Katilda's two colours, a one-colour Human) and
// the Katilda counter activations.
type kkatildaTally struct{ katilda, human, games int }

func kkatildaCheck(tal *kkatildaTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur = ref
			tal.games++
			e := ref.E
			e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
				if sa.API != "ManaReflected" {
					return
				}
				if o := e.G.Obj(src); o != nil && o.Face() != nil && o.Face().Name == "Katilda, Dawnhart Prime" {
					tal.katilda++
				} else {
					tal.human++
				}
			}
		}
		return nil
	}
}

func TestCardKatildaLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeKatildaFixTime) {
		t.Skip("gorge before the prefix-script own-tap fix (04c2c4c46)")
	}
	var tal kkatildaTally
	sonRun(t, "katilda", kkatildaMatchups, 300, true, nil, kkatildaCheck(&tal))
	t.Logf("%d games: %d granted reflected activations by Katilda, %d by other Humans", tal.games, tal.katilda, tal.human)
	if tal.katilda < 100 || tal.human < 300 {
		t.Errorf("katilda %d, human %d activations; want at least 100 and 300", tal.katilda, tal.human)
	}
}

// TestCardKatildaLockstepMutation: with the grantee's colours read as W alone
// the granted mana pays differently, and some seed diverges.
func TestCardKatildaLockstepMutation(t *testing.T) {
	// (serial: the mutation switch is a package global, so the parallel lockstep waits for it)
	if !gorgePinAtLeast(t, gorgeKatildaFixTime) {
		t.Skip("gorge before the prefix-script own-tap fix (04c2c4c46)")
	}
	dirac.SetDebugOwnColoursMutate(1)
	defer dirac.SetDebugOwnColoursMutate(0)
	n := sonRun(t, "katilda", kkatildaMatchups, 100, false, func(tbl *kop.Table) {}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with the grantee's colours mutated")
	}
}
