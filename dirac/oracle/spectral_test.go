package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorgeIndirectPhasingTime is the commit time (UTC) of gorge (son-p-indirect-phasing):
// before it an Aura or Equipment attached to a permanent that phased out stayed phased in (CR 702.26g,h).
const gorgeIndirectPhasingTime = "20261026191726"

// The Spectral Adversary lockstep: "When it enters, you may pay
// {1}{U} any number of times. When you pay this cost one or more times, put
// that many +1/+1 counters on it, then up to that many other target
// artifacts, creatures, and/or enchantments phase out." A phased-out
// permanent is treated as though it does not exist (CR 702.26b), phases in at
// its controller's untap step (702.26a), and takes its Auras and Equipment
// with it (702.26g) and back (702.26h). Pacifism and Bonesplitter on the
// board give the indirect phasing something to take. The expected answer at
// every decision is gorge's.
var kspectralMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Spectral Adversary", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Pacifism", Count: 4},
		{Name: "Bonesplitter", Count: 4}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 9},
	},
	{
		{Name: "Spectral Adversary", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Bonesplitter", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	{
		{Name: "Spectral Adversary", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Pacifism", Count: 4},
		{Name: "Bonesplitter", Count: 4}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 6}, {Name: "Bonesplitter", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}}

// kspectralTally counts, on gorge's side, the phasing the games reach.
type kspectralTally struct {
	games, out, in, indirect, indirectIn int
}

func kspectralCheck(tal *kspectralTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	type st struct{ out, ind bool }
	prev := map[state.ObjID]st{}
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, prev = ref, map[state.ObjID]st{}
			tal.games++
		}
		g := ref.E.G
		for i := range g.Objs {
			o := &g.Objs[i]
			now := st{o.PhasedOut && o.Zone == state.ZBattlefield, o.PhasedOutIndirect}
			was := prev[o.ID]
			switch {
			case now.out && !was.out && now.ind:
				tal.indirect++
			case now.out && !was.out:
				tal.out++
			case !now.out && was.out && was.ind:
				tal.indirectIn++
			case !now.out && was.out:
				tal.in++
			}
			prev[o.ID] = now
		}
		return nil
	}
}

func TestSpectralLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeIndirectPhasingTime) {
		t.Skip("gorge before the indirect phasing fix (49b193548)")
	}
	var tal kspectralTally
	_, stalled := sonRunStalls(t, "spectral", kspectralMatchups, 300, true, nil, kspectralCheck(&tal))
	t.Logf("%d games: %d direct phase-outs, %d direct phase-ins, %d indirect phase-outs, %d indirect phase-ins; %d stalled loud on an eighth payable repetition",
		tal.games, tal.out, tal.in, tal.indirect, tal.indirectIn, stalled)
	if stalled > 12 {
		t.Errorf("%d games stalled on StallNumTimes; want at most 12", stalled)
	}
	if tal.out < 100 || tal.in < 50 || tal.indirect < 20 || tal.indirectIn < 10 {
		t.Errorf("phase-outs %d (want >= 100), phase-ins %d (>= 50), indirect phase-outs %d (>= 20), indirect phase-ins %d (>= 10)",
			tal.out, tal.in, tal.indirect, tal.indirectIn)
	}
}

// TestSpectralLockstepMutation: with the phase-out dropped from the
// program, some seed diverges.
func TestSpectralLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeIndirectPhasingTime) {
		t.Skip("gorge before the indirect phasing fix (49b193548)")
	}
	n := sonRun(t, "spectral", kspectralMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtPhases {
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
		t.Fatal("no seed diverged with the phase-out dropped")
	}
}
