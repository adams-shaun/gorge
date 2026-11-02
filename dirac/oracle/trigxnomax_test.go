package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// The no-XMax$ trigger-own {X} lockstep: "you may pay {X}.
// When you do, ..." with no XMax$ (Tester of the Tangential: move X +1/+1
// counters onto another target creature; Wildborn Preserver: put X +1/+1
// counters on it). gorge's triggeredCostXAsk bounds X by the payer's mana
// alone (capped at 64); the kernel's PayManaX carries A = 64.
var ktrigxnomaxMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Wildborn Preserver", Count: 4}, {Name: "Tester of the Tangential", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Suntail Hawk", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
	},
	{
		{Name: "Tester of the Tangential", Count: 4}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Island", Count: 10},
	},
}}

// ktrigxnomaxTally counts, on gorge's side, the carriers' X announcements,
// those whose top offered X is 3 or more, and the paid X >= 1 (a Tester's
// +1/+1 counters leaving it, which only its own move does; a Preserver's
// +1/+1 counters arriving other than as the other half of such a move).
type ktrigxnomaxTally struct{ asks, top3, paid, games int }

func ktrigxnomaxCheck(tal *ktrigxnomaxTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	var last *decision.Decision
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, last, seen = ref, nil, 0
			tal.games++
		}
		g := ref.E.G
		evs := ref.E.L.Events
		moved := false
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.Kind != events.CounterChange || ev.Counter != "P1P1" {
				moved = false
				continue
			}
			switch n := chName(g, ev.Obj); {
			case n == "Tester of the Tangential" && ev.Amount < 0:
				tal.paid++
				moved = true
			case n == "Wildborn Preserver" && ev.Amount > 0 && !moved:
				tal.paid++
			default:
				moved = false
			}
		}
		d := ref.E.Pending()
		if d == nil || d == last {
			return nil
		}
		last = d
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_x" {
			if n := chName(g, d.Source); n == "Tester of the Tangential" || n == "Wildborn Preserver" {
				tal.asks++
				if d.Options[len(d.Options)-1].Amount >= 3 {
					tal.top3++
				}
			}
		}
		return nil
	}
}

func TestTrigxnomaxLockstep(t *testing.T) {
	t.Parallel()
	var tal ktrigxnomaxTally
	sonRun(t, "shanna", ktrigxnomaxMatchups, 128, true, nil, ktrigxnomaxCheck(&tal))
	t.Logf("%d games: %d carrier X announcements, %d with a top X >= 3, %d paid X >= 1", tal.games, tal.asks, tal.top3, tal.paid)
	if tal.asks < 20 || tal.top3 < 5 || tal.paid < 5 {
		t.Errorf("announcements %d (want >= 20), top X >= 3 %d (want >= 5), paid X >= 1 %d (want >= 5)", tal.asks, tal.top3, tal.paid)
	}
}

// TestTrigxnomaxMutation: with Wildborn Preserver's PayManaX cap read as 1
// (an XMax$ of 1), some seed diverges: gorge offers X above 1.
func TestTrigxnomaxMutation(t *testing.T) {
	t.Parallel()
	n := sonRun(t, "shanna", ktrigxnomaxMatchups, 64, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			if cards[i].Name != "Wildborn Preserver" {
				continue
			}
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpIfPaid && p[x].C&0xF == kop.PayManaX {
						p[x].A = kop.Const(1)
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with Wildborn Preserver's X cap at 1")
	}
}

// The announcement's bound counts each colour of a dual land.
func TestTrigXBoundCountsDualColours(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Tester of the Tangential", Count: 12}, {Name: "Hill Giant", Count: 8},
		{Name: "Stormcarved Coast", Count: 12}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4},
	}
	lockstepRun(t, "shanna", [2][]cardset.Entry{deck, deck}, 60, true, nil)
}
