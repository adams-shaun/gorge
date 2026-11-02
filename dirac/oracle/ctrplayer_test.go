package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Great Goblin lockstep: "Whenever you put one or
// more counters on a Goblin, Orc, or Army you control, The Great Goblin
// deals 2 damage to target opponent" (Mode$ CounterPlayerAddedAll without
// CounterType$) is the kernel's per-recipient CounterAddedOnce event over
// every counter kind. Luminarch Aspirant's +1/+1 counter fires it on a
// Goblin and not on Grizzly Bears or on Luminarch itself; the expected answer
// at every decision is gorge's.
var kctrplayerDeck = []cardset.Entry{
	{Name: "The Great Goblin", Count: 4}, {Name: "Luminarch Aspirant", Count: 4}, {Name: "Raging Goblin", Count: 6},
	{Name: "Goblin Piker", Count: 6}, {Name: "Grizzly Bears", Count: 3}, {Name: "Shock", Count: 3},
	{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 4},
}

// kctrplayerMatchups: the deck against itself; the seed picks who goes first,
// so both seat orders are played.
var kctrplayerMatchups = [][2][]cardset.Entry{{kctrplayerDeck, kctrplayerDeck}}

type kctrplayerTally struct{ trig, nonGoblin int }

// kctrplayerCheck counts, on gorge's side, The Great Goblin's 2 damage to a
// player (its counter trigger resolving) and counters placed on a recipient
// that is no Goblin.
func kctrplayerCheck(tal *kctrplayerTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			o := ref.E.G.Obj(ev.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			switch ev.Kind {
			case events.DamageProvenance:
				if _, pl := ev.IDs[0].PlayerRef(); pl && ev.Amount == 2 && o.Face().Name == "The Great Goblin" {
					tal.trig++
				}
			case events.CounterChange:
				if ev.Text != events.EntryCounterNotice && ev.Amount > 0 {
					switch o.Face().Name {
					case "Grizzly Bears", "Luminarch Aspirant":
						tal.nonGoblin++
					}
				}
			}
		}
		return nil
	}
}

func TestCtrplayerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var tal kctrplayerTally
	sonRun(t, "ctrplayer", kctrplayerMatchups, 300, true, nil, kctrplayerCheck(&tal))
	t.Logf("The Great Goblin counter-trigger damage events %d, counters placed on a non-Goblin %d", tal.trig, tal.nonGoblin)
	if tal.trig < 20 || tal.nonGoblin < 5 {
		t.Errorf("trigger resolutions %d (want >= 20), counters on a non-Goblin %d (want >= 5)", tal.trig, tal.nonGoblin)
	}
}

// TestCtrplayerMutation: The Great Goblin's counter trigger turned to an
// opponent's placements (Trig.Whose = WhoseOpp) must diverge.
func TestCtrplayerMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	mut := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbTriggered && abs[j].Trig.Event == kop.TrigCounterAddedOnce && abs[j].Trig.N == kop.TrigNAnyCounter {
					abs[j].Trig.Whose = kop.WhoseOpp
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	if n := sonRun(t, "ctrplayer", kctrplayerMatchups, 128, false, mut, nil); n == 0 {
		t.Errorf("mutation: no seed in 128 diverged")
	}
}
