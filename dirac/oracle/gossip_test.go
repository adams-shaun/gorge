package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
)

// Gossip's Talent lockstep: its level-2
// AttackersDeclared trigger targets an attacking creature with power 3 or
// less and grants "can't be blocked this turn" through an Effect CantBlockBy
// with no ExileOnMoved$. The kernel lowers that Effect to GrantKeyword
// Unblockable on the remembered target, so the only way the two engines
// agree is if gorge's grant is also exactly that target for exactly this
// combat.
var (
	kgossipUG = []cardset.Entry{
		{Name: "Gossip's Talent", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Elvish Mystic", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Forest", Count: 8},
	}
	kgossipG = []cardset.Entry{
		{Name: "Giant Spider", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Elvish Mystic", Count: 4}, {Name: "Forest", Count: 20},
	}
	kgossipMatchups = [][2][]cardset.Entry{{kgossipUG, kgossipG}, {kgossipG, kgossipUG}}
)

// kgossipTally counts, on gorge's side, the level-2 triggers that resolved:
// a triggered stack object whose source is a Gossip's Talent and whose
// effect carries the Unblockable static. A target-bearing trigger only goes
// on the stack with a legal target (CR 603.3d), so every Resolve here had
// one.
type kgossipTally struct {
	games, resolved int
}

func kgossipCheck(tal *kgossipTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.Kind != events.Resolve {
				continue
			}
			o := ref.E.G.Obj(ev.Obj)
			if o == nil || o.Source == 0 || o.Ability == nil {
				continue
			}
			src := ref.E.G.Obj(o.Source)
			if src == nil || src.Face() == nil || src.Face().Name != "Gossip's Talent" {
				continue
			}
			if strings.Contains(o.Ability.Params["StaticAbilities"], "Unblockable") {
				tal.resolved++
			}
		}
		return nil
	}
}

func TestGossipLockstep(t *testing.T) {
	var tal kgossipTally
	_, stalled := sonRunStalls(t, "gossip", kgossipMatchups, 300, true, nil, kgossipCheck(&tal))
	t.Logf("%d games: %d Gossip's Talent level-2 triggers resolved with a legal target; %d stalled loud", tal.games, tal.resolved, stalled)
	if tal.resolved < 20 {
		t.Errorf("Gossip's Talent level-2 triggers resolved %d; want >= 20", tal.resolved)
	}
}

// TestGossipMutation: deleting the GrantKeyword Unblockable instruction
// from the generated table makes some seed diverge: the lockstep is actually
// exercising the grant, not just the surrounding decks.
func TestGossipMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	mut := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := abs[j].Prog
				kept := make([]kop.Instr, 0, len(prog))
				for _, in := range prog {
					m := kop.KwMask(uint16(in.C)) | kop.KwMask(uint16(in.B.N))<<16
					if in.Op == kop.OpGrantKeyword && m&kop.KwUnblockable != 0 {
						continue
					}
					kept = append(kept, in)
				}
				abs[j].Prog = kept
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	if n := sonRun(t, "gossip", kgossipMatchups, 64, false, mut, nil); n == 0 {
		t.Error("mutation (GrantKeyword Unblockable deleted): no seed diverged")
	}
}
