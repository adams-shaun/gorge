package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated
// ctrnamed pool, whose carriers put named counters with no inherent rules
// meaning (CR 122.1): INVASION (Alien Invasion), LOOT (Bandit's Haul), REV
// (Chainsaw), PAGE (Diary of Dreams), INGENUITY (Lady Octopus, Inspired
// Inventor), FILM (Peter Parker's Camera) and FIRE (War Balloon). The kernel
// keeps each kind in a permanent's sparse counter slots and projects it as
// ProjObj.CountersX[k - CtrCharge]; gorge keeps the bare name string. Each
// carrier's own remove/read cost (the Haul's and the Camera's SubCounter
// costs, Diary's cost reduction, Lady Octopus's CastFree) exercises the
// reading side, and cheap burn and creatures move LOOT (a crime trigger)
// and REV (creatures dying). HONE (Dwalin, Weaponmaster) is not plain: CR
// 122.1j gives a hone counter on an Equipment +1/+0 on the equipped creature
// (gorge engine/rules/layers.go:144: a counter's effect), which the kernel
// reads in ptGen, so the lockstep compares the equipped
// creature's Power with Dwalin's counters on Chainsaw and Bonesplitter.

var kctrnamedDecks = [2][]cardset.Entry{
	// the R/G aggro deck: Alien Invasion, Chainsaw, War Balloon and
	// Equipment for the equip path, with burn that commits a crime for
	// Bandit's Haul
	{
		{Name: "Alien Invasion", Count: 2}, {Name: "Chainsaw", Count: 2},
		{Name: "War Balloon", Count: 2}, {Name: "Bonesplitter", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Dwalin, Weaponmaster", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 2},
	},
	// the U/R artifacts deck: Bandit's Haul, Diary of Dreams, Lady Octopus
	// and Peter Parker's Camera, with cheap artifacts for the CastFree and
	// instants/sorceries that put Diary's page counters
	{
		{Name: "Bandit's Haul", Count: 2}, {Name: "Diary of Dreams", Count: 2},
		{Name: "Lady Octopus, Inspired Inventor", Count: 2}, {Name: "Peter Parker's Camera", Count: 2},
		{Name: "Bonesplitter", Count: 2}, {Name: "Ornithopter", Count: 2},
		{Name: "Wind Drake", Count: 2}, {Name: "Goblin Piker", Count: 2},
		{Name: "Coral Merfolk", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Opt", Count: 4}, {Name: "Divination", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 6},
	},
}

// kctrnamedKinds is the seven plain named kinds, and HONE (CR 122.1j), with
// the gorge (CounterType) name each must count in the kernel's projection.
var kctrnamedKinds = []struct {
	kind kop.CounterKind
	name string
}{
	{kop.CtrInvasion, "INVASION"},
	{kop.CtrLoot, "LOOT"},
	{kop.CtrRev, "REV"},
	{kop.CtrPage, "PAGE"},
	{kop.CtrIngenuity, "INGENUITY"},
	{kop.CtrFilm, "FILM"},
	{kop.CtrFire, "FIRE"},
	{kop.CtrHone, "HONE"},
}

func kctrnamedSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ctrnamed")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kctrnamedRun plays seeds 1..seeds of the matchup through tbl; it returns
// the games played, whether any diverged (the first is logged) and the
// stalls, and adds to held one game per seed in which a battlefield
// permanent held at least one counter of a kind (the kernel's projection).
func kctrnamedRun(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, seeds uint64, held *[kop.NumCounterKinds3]int, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	decks := kcastDeck(t, reg, kctrnamedDecks)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kctrnamed-a", "kctrnamed-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		// seen is this game's kinds: a permanent held one at some decision.
		// The hook projects the kernel state every decision the lockstep
		// poses, so a counter that is later removed still counts.
		var seen [kop.NumCounterKinds3]bool
		prev := OccupancyHook
		OccupancyHook = func(s *dirac.State) {
			var p dirac.Projection
			s.Project(&p)
			for i := 0; i < int(p.NObj); i++ {
				o := &p.Objs[i]
				if o.Zone != kop.ZoneBattlefield {
					continue
				}
				for x, n := range o.CountersX {
					if n > 0 {
						seen[int(kop.CtrCharge)+x] = true
					}
				}
			}
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		OccupancyHook = prev
		CloseSide(k)
		games++
		for kind, ok := range seen {
			if ok {
				held[kind]++
			}
		}
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div = true
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			if !stopOnDiv {
				return
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestCtrnamedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set := kctrnamedSetup(t)
	tbl, _, _, err := set.Matchup(kctrnamedDecks[0], kctrnamedDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var held [kop.NumCounterKinds3]int
	games, div, stalls := kctrnamedRun(t, reg, set, tbl, 256, &held, true)
	t.Logf("%d games, diverged %v, stalls %d, held %v", games, div, stalls, held)
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	for _, k := range kctrnamedKinds {
		if held[k.kind] < 5 {
			t.Errorf("%s: a permanent held a counter in %d games, want at least 5", k.name, held[k.kind])
		}
	}
}

// TestCtrnamedMutation: the kernel table rewritten so that every counter
// operand naming LOOT names CHARGE must make the lockstep diverge (gorge
// holds LOOT, the kernel CHARGE) within 64 seeds.
func TestCtrnamedMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set := kctrnamedSetup(t)
	tbl, _, _, err := set.Matchup(kctrnamedDecks[0], kctrnamedDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	mut := kctrnamedLootToCharge(tbl)
	var held [kop.NumCounterKinds3]int
	_, div, _ := kctrnamedRun(t, reg, set, mut, 64, &held, false)
	if !div {
		t.Fatalf("table rewritten LOOT -> CHARGE: no divergence in 64 seeds")
	}
}

// kctrnamedCopyTable deep-copies a generated table's cards, abilities,
// programs and filters so the mutation can edit its instructions without
// touching the shared pool.
func kctrnamedCopyTable(tbl *kop.Table) *kop.Table {
	cp := *tbl
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cp.Cards {
		abs := make([]kop.Ability, len(cp.Cards[i].Abilities))
		copy(abs, cp.Cards[i].Abilities)
		for j := range abs {
			if abs[j].Prog != nil {
				abs[j].Prog = append([]kop.Instr(nil), abs[j].Prog...)
			}
		}
		cp.Cards[i].Abilities = abs
	}
	cp.Filters = append([]kop.Filter(nil), tbl.Filters...)
	return &cp
}

// kctrnamedLootToCharge rewrites every counter-kind operand that names
// CtrLoot to CtrCharge: the instruction's C on the ops that take a kind
// there, B.N on the exile/return/move ops (the kind sits in B), an
// ability's Cost.Ctr, and a PredHasCounter filter's PredN. The rewrite is
// the class, not the one carrier that puts LOOT here, so a card that
// removes, moves or doubles LOOT counters would diverge too.
func kctrnamedLootToCharge(tbl *kop.Table) *kop.Table {
	cp := kctrnamedCopyTable(tbl)
	for i := range cp.Cards {
		for a := range cp.Cards[i].Abilities {
			ab := &cp.Cards[i].Abilities[a]
			if ab.Cost.Ctr == kop.CtrLoot {
				ab.Cost.Ctr = kop.CtrCharge
			}
			for j := range ab.Prog {
				kctrnamedFlipInstr(&ab.Prog[j])
			}
		}
	}
	for i := range cp.Filters {
		if cp.Filters[i].Pred == kop.PredHasCounter && kop.CounterKind(cp.Filters[i].PredN) == kop.CtrLoot {
			cp.Filters[i].PredN = int16(kop.CtrCharge)
		}
	}
	return cp
}

// kctrnamedFlipInstr rewrites one instruction's counter kind when it names
// CtrLoot. The op says which field is the kind, so a non-counter op whose C
// happens to share CtrLoot's numeric value is never touched.
func kctrnamedFlipInstr(in *kop.Instr) {
	switch in.Op {
	case kop.OpPutCounter, kop.OpRemoveCounter, kop.OpDoubleCounters, kop.OpRemoveCounterRem,
		kop.OpEnterWithCounters, kop.OpPutCounterAll, kop.OpRemoveCounterAll:
		if kop.CounterKind(in.C) == kop.CtrLoot {
			in.C = int16(kop.CtrCharge)
		}
	case kop.OpSupport:
		// C&0xFF is the kind; C&0x100 is "other than the source".
		if kop.CounterKind(in.C&0xff) == kop.CtrLoot {
			in.C = in.C&^0xff | int16(kop.CtrCharge)
		}
	case kop.OpExileEx, kop.OpReturnEx, kop.OpMoveCounter:
		if kop.CounterKind(in.B.N) == kop.CtrLoot {
			in.B.N = int16(kop.CtrCharge)
		}
	}
}
