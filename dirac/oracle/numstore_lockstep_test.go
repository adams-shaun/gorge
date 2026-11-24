package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The StoreSVar number store in lockstep (dirac/act_storenum.go), over a
// synthetic probe built in process through kgen.TableFromCards: no pooled card
// is lowered through it yet (Phyrexian Processor and the Tempts need the
// token and CDA readers). The probe stores 3 as it enters, may add 1 or reset
// to its printed 0 (the delete-on-default write), and taps to gain its stored
// number in life, so every seat's life total shows the number gorge holds.
// The opponent's Naturalizes destroy it, and a destroyed probe reads its
// printed 0 again (CR 400.7).
//
// Two seats only: dirac is a two-seat kernel and kgen refuses a pod card.

const numStoreProbe = `Name:Num Store Probe
ManaCost:2
Types:Artifact
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigStore | TriggerDescription$ probe
SVar:TrigStore:DB$ StoreSVar | SVar$ N | Type$ Number | Expression$ 3
SVar:N:Number$0
A:AB$ GainLife | Cost$ T | LifeAmount$ N | SpellDescription$ probe
A:AB$ StoreSVar | Cost$ 1 | SVar$ N | Type$ CountSVar | Expression$ N/Plus.1 | SpellDescription$ probe
A:AB$ StoreSVar | Cost$ 2 | SVar$ N | Type$ Number | Expression$ 0 | SpellDescription$ probe
`

var numStoreDecks = [2][]cardset.Entry{
	{{Name: "Num Store Probe", Count: 12}, {Name: "Island", Count: 14}, {Name: "Plains", Count: 14}},
	{{Name: "Naturalize", Count: 6}, {Name: "Num Store Probe", Count: 4}, {Name: "Forest", Count: 15}, {Name: "Island", Count: 15}},
}

// numStoreRun plays seeds 1..seeds; mutate runs before the games. It returns
// how many seeds diverged (report: each is an error, else the first stops).
func numStoreRun(t *testing.T, reg *cards.Registry, seeds uint64, report bool, mutate func()) int {
	t.Helper()
	set, gd := sweepcapBuild(t, reg, numStoreProbe, numStoreDecks)
	tbl := sweepcapTable(t, set, numStoreDecks)
	if len(tbl.SVarNames) != 1 || tbl.SVarNames[0] != "N" {
		t.Fatalf("table SVarNames = %v, want [N]", tbl.SVarNames)
	}
	if mutate != nil {
		mutate()
	}
	div := 0
	played := map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"numstore-a", "numstore-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, played), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	if played["Num Store Probe"] == 0 {
		t.Fatal("the probe was never played")
	}
	return div
}

// TestNumStoreLockstep: the probe plays identically in gorge and dirac, and the
// pool reached each arm of the store: writes, a read that found its record, a
// write of the printed default that deleted one, a departure that cleared a
// number, and a write by an ability that outlived its destroyed source (gorge
// keeps that number on the departed object: the first divergence this test found).
func TestNumStoreLockstep(t *testing.T) {
	reg := synthReg(t)
	var st dirac.NumStoreStats
	dirac.DebugNumStoreStats = &st
	defer func() { dirac.DebugNumStoreStats = nil }()
	if div := numStoreRun(t, reg, 40, true, nil); div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	if st.Writes == 0 || st.Hits == 0 || st.Deletes == 0 || st.Reads <= st.Hits || st.Cleared == 0 || st.Late == 0 {
		t.Fatalf("the pool never reached the store: %+v", st)
	}
	t.Logf("store stats over 40 seeds: %+v", st)
}

// TestNumStoreMutation: with ExtStoreNum writing nothing some seed diverges, so
// the lockstep does observe the stored number. Test-only globals: never
// parallel.
func TestNumStoreMutation(t *testing.T) {
	reg := synthReg(t)
	defer func() { dirac.DebugNumStoreNoWrite = false }()
	if div := numStoreRun(t, reg, 40, false, func() { dirac.DebugNumStoreNoWrite = true }); div == 0 {
		t.Fatal("no seed diverged with the write dropped: the pool does not read the stored number")
	}
}
