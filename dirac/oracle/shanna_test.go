package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorgeShannaXMaxFixTime is gorge cr-trigger-x-xmax (166c27426, 2026-10-26
// 16:31:02Z): before it triggeredCostXAsk offered X beyond the XMax$ cap
// (Shanna: the life gained this turn).
const gorgeShannaXMaxFixTime = "20261026163102"

// The Shanna, Purifying Blade lockstep: "at the beginning of your
// end step, you may pay {X}. If you do, draw X cards. X can't be greater than
// the amount of life you gained this turn." A trigger-own announced X
// (ChTrigX, kop.PayManaX), then the ordinary pay election and mana payment,
// then Count$xPaid as the draw. Life comes from Shanna's lifelink and Lone
// Missionary / Staunch Defenders; the opponent burns, pings and blocks.
var kshannaMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Shanna, Purifying Blade", Count: 8}, {Name: "Lone Missionary", Count: 6}, {Name: "Staunch Defenders", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Shock", Count: 6}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Suntail Hawk", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Island", Count: 9},
	},
}, {
	{
		{Name: "Shanna, Purifying Blade", Count: 8}, {Name: "Lone Missionary", Count: 6}, {Name: "Staunch Defenders", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Shanna, Purifying Blade", Count: 8}, {Name: "Lone Missionary", Count: 6}, {Name: "Staunch Defenders", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 6},
	},
}}

// kshannaTally counts, on gorge's side, the cards drawn in an end step (only
// a paid Shanna trigger draws then), the end steps with a draw and the games.
type kshannaTally struct{ draws, paid, games int }

func kshannaCheck(tal *kshannaTally) func(*GorgeSide, Side) *Divergence {
	seen := 0
	var cur *GorgeSide
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			if ev := evs[seen]; ev.Kind == events.Draw && ref.E.G.Step == state.StepEnd {
				tal.draws++
				tal.paid++
			}
		}
		return nil
	}
}

func TestShannaLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeShannaXMaxFixTime) {
		t.Skip("gorge before cr-trigger-x-xmax (166c27426)")
	}
	var tal kshannaTally
	sonRun(t, "shanna", kshannaMatchups, 300, true, nil, kshannaCheck(&tal))
	t.Logf("%d games: %d cards drawn by a paid Shanna trigger", tal.games, tal.draws)
	if tal.draws < 100 {
		t.Errorf("%d end-step draws; want at least 100", tal.draws)
	}
}

// TestShannaLockstepMutation: with the paid X unbound (the draw reads 0),
// and with the announcement dropped, some seed diverges.
func TestShannaLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeShannaXMaxFixTime) {
		t.Skip("gorge before cr-trigger-x-xmax (166c27426)")
	}
	n := sonRun(t, "shanna", kshannaMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpIfPaid && p[x].C&0xF == kop.PayManaX {
						p[x].A = kop.Const(0) // XMax$ read as 0: no X to announce
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with the X cap dropped")
	}
}
