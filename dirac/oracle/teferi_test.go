package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Teferi, Temporal Pilgrim lockstep: [-12] "Target opponent
// chooses a permanent they control and returns it to its owner's hand. Then
// they shuffle each nonland permanent they control into its owner's library."
// Drawing decks load it up (every draw adds loyalty), Island/Mountain boards
// give the target lands, creatures, Spirit tokens (Teferi's [-2]) and, with
// Act of Treason, a stolen creature owned by the other player. The expected
// answer at every decision is gorge's.
var kteferiMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Teferi, Temporal Pilgrim", Count: 6}, {Name: "Divination", Count: 10}, {Name: "Concentrate", Count: 4},
		{Name: "Island", Count: 20},
	},
	{
		{Name: "Teferi, Temporal Pilgrim", Count: 6}, {Name: "Divination", Count: 6}, {Name: "Hill Giant", Count: 6},
		{Name: "Act of Treason", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 8},
	},
}}

// kteferiTally counts, on gorge's side, the ultimate's hidden pick, the
// nonland permanents shuffled away and games.
type kteferiTally struct {
	picks, swept, shuffles, games int
}

func kteferiCheck(tal *kteferiTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	var lastAsk *decision.Decision
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, lastAsk, seen = ref, nil, 0
			tal.games++
		}
		for ; seen < len(ref.E.L.Events); seen++ {
			switch ev := ref.E.L.Events[seen]; {
			case ev.Kind == events.MoveZone && ev.From == state.ZBattlefield && ev.To == state.ZLibrary:
				tal.swept++
			case ev.Kind == events.Shuffle:
				tal.shuffles++
			}
		}
		d := ref.E.Pending()
		if d == nil || d == lastAsk {
			return nil
		}
		lastAsk = d
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "hidden_pick" && d.Options[0].Obj != 0 {
			if o := ref.E.G.Obj(d.Options[0].Obj); o != nil && o.Zone == state.ZBattlefield {
				tal.picks++
			}
		}
		return nil
	}
}

func TestTeferiLockstep(t *testing.T) {
	t.Parallel()
	var tal kteferiTally
	sonRun(t, "teferi", kteferiMatchups, 600, true, nil, kteferiCheck(&tal))
	t.Logf("%d games: %d ultimate picks, %d permanents swept into libraries, %d shuffles", tal.games, tal.picks, tal.swept, tal.shuffles)
	if tal.picks < 20 || tal.swept < 40 {
		t.Errorf("ultimate picks %d (want >= 20), permanents swept %d (want >= 40)", tal.picks, tal.swept)
	}
}

// TestTeferiLockstepMutation: with the library sweep dropped, or the hidden
// pick dropped, some seed diverges.
func TestTeferiLockstepMutation(t *testing.T) {
	t.Parallel()
	for name, drop := range map[string]func(kop.Instr) bool{
		"dropSweep": func(in kop.Instr) bool { return in.Ext() == kop.ExtLibSweep },
		"dropPick":  func(in kop.Instr) bool { return in.Op == kop.OpChooseMove },
	} {
		n := sonRun(t, "teferi", kteferiMatchups, 600, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				abs := append([]kop.Ability(nil), cards[i].Abilities...)
				for j := range abs {
					p := append([]kop.Instr(nil), abs[j].Prog...)
					for x := range p {
						if drop(p[x]) {
							p[x] = kop.Instr{Op: kop.OpNop}
						}
					}
					abs[j].Prog = p
				}
				cards[i].Abilities = abs
			}
			tbl.Cards = cards
		}, nil)
		if n == 0 {
			t.Errorf("%s: no seed diverged", name)
		}
	}
}
