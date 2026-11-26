package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The Leyline Tyrant lockstep: "when CARDNAME dies, you may pay any amount of
// {R}. When you do, it deals that much damage to any target." The dies
// trigger's own cost is {X} with XColor$ R, so the announced X must be funded
// by red pips only (kop.PayXColorB(kop.R), trig_maypayx.go). Mixed Mountains
// and Forests make a generic X (the untinted bound) offer values red cannot
// pay, which the announcement's option list exposes.
var kleylineTyrantMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Leyline Tyrant", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Leyline Tyrant", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}}

// kleylineTyrantTally counts, on gorge's side, the Tyrant X announcements and
// those whose top offered X is 2 or more (a real coloured payment).
type kleylineTyrantTally struct{ asks, top2, games int }

func kleylineTyrantCheck(tal *kleylineTyrantTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	var last *decision.Decision
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, last = ref, nil
			tal.games++
		}
		if d := ref.E.Pending(); d != nil && d != last {
			last = d
			if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_x" &&
				chName(ref.E.G, d.Source) == "Leyline Tyrant" {
				tal.asks++
				if d.Options[len(d.Options)-1].Amount >= 2 {
					tal.top2++
				}
			}
		}
		return nil
	}
}

func TestLeylineTyrantLockstep(t *testing.T) {
	t.Parallel()
	var tal kleylineTyrantTally
	sonRun(t, "leyline_tyrant", kleylineTyrantMatchups, 200, true, nil, kleylineTyrantCheck(&tal))
	t.Logf("%d games: %d Tyrant X announcements, %d with a top X >= 2", tal.games, tal.asks, tal.top2)
	if tal.asks < 20 || tal.top2 < 5 {
		t.Errorf("announcements %d (want >= 20), top X >= 2 %d (want >= 5)", tal.asks, tal.top2)
	}
}

// TestLeylineTyrantMutation: reading the coloured X as a generic one (the
// kernel's old shape) offers X values the red sources cannot pay, so some
// seed diverges. This is the proof the lockstep above reaches the colour.
func TestLeylineTyrantMutation(t *testing.T) {
	t.Parallel()
	n := sonRun(t, "leyline_tyrant", kleylineTyrantMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			if cards[i].Name != "Leyline Tyrant" {
				continue
			}
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpIfPaid && p[x].C&0xF == kop.PayManaX {
						p[x].B = kop.Const(0) // generic X: no XColor$ colour
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with Leyline Tyrant's coloured X read as generic")
	}
}
